# app

| Folder | Description |
|---|---|
| [`backend/`](backend/README.md) | Go API that receives, requests and stores the purchase events in PostgreSQL 14 |
| [`frontend/`](frontend/README.md) | React UI with filterable tables of the backend data |

```bash
mise run dev          # everything in a kind cluster; open http://localhost:30080
mise run dev:local    # everything from source; open http://localhost:5173
```
