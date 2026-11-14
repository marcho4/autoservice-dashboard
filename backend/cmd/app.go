package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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

func resetPassword(ctx context.Context, email string) error {
	cfg, err := config.LoadDB()
	if err != nil {
		return err
	}
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	password, err := app.ResetPassword(ctx, pool, email)
	if err != nil {
		return err
	}
	fmt.Printf("New password: %s\nAll sessions of this account are signed out.\n", password)
	return nil
}

func serve(ctx context.Context, spamGo bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.SetupLogging("autoservice-backend")
	slog.SetDefault(log)
	for name, value := range map[string]string{"JWT_SECRET": cfg.JWTSecret, "BOT_API_KEY": cfg.BotAPIKey} {
		if strings.HasPrefix(value, "change-me") {
			log.Warn("secret is still the placeholder from .env.example, replace it", "variable", name)
		}
	}

	if spamGo {
		slog.Info("Starting goroutines to test alert", "count", 150, "duration", 15*time.Minute)
		for range 150 {
			go func() {
				timer := time.NewTimer(15 * time.Minute)
				defer timer.Stop()
				select {
				case <-timer.C:
				case <-ctx.Done():
				}
			}()
		}
	}

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	router, err := app.NewRouter(cfg, pool, log)
	if err != nil {
		return err
	}

	botCtx, stopBot := context.WithCancel(ctx)
	botDone := make(chan struct{})
	if cfg.BotRunner {
		go func() {
			defer close(botDone)
			_ = app.NewBotRunner(cfg, pool, log).Run(botCtx)
		}()
	} else {
		close(botDone)
	}
	defer func() {
		stopBot()
		<-botDone
	}()

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

func runBot(ctx context.Context) error {
	cfg, err := config.LoadBot()
	if err != nil {
		return err
	}
	log := logger.SetupLogging("autoservice-bot")

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	return app.NewBotRunner(cfg, pool, log).Run(ctx)
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
