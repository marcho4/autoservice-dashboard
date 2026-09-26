package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type EmployeeRepo struct{ db *pgxpool.Pool }

func NewEmployeeRepo(db *pgxpool.Pool) *EmployeeRepo { return &EmployeeRepo{db: db} }

const employeeColumns = `id, email, full_name, password_hash, token_version, created_at, updated_at`

func scanEmployee(row pgx.Row) (domain.Employee, error) {
	var e domain.Employee
	err := row.Scan(&e.ID, &e.Email, &e.FullName, &e.PasswordHash, &e.TokenVersion, &e.CreatedAt, &e.UpdatedAt)
	return e, notFound(err)
}

func emailTaken(err error) error {
	if uniqueConstraint(err) == "employees_email_key" {
		return domain.ErrEmailTaken
	}
	return err
}

func (r *EmployeeRepo) Create(ctx context.Context, e domain.Employee) (domain.Employee, error) {
	out, err := scanEmployee(r.db.QueryRow(ctx,
		`INSERT INTO employees (email, full_name, password_hash) VALUES ($1, $2, $3) RETURNING `+employeeColumns,
		e.Email, e.FullName, e.PasswordHash))
	return out, emailTaken(err)
}

func (r *EmployeeRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Employee, error) {
	return scanEmployee(r.db.QueryRow(ctx, `SELECT `+employeeColumns+` FROM employees WHERE id = $1`, id))
}

func (r *EmployeeRepo) GetByEmail(ctx context.Context, email string) (domain.Employee, error) {
	return scanEmployee(r.db.QueryRow(ctx, `SELECT `+employeeColumns+` FROM employees WHERE email = $1`, email))
}

func (r *EmployeeRepo) UpdateProfile(ctx context.Context, id uuid.UUID, email, fullName string) (domain.Employee, error) {
	out, err := scanEmployee(r.db.QueryRow(ctx,
		`UPDATE employees SET email = $2, full_name = $3, updated_at = now() WHERE id = $1 RETURNING `+employeeColumns,
		id, email, fullName))
	return out, emailTaken(err)
}

func (r *EmployeeRepo) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) (domain.Employee, error) {
	return scanEmployee(r.db.QueryRow(ctx,
		`UPDATE employees SET password_hash = $2, token_version = token_version + 1, updated_at = now()
		 WHERE id = $1 RETURNING `+employeeColumns, id, hash))
}

func (r *EmployeeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return affectedOrNotFound(r.db.Exec(ctx, `DELETE FROM employees WHERE id = $1`, id))
}

type RevokedTokenRepo struct{ db *pgxpool.Pool }

func NewRevokedTokenRepo(db *pgxpool.Pool) *RevokedTokenRepo { return &RevokedTokenRepo{db: db} }

func (r *RevokedTokenRepo) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	if err := r.deleteExpired(ctx); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO revoked_tokens (jti, expires_at) VALUES ($1, $2) ON CONFLICT (jti) DO NOTHING`, jti, expiresAt)
	return err
}

func (r *RevokedTokenRepo) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var revoked bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM revoked_tokens WHERE jti = $1)`, jti).Scan(&revoked)
	return revoked, err
}

func (r *RevokedTokenRepo) deleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM revoked_tokens WHERE expires_at < now()`)
	return err
}
