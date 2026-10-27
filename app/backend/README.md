# backend

Go backend that stores purchase events in PostgreSQL 14. Events arrive in two ways:

- **push**: the generator sends events to `POST /events` (generator with `SINK=http`).
- **pull**: the backend requests events from the generator through `POST /events/fetch`.

Events are stored idempotently: an `event_id` that is already in the database is ignored.

## Running

From the repository root, using [mise](https://mise.jdx.dev) (see the [root README](../../README.md)):

```bash
mise run backend:dev    # starts PostgreSQL 14 if needed and runs the backend on localhost:8001
mise run dev:local      # same, plus the generator and the frontend
```

The backend reads `DATABASE_URL` from `mise.toml`, which points to the database started by `mise run db:start`.

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://catnrat:catnrat@localhost:5432/catnrat?sslmode=disable` | Database URL |
| `GENERATOR_URL` | `http://localhost:8000` | Base URL of the generator |
| `GENERATOR_TIMEOUT` | `30` | Timeout in seconds for requests to the generator |
| `MAX_BATCH` | `10000` | Maximum number of events per request |
| `RUN_MIGRATIONS` | `true` | Applies pending migrations when the server starts |
| `PORT` | `8000` | API port (`mise run backend:dev` uses `BACKEND_PORT`, default `8001`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP/HTTP endpoint of the OpenTelemetry Collector; traces and metrics are only exported when it is set |
| `OTEL_SERVICE_NAME` | `backend` | Service name of the traces and metrics |

## Telemetry

With `OTEL_EXPORTER_OTLP_ENDPOINT` set, the backend exports with OpenTelemetry (any other standard `OTEL_*`
variable is read by the SDK):

- a span for every request (except `/health`), named after its route, e.g. `GET /purchases/{event_id}`;
- a span for every database query ([otelpgx](https://github.com/exaring/otelpgx)) and every request to the
  generator, which propagates the trace context, so the generator spans join the same trace;
- the HTTP server metrics and the counters `catnrat.events.received` and `catnrat.events.inserted`
  (attribute `source`: `push` or `pull`).

The request log lines include `trace_id` and `span_id`, which the collector uses to link each log to its trace.
In the kind cluster everything ends up in ClickStack (see [`infra/`](../../infra/README.md#monitoring)).

## API

Errors are returned as `{"detail": "..."}`. Invalid query parameters return `422`.

| Method | Route | Description |
|---|---|---|
| `GET` | `/health` | Health check (also checks the database connection) |
| `POST` | `/events` | Receives a list of purchase events pushed by the generator |
| `POST` | `/events/fetch` | Requests events from the generator and stores them: `{"count": 100}` (default 10) |
| `GET` | `/purchases` | Lists purchases (filters below) |
| `GET` | `/purchases/{event_id}` | Returns a single purchase with its customer and items |
| `GET` | `/customers` | Lists customers with purchase aggregates (filters below) |
| `GET` | `/filters` | Distinct values for the select filters: statuses, channels, payment methods, sources and states |

The list routes return one page plus the number of matching rows:

```json
{"data": [...], "total": 132, "limit": 50, "offset": 0}
```

### `GET /purchases`

| Parameter | Description |
|---|---|
| `q` | Case-insensitive text search on order ID, customer name or customer email |
| `status`, `channel`, `payment_method`, `source`, `customer_id` | Exact match |
| `from`, `to` | Event time range; `from` is inclusive and `to` is exclusive. RFC 3339 timestamp or `YYYY-MM-DD` (midnight UTC) |
| `min_total`, `max_total` | Total range, inclusive |
| `sort` | `event_timestamp` (default), `received_at`, `total`, `order_id`, `customer`, `status` |
| `order` | `desc` (default) or `asc` |
| `limit`, `offset` | Page size (1-500, default 50) and offset |

### `GET /customers`

| Parameter | Description |
|---|---|
| `q` | Case-insensitive text search on customer ID, name or email |
| `state` | Exact match |
| `city` | Case-insensitive partial match |
| `sort` | `name` (default), `customer_id`, `state`, `purchase_count`, `total_spent`, `last_purchase_at` |
| `order` | `asc` (default) or `desc` |
| `limit`, `offset` | Page size (1-500, default 50) and offset |

Each customer includes `purchase_count`, `total_spent` (sum of the totals of the purchases with status `paid`)
and `last_purchase_at`.

```bash
curl -X POST localhost:8001/events/fetch -H 'content-type: application/json' -d '{"count": 50}'
curl "localhost:8001/purchases?status=paid&min_total=100&sort=total&order=desc&limit=5"
curl "localhost:8001/customers?state=SP&sort=total_spent&order=desc"
```

## Project layout

| Path | Description |
|---|---|
| `cmd/server` | Entry point: API server and `migrate` subcommands |
| `internal/api` | HTTP routes and handlers |
| `internal/purchase` | Event model, validation and database access |
| `internal/generator` | HTTP client for the generator |
| `internal/database` | Connection pool and migration runner |
| `internal/config` | Settings loaded from environment variables |
| `internal/telemetry` | OpenTelemetry tracer and meter providers |
| `migrations` | SQL migrations, embedded in the binary |

## Database

Tables created by the initial migration:

- `customers`: one row per customer, updated with the latest data received.
- `purchases`: one row per event (`event_id` is the primary key), with totals, payment data, `source` (`push` or `pull`) and the original event in `payload` (JSONB).
- `purchase_items`: the items of each purchase.

Money values are `NUMERIC(12,2)` and are never converted to floating point on the way in or out.

## Migrations

Migrations use [golang-migrate](https://github.com/golang-migrate/migrate). Each one is a pair of SQL files in
`migrations/` (`NNNNNN_name.up.sql` and `NNNNNN_name.down.sql`). They are embedded in the binary and run
automatically when the server starts. To run them manually:

```bash
mise run db:migrate         # apply all pending migrations
mise run db:rollback [N]    # revert the last N migrations (default 1)
mise run db:version         # print the current schema version
```

The same commands are available on the binary itself: `server migrate up | down [N] | version`.

To add a migration, create the next pair of files, e.g. `000002_add_x.up.sql` and `000002_add_x.down.sql`.
