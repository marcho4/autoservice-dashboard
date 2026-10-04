package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/marcho4/autoservice-dashboard/backend/internal/app"
	"github.com/marcho4/autoservice-dashboard/backend/internal/config"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/postgres"
	"github.com/marcho4/autoservice-dashboard/backend/pkg/logger"
)

func migrate(ctx context.Context) error {
	cfg, err := config.LoadDB()
	if err != nil {
		return err
	}
	log := logger.SetupLogging("autoservice-migrate")

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	return postgres.Migrate(ctx, pool, log)
}

func serve(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.SetupLogging("autoservice-backend")
	slog.SetDefault(log)

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	router, err := app.NewRouter(cfg, pool, log)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return runServer(ctx, srv, cfg.ShutdownTimeout, log)
}

func runServer(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration, log *slog.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutdown signal received, draining connections", "timeout", shutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("server stopped gracefully")
	return nil
}
