// Command server runs the backend API and its database migrations.
//
// Usage:
//
//	server                   start the API (applies pending migrations when RUN_MIGRATIONS=true)
//	server migrate up        apply all pending migrations
//	server migrate down [N]  revert the last N migrations (default 1)
//	server migrate version   print the current schema version
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brunohms/cat-n-rat/app/backend/internal/api"
	"github.com/brunohms/cat-n-rat/app/backend/internal/config"
	"github.com/brunohms/cat-n-rat/app/backend/internal/database"
	"github.com/brunohms/cat-n-rat/app/backend/internal/generator"
	"github.com/brunohms/cat-n-rat/app/backend/internal/purchase"
	"github.com/brunohms/cat-n-rat/app/backend/internal/telemetry"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger, os.Args[1:]); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, telemetryEnabled, err := telemetry.Setup(ctx, "backend")
	if err != nil {
		return fmt.Errorf("set up telemetry: %w", err)
	}
	defer func() {
		// Runs after the context is cancelled, so it gets its own deadline to flush the last spans and metrics
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(flushCtx); err != nil {
			logger.Warn("failed to flush telemetry", "error", err)
		}
	}()
	if telemetryEnabled {
		logger.Info("telemetry enabled", "endpoint", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if len(args) > 0 && args[0] == "migrate" {
		return runMigrate(logger, pool, args[1:])
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown command %q", args[0])
	}

	if cfg.RunMigrations {
		if err := database.MigrateUp(pool); err != nil {
			return err
		}
		logger.Info("migrations applied")
	}
	return serve(ctx, logger, cfg, pool)
}

func runMigrate(logger *slog.Logger, pool *pgxpool.Pool, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: server migrate up | down [N] | version")
	}
	switch args[0] {
	case "up":
		if err := database.MigrateUp(pool); err != nil {
			return err
		}
	case "down":
		steps := 1
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid number of steps %q", args[1])
			}
			steps = n
		}
		if err := database.MigrateDown(pool, steps); err != nil {
			return err
		}
	case "version":
	default:
		return fmt.Errorf("unknown migrate command %q", args[0])
	}

	version, dirty, err := database.MigrationVersion(pool)
	if err != nil {
		return err
	}
	logger.Info("schema version", "version", version, "dirty", dirty)
	return nil
}

func serve(ctx context.Context, logger *slog.Logger, cfg config.Config, pool *pgxpool.Pool) error {
	server := api.NewServer(
		pool,
		purchase.NewStore(pool),
		generator.NewClient(cfg.GeneratorURL, cfg.GeneratorTimeout),
		cfg.MaxBatch,
		logger,
	)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server started", "port", cfg.Port, "generator_url", cfg.GeneratorURL)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
