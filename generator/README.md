# generator

Python (FastAPI) service that generates random purchase events and sends them to a destination (sink).
It streams events continuously and also generates more events on demand through an HTTP API.

## Running

From the repository root, using [mise](https://mise.jdx.dev) (see the [root README](../README.md)):

```bash
mise run generator:dev    # runs on localhost:8000, pushing events to the backend
```

Or with Docker:

```bash
mise run generator:image
docker run --rm -p 8000:8000 -e EVENTS_PER_SECOND=2 purchase-generator
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `SINK` | `stdout` | Event destination: `stdout`, `http` or `kafka` |
| `EVENTS_PER_SECOND` | `1` | Continuous stream rate (`0` disables it) |
| `HTTP_TARGET_URL` | `http://localhost:9000/events` | URL that receives a POST with the list of events (`SINK=http`) |
| `KAFKA_BOOTSTRAP_SERVERS` | `localhost:9092` | Kafka brokers (`SINK=kafka`) |
| `KAFKA_TOPIC` | `purchases` | Kafka topic (the message key is the `order_id`) |
| `FAKER_LOCALE` | `pt_BR` | Locale for the fake data |
| `SEED` | — | Seed for reproducible data |
| `MAX_BATCH` | `10000` | Maximum number of events per request |
| `PORT` | `8000` | API port |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP/HTTP endpoint of the OpenTelemetry Collector; traces and metrics are only exported when it is set |
| `OTEL_SERVICE_NAME` | `generator` | Service name of the traces and metrics |

Any other standard `OTEL_*` variable (e.g. `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`) is read by the
OpenTelemetry SDK. With telemetry enabled, every request (except `/health`) and every event pushed to the sink
is traced, and the counters `generator.events.generated`, `generator.events.sent` and `generator.send.errors`
are exported (attribute `origin`: `stream` or `api`).

## API

Interactive docs at `http://localhost:8000/docs`.

| Method | Route | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `GET` | `/events?count=N` | Generates N events and only returns them (does not send) |
| `POST` | `/events/generate` | Generates more events on demand: `{"count": 100, "send": true, "return_events": false}` |
| `GET` | `/stream` | Continuous stream status (running, rate, sent, errors) |
| `POST` | `/stream/start` | Starts the stream, optionally with a new rate: `{"events_per_second": 5}` |
| `POST` | `/stream/stop` | Stops the stream |

```bash
curl -X POST localhost:8000/events/generate -H 'content-type: application/json' -d '{"count": 500}'
```

## Tests

```bash
mise run generator:test
```
