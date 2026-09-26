package postgres

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationLockKey = 7_311_202_601

func Migrate(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLockKey)

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	files, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	applied := 0
	for _, file := range files {
		version := strings.TrimSuffix(file.Name(), ".sql")
		done, err := isMigrationApplied(ctx, conn, version)
		if err != nil {
			return err
		}
		if done {
			continue
		}
		if err := applyMigration(ctx, conn, version); err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		log.Info("migration applied", "version", version)
		applied++
	}
	log.Info("migrations up to date", "applied", applied, "total", len(files))
	return nil
}

func isMigrationApplied(ctx context.Context, conn *pgxpool.Conn, version string) (bool, error) {
	var applied bool
	err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied)
	return applied, err
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, version string) error {
	sql, err := migrationsFS.ReadFile("migrations/" + version + ".sql")
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version)
		return err
	})
}
