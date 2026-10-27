// Package outbox publishes the messages of the outbox table to Kafka.
//
// The messages are written by the purchase store in the same transaction as the purchases (transactional outbox),
// so they survive a Kafka outage or a crash: the relay keeps retrying, and a row is only deleted after Kafka has
// acknowledged it. A crash between the acknowledgement and the delete publishes the row again, so consumers
// must be idempotent (the worker skips event_ids it has already counted).
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
)

const (
	// Rows published per transaction
	batchSize = 500
	// Wait between polls when the outbox is empty
	idleInterval = 500 * time.Millisecond
	maxBackoff   = 30 * time.Second
)

// NewClient creates the Kafka producer, traced with OpenTelemetry (a no-op when telemetry is disabled).
func NewClient(brokers []string) (*kgo.Client, error) {
	tracing := kotel.NewKotel(kotel.WithTracer(kotel.NewTracer()), kotel.WithMeter(kotel.NewMeter()))
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.WithHooks(tracing.Hooks()...),
		// Waits for the in-sync replicas, so an acknowledged message is not lost
		kgo.RequiredAcks(kgo.AllISRAcks()),
		// Without it a send retries forever while Kafka is down, silently holding the outbox transaction open;
		// with it the batch fails, is logged and is retried with backoff
		kgo.RecordDeliveryTimeout(15*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}
	return client, nil
}

type Relay struct {
	pool      *pgxpool.Pool
	client    *kgo.Client
	logger    *slog.Logger
	published metric.Int64Counter
}

func NewRelay(pool *pgxpool.Pool, client *kgo.Client, logger *slog.Logger) *Relay {
	// Instruments of the global meter are no-ops when telemetry is disabled, so the error can be ignored
	published, _ := otel.Meter("github.com/brunohms/cat-n-rat/app/backend/internal/outbox").Int64Counter(
		"catnrat.outbox.published", metric.WithDescription("Outbox messages published to Kafka"), metric.WithUnit("{message}"))
	return &Relay{pool: pool, client: client, logger: logger, published: published}
}

// Run publishes the outbox until the context is cancelled.
func (r *Relay) Run(ctx context.Context) {
	backoff := time.Second
	for {
		n, err := r.publishBatch(ctx)
		if ctx.Err() != nil {
			return
		}

		wait := time.Duration(0)
		switch {
		case err != nil:
			r.logger.Error("failed to publish outbox messages, retrying", "error", err, "retry_in", backoff.String())
			wait = backoff
			backoff = min(backoff*2, maxBackoff)
		case n < batchSize:
			// Caught up; a full batch means there may be more rows waiting, so the next one starts right away
			backoff = time.Second
			wait = idleInterval
		default:
			backoff = time.Second
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}
}

// publishBatch sends the oldest outbox rows to Kafka and deletes them once they are acknowledged.
// It returns the number of rows published.
func (r *Relay) publishBatch(ctx context.Context) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// SKIP LOCKED lets several backend replicas relay at the same time without sending the same row twice
	rows, err := tx.Query(ctx, `
		SELECT id, topic, message_key, payload, headers
		FROM outbox
		ORDER BY id
		LIMIT $1
		FOR UPDATE SKIP LOCKED`, batchSize)
	if err != nil {
		return 0, fmt.Errorf("read outbox: %w", err)
	}
	var ids []int64
	records, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*kgo.Record, error) {
		var (
			id      int64
			record  kgo.Record
			key     string
			headers []byte
			carrier propagation.MapCarrier
		)
		if err := row.Scan(&id, &record.Topic, &key, &record.Value, &headers); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(headers, &carrier); err != nil {
			return nil, fmt.Errorf("decode headers of outbox row %d: %w", id, err)
		}
		ids = append(ids, id)
		record.Key = []byte(key)
		// The publish span (kotel) becomes a child of the request that stored the purchase
		record.Context = otel.GetTextMapPropagator().Extract(ctx, carrier)
		return &record, nil
	})
	if err != nil {
		return 0, fmt.Errorf("read outbox: %w", err)
	}
	if len(records) == 0 {
		return 0, nil
	}

	if err := r.client.ProduceSync(ctx, records...).FirstErr(); err != nil {
		return 0, fmt.Errorf("produce: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM outbox WHERE id = ANY($1)`, ids); err != nil {
		return 0, fmt.Errorf("delete published rows: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	r.published.Add(ctx, int64(len(records)))
	return len(records), nil
}
