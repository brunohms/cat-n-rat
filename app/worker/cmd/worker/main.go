// Command worker consumes the purchase messages that the backend publishes to Kafka and keeps the
// customer_stats table up to date.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brunohms/cat-n-rat/app/worker/internal/config"
	"github.com/brunohms/cat-n-rat/app/worker/internal/consumer"
	"github.com/brunohms/cat-n-rat/app/worker/internal/stats"
	"github.com/brunohms/cat-n-rat/app/worker/internal/telemetry"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, telemetryEnabled, err := telemetry.Setup(ctx, "worker")
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

	pool, err := connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	c, err := consumer.New(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup, stats.NewStore(pool), logger)
	if err != nil {
		return err
	}
	defer c.Close()

	healthServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           healthHandler(pool),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		if err := healthServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	logger.Info("worker started", "kafka_brokers", strings.Join(cfg.KafkaBrokers, ","),
		"kafka_topic", cfg.KafkaTopic, "kafka_group", cfg.KafkaGroup, "port", cfg.Port)
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		c.Run(ctx)
	}()

	select {
	case err := <-errCh:
		stop()
		<-consumerDone
		return fmt.Errorf("health server: %w", err)
	case <-consumerDone:
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return healthServer.Shutdown(shutdownCtx)
}

func connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	// One span per query; a no-op when telemetry is disabled
	config.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// healthHandler answers the Kubernetes probes; the worker is healthy while it can reach the database.
func healthHandler(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"detail": %q}`, "database unavailable: "+err.Error())
			return
		}
		fmt.Fprint(w, `{"status": "ok"}`)
	})
	return mux
}
