// Package consumer reads the purchase messages from Kafka and applies them to the customer stats.
package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/brunohms/cat-n-rat/app/worker/internal/stats"
)

const maxBackoff = 30 * time.Second

type Consumer struct {
	client   *kgo.Client
	tracer   *kotel.Tracer
	store    *stats.Store
	logger   *slog.Logger
	messages metric.Int64Counter
}

// New joins the consumer group. Offsets are committed only after the messages are applied, so a crash makes
// Kafka redeliver them, and the store skips the ones already applied.
func New(brokers []string, topic, group string, store *stats.Store, logger *slog.Logger) (*Consumer, error) {
	tracer := kotel.NewTracer(kotel.ConsumerGroup(group))
	tracing := kotel.NewKotel(kotel.WithTracer(tracer), kotel.WithMeter(kotel.NewMeter()))
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		// A new group starts from the oldest message, so nothing published before the first start is missed
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
		// Partitions are not reassigned while a poll is being processed, so its offsets are still ours to commit
		kgo.BlockRebalanceOnPoll(),
		kgo.WithHooks(tracing.Hooks()...),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	// Instruments of the global meter are no-ops when telemetry is disabled, so the errors can be ignored
	meter := otel.Meter("github.com/brunohms/cat-n-rat/app/worker/internal/consumer")
	messages, _ := meter.Int64Counter(
		"catnrat.stats.messages",
		metric.WithDescription("Purchase messages consumed, by result (applied, duplicate or invalid)"),
		metric.WithUnit("{message}"))

	c := &Consumer{client: client, tracer: tracer, store: store, logger: logger, messages: messages}
	c.registerLag(meter, group)
	return c, nil
}

// registerLag exports the lag of the group: the messages already in each partition that the group has not
// committed yet. Growing lag means the worker is falling behind (or stuck on a database error).
// Every replica reports the lag of the whole group, so use the maximum, not the sum, across pods.
func (c *Consumer) registerLag(meter metric.Meter, group string) {
	admin := kadm.NewClient(c.client)
	// Only called when the metrics are collected, so it costs nothing while telemetry is disabled
	_, _ = meter.Int64ObservableGauge("catnrat.stats.consumer_lag",
		metric.WithDescription("Messages not yet processed by the consumer group, by topic and partition"),
		metric.WithUnit("{message}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			lags, err := admin.Lag(ctx, group)
			if err == nil {
				err = lags.Error()
			}
			if err != nil {
				// Skips this collection instead of failing the export of the other metrics
				c.logger.Warn("failed to read consumer lag", "error", err)
				return nil
			}
			for _, lag := range lags[group].Lag.Sorted() {
				// -1 means the lag of the partition could not be computed
				if lag.Lag >= 0 {
					o.Observe(lag.Lag, metric.WithAttributes(
						attribute.String("topic", lag.Topic), attribute.Int("partition", int(lag.Partition))))
				}
			}
			return nil
		}))
}

func (c *Consumer) Close() {
	c.client.Close()
}

// Run consumes until the context is cancelled.
func (c *Consumer) Run(ctx context.Context) {
	// Close waits for blocked rebalances, so a return between a poll and its AllowRebalance would hang the shutdown
	defer c.client.AllowRebalance()
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			c.logger.Error("failed to fetch messages", "topic", topic, "partition", partition, "error", err)
		})

		var done []*kgo.Record
		for _, record := range fetches.Records() {
			if err := c.handle(ctx, record); err != nil {
				// Only happens on shutdown; the record is not committed and is redelivered on the next start
				break
			}
			done = append(done, record)
		}

		if len(done) > 0 {
			// Its own context, so the offsets of the applied records are still committed during a shutdown
			commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			if err := c.client.CommitRecords(commitCtx, done...); err != nil {
				// The records are redelivered and skipped as duplicates, so this only costs some extra work
				c.logger.Warn("failed to commit offsets", "error", err)
			}
			cancel()
		}
		c.client.AllowRebalance()
	}
}

// handle applies one record, retrying database errors until it succeeds; it only fails when ctx is cancelled.
func (c *Consumer) handle(ctx context.Context, record *kgo.Record) error {
	// The process span continues the trace of the backend request that stored the purchase
	spanCtx, span := c.tracer.WithProcessSpan(record)
	defer span.End()
	ctx = trace.ContextWithSpan(ctx, trace.SpanFromContext(spanCtx))

	logAttrs := []any{"partition", record.Partition, "offset", record.Offset}
	if sc := span.SpanContext(); sc.IsValid() {
		logAttrs = append(logAttrs, "trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
	}

	msg, err := stats.Decode(record.Value)
	if err != nil {
		// Retrying would never succeed, so the message is skipped instead of blocking the partition
		span.SetStatus(codes.Error, err.Error())
		c.count(ctx, "invalid")
		c.logger.Error("skipping invalid message", append(logAttrs, "error", err)...)
		return nil
	}
	span.SetAttributes(attribute.String("catnrat.event_id", msg.EventID.String()),
		attribute.String("catnrat.customer_id", msg.CustomerID))

	backoff := time.Second
	for {
		applied, err := c.store.Apply(ctx, msg)
		if err == nil {
			result := "applied"
			if !applied {
				result = "duplicate"
			}
			c.count(ctx, result)
			c.logger.Info("customer stats updated", append(logAttrs,
				"result", result, "event_id", msg.EventID, "customer_id", msg.CustomerID)...)
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		span.RecordError(err)
		c.logger.Error("failed to update customer stats, retrying",
			append(logAttrs, "error", err, "retry_in", backoff.String())...)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func (c *Consumer) count(ctx context.Context, result string) {
	c.messages.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}
