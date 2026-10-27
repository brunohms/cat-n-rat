# frontend

React UI (Vite + TypeScript) that shows the backend data in two tables:

- **Purchases**: search by order, customer name or email; filters by status, channel, payment method, source,
  customer, date range and total range; click a row to see its items.
- **Customers**: search by ID, name or email; filters by state and city; purchase count, total spent and
  last purchase; "View purchases" opens the purchases of that customer.

Both tables sort by clicking the column headers and are paginated.

Filtering, sorting and pagination are always done by the backend: every change sends a new request with the
filters as query parameters. Text fields wait until the user stops typing (350 ms) before sending. Filters,
sorting and page are kept in the URL, so a filtered view can be shared and the back button works.
"Auto-refresh" reloads the current page every 5 seconds.

## Running

From the repository root, using [mise](https://mise.jdx.dev) (see the [root README](../../README.md)):

```bash
mise run dev:local       # database, backend, generator and frontend from source on http://localhost:5173
mise run dev             # everything in a kind cluster, frontend on http://localhost:30080
mise run frontend:dev    # only the frontend (expects the backend on localhost:$BACKEND_PORT)
```

## Talking to the backend

The browser only calls `/api/...` on the frontend origin, so the backend needs no CORS configuration:

- **Development**: the Vite dev server proxies `/api` to `http://localhost:$BACKEND_PORT` (default `8001`),
  or to `BACKEND_URL` when set.
- **Docker / Kubernetes**: nginx serves the built files and proxies `/api` to `BACKEND_URL`
  (default `http://backend:8000`), see `nginx.conf.template`.

## Tasks

| Task | Description |
|---|---|
| `mise run frontend:test` | Run the tests (Vitest + Testing Library) |
| `mise run frontend:lint` | Type-check |
| `mise run frontend:build` | Build into `dist/` |
| `mise run frontend:image` | Build the Docker image (`catnrat-frontend`) |

## Layout

| Path | Description |
|---|---|
| `src/api.ts` | API client and types; builds the query string from the filters |
| `src/views/` | Purchases and customers tables with their filters |
| `src/components/` | Debounced input, select field, sortable header, pagination and table states |
| `src/useUrlParams.ts` | Keeps the filters in the URL query string |
