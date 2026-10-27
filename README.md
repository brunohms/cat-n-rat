# cat-n-rat

## About the project

### Technologies

<p align="center">
  <img src="https://img.shields.io/badge/Kubernetes-326CE5?logo=kubernetes&logoColor=white">
  <img src="https://img.shields.io/badge/Helm-0F1689?logo=helm&logoColor=white">
  <img src="https://img.shields.io/badge/Terraform-7B42BC?logo=terraform&logoColor=white">
  <img src="https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white">
  <img src="https://img.shields.io/badge/Python-3776AB?logo=python&logoColor=white">
  <img src="https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB">
  <img src="https://img.shields.io/badge/ClickHouse-FFCC01?logo=clickhouse&logoColor=black">
  <img src="https://img.shields.io/badge/OpenTelemetry-425CC7?logo=opentelemetry&logoColor=white">
</p>

---

| Folder | Description |
|---|---|
| [`generator/`](generator/README.md) | Python service that generates random purchase events |
| [`app/backend/`](app/backend/README.md) | Go backend that receives, requests and stores the events in PostgreSQL 14 |
| [`app/frontend/`](app/frontend/README.md) | React frontend with filterable tables of the backend data |
| [`infra/`](infra/README.md) | Terraform + Helm: kind Kubernetes cluster running everything, monitored with OpenTelemetry and ClickStack |

## Getting started

The project uses [mise](https://mise.jdx.dev) to install Go 1.27, Python 3.12, Node 24, Terraform, kind, Helm and kubectl,
and to run every task.
PostgreSQL 14 runs in Docker, so Docker must be installed as well.

```bash
mise trust          # allow mise to load mise.toml (first time only)
mise install        # install Go and Python, and create generator/.venv
mise run install    # install the dependencies of the generator and the app
mise run dev        # create the kind cluster and deploy everything with Terraform + Helm
```

| Service | `mise run dev` (kind cluster) | `mise run dev:local` |
|---|---|---|
| Frontend | `http://localhost:30080` | `http://localhost:5173` |
| Backend | `http://localhost:30001` | `http://localhost:8001` |
| Generator | `http://localhost:30000` (API docs at `/docs`) | `http://localhost:8000` (API docs at `/docs`) |
| PostgreSQL 14 | Only inside the cluster | `localhost:5432` (user, password and database: `catnrat`) |
| HyperDX (ClickStack) | `http://localhost:30030` (create an account on the first visit) | — |

There are two ways to run the project:

- `mise run dev`: runs `infra:up`, which creates a [kind](https://kind.sigs.k8s.io) cluster with Terraform and
  deploys PostgreSQL, the backend, the generator and the frontend with Helm (see [`infra/`](infra/README.md)),
  plus [ClickStack](https://clickhouse.com/docs/use-cases/observability/clickstack): logs, metrics and traces
  are collected with OpenTelemetry, stored in ClickHouse and explored in HyperDX (see [Monitoring](infra/README.md#monitoring)).
  After a code change, run it again to rebuild the changed images and roll the pods.
- `mise run dev:local`: runs the backend, the generator and the frontend straight from the source code, with
  only PostgreSQL in Docker. Faster to start, and the frontend reloads on every change. Telemetry is off here,
  unless `OTEL_EXPORTER_OTLP_ENDPOINT` points to an OpenTelemetry Collector.

In both, the generator pushes one event per second to the backend. The local ports can be changed with
environment variables, e.g. `BACKEND_PORT=9001 GENERATOR_PORT=9000 FRONTEND_PORT=9002 mise run dev:local`.

## Tasks

Run `mise tasks` to see them all.

| Task | Description |
|---|---|
| `install` | Install the dependencies of the generator, the backend and the frontend |
| `dev` | Create the kind cluster and deploy everything (same as `infra:up`, then shows the pods) |
| `dev:local` | Run the backend, the generator and the frontend from source, with the database in Docker |
| `test` | Run all tests |
| `db:start` / `db:stop` | Start or stop PostgreSQL 14 (data is kept in the `catnrat-db-data` volume) |
| `db:reset` | Delete the database container and its data |
| `db:psql` | Open psql in the database |
| `db:migrate` | Apply all pending migrations |
| `db:rollback [N]` | Revert the last N migrations (default 1) |
| `db:version` | Print the current schema version |
| `backend:dev` / `generator:dev` / `frontend:dev` | Run only one of the services |
| `backend:lint` / `frontend:lint` | Check formatting and run `go vet` / type-check the frontend |
| `backend:test` / `generator:test` / `frontend:test` | Run the tests of one service |
| `backend:build` / `frontend:build` | Build the backend binary into `app/backend/bin/server` / the frontend into `app/frontend/dist` |
| `backend:image` / `generator:image` / `frontend:image` | Build the Docker image of one service |
| `infra:up` / `infra:down` | Create or destroy the kind cluster with everything deployed (see [`infra/`](infra/README.md)) |
| `infra:plan` | Show what Terraform would change |
| `infra:status` | Show the pods and services of the cluster |
| `infra:lint` | Check the Terraform code and lint the Helm charts |
| `monitoring:clickhouse ["SQL"]` | Open `clickhouse-client` in the ClickStack ClickHouse, or run a query |

To run everything in a local Kubernetes cluster instead, use `mise run infra:up` (see [`infra/`](infra/README.md)).
