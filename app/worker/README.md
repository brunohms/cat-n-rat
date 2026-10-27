# worker

Go service that keeps the purchase aggregates of each customer up to date. The backend publishes a message to
Kafka for every purchase it stores for the first time; the worker consumes them and updates the `customer_stats`
table in PostgreSQL, which `GET /customers` of the backend reads.

```text
backend --(outbox table)--> Kafka topic purchases.stored --> worker --> customer_stats
```

## Running

From the repository root, using [mise](https://mise.jdx.dev) (see the [root README](../../README.md)):

```bash
mise run worker:dev    # starts PostgreSQL and Kafka in Docker if needed and runs the worker
mise run dev:local     # same, plus the backend, the generator and the frontend
```

The worker does not create tables: the schema comes from the [backend migrations](../backend/README.md#migrations),
so the backend must have started at least once. Until then the worker keeps retrying.

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://catnrat:catnrat@localhost:5432/catnrat?sslmode=disable` | Database URL |
| `KAFKA_BROKERS` | `localhost:9092` | Kafka bootstrap brokers, comma-separated |
| `KAFKA_TOPIC` | `purchases.stored` | Topic with the messages published by the backend |
| `KAFKA_GROUP` | `customer-stats` | Consumer group; Kafka keeps its committed offsets |
| `PORT` | `8000` | Port of the health check, `GET /health` (`mise run worker:dev` uses `WORKER_PORT`, default `8002`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP/HTTP endpoint of the OpenTelemetry Collector; traces and metrics are only exported when it is set |
| `OTEL_SERVICE_NAME` | `worker` | Service name of the traces and metrics |

## Message

One JSON message per purchase, with the `customer_id` as the key, so all the messages of a customer go to the
same partition:

```json
{
  "event_id": "6f1c1a6e-2d2f-4c39-9a8e-3c1d6b2a9f10",
  "order_id": "ORD-1",
  "customer_id": "CUS-000123",
  "status": "paid",
  "total": 123.45,
  "event_timestamp": "2026-10-27T12:00:00Z"
}
```

For each message the worker adds 1 to `purchase_count`, adds the `total` to `total_spent` when the status is
`paid`, and keeps the latest `event_timestamp` in `last_purchase_at`.

## Delivery guarantees

- **Nothing is lost**: the backend writes the message to its `outbox` table in the same transaction as the
  purchase, and only deletes it after Kafka acknowledges it. If Kafka is down, the messages wait there.
- **Nothing is counted twice**: Kafka delivers at least once (a message can come again after a crash or a
  rebalance). The worker records each `event_id` in `customer_stats_events` in the same transaction as the
  update, and skips the ones already there. Offsets are committed only after the update.
- **Order does not matter**: every update is a sum, a count or a maximum, so messages can be applied in any
  order, by any number of replicas (up to the number of partitions).
- An invalid message (bad JSON, missing fields) is logged and skipped, so it does not block its partition.
  Database errors are retried with backoff until they succeed.

## Monitoring

With `OTEL_EXPORTER_OTLP_ENDPOINT` set (always the case in the kind cluster, where everything goes to ClickStack):

- **Traces**: each message gets a `purchases.stored process` span in the same trace as the backend request that
  stored the purchase, with the SQL queries of the update below it.
- **Metrics**:
  - `catnrat.stats.messages`: messages consumed, by `result` (`applied`, `duplicate` or `invalid`).
  - `catnrat.stats.consumer_lag`: messages in each partition (`topic`, `partition`) that the group has not
    processed yet. A lag that keeps growing means the worker is falling behind or stuck retrying a database
    error. Every replica reports the lag of the whole group, so take the maximum across pods, not the sum.
  - `messaging.kafka.*` (Kafka client) and `db.client.operation.duration` (database queries).
- **Logs**: the pod logs are collected by the cluster agent; the `customer stats updated` lines carry
  `trace_id` and `span_id`, so in HyperDX each one opens the trace of its message.

CPU and memory of the pod come from the kubelet metrics, like every other pod.

## Project layout

| Path | Description |
|---|---|
| `cmd/worker` | Entry point: consumer, health check and graceful shutdown |
| `internal/consumer` | Kafka consumer group: polling, retries and offset commits |
| `internal/stats` | Message format and the `customer_stats` update |
| `internal/config` | Settings loaded from environment variables |
| `internal/telemetry` | OpenTelemetry tracer and meter providers |

## Tests

```bash
mise run worker:test
mise run worker:lint
```
