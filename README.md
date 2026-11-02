# BitDash

A dashboard for the MMO [BitCraft](https://bitcraftonline.com/): pin player inventories, market items, crafting plans, skill progress and the live map side by side, and keep them on screen while you play. Game data comes from the community API [BitJita](https://bitjita.com/); BitDash caches it so many visitors share a small upstream budget.

Built with AI assistance (Cursor, Claude Code).

## Features

- **Dashboard** of pinned blocks with a saved layout per screen width. Each visitor gets their own dashboard, saved on the server under an anonymous cookie — no accounts.
- **Inventories** shown in their in-game pocket layout, with item icons and names.
- **Market**: prices, your listings against other sellers in the region, 7-day averages, completed trades.
- **Planner**: crafting plans.
- **Skills**: XP history and progress; an ore tab for Smithing with gathering speed and money per ingot.
- **Live map** of player positions via a WebSocket proxy.
- English and Russian UI.

## Architecture

```
browser (Vue 3)  ──►  Go backend  ──►  bitjita.com  (REST, relay, live WebSocket)
                      │
                      ├─ delivery/    HTTP handlers, WebSocket and map proxies,
                      │               per-visitor cookie, per-IP rate limit
                      ├─ usecase/     player lookup
                      ├─ repository/  BitJita client with memory + disk cache,
                      │               dashboard, XP history and item-name stores
                      └─ cache/       TTL cache
```

- **Backend:** Go standard library `net/http` plus `gorilla/websocket`. Layers: `delivery` → `usecase` → `repository`.
- **Shared upstream budget:** identical BitJita calls are shared and cached in memory and on disk, with a TTL that depends on how fast each kind of data changes. Every client IP gets a token bucket (`RATE_LIMIT_RPS`, `RATE_LIMIT_BURST`); behind Cloudflare the client IP is taken from `CF-Connecting-IP`.
- **Dashboard sync:** saves carry a revision, so a stale tab cannot overwrite newer blocks — it gets `409` and reloads the saved version instead. localStorage is only a cache.
- **Frontend:** Vue 3 + Vite, no UI framework. Playwright end-to-end tests.

## Run

With Docker (serves the built frontend and the API on port 8090):

```bash
docker compose up -d --build
```

For development:

```bash
cd backend && go run ./cmd/server
cd frontend && npm install && npm run dev
```

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8090` | HTTP port |
| `DASHBOARD_DIR` | `/var/lib/bitdash/dashboards` | One folder per visitor with their dashboard and its backups |
| `DASHBOARD_TEMPLATE_FILE` | — | A saved `dashboard.json` that new visitors start from |
| `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` | `10` / `300` | Requests per second and burst per client IP |
| `BITJITA_BASE_URL` | `https://bitjita.com` | Upstream API |
| `BITJITA_RELAY_URL` | `https://relay.bitjita.com` | Inventory relay, `off` to disable |
| `BITJITA_CACHE_DIR`, `XP_HISTORY_DIR`, `ITEM_NAME_FILE` | under `/var/lib/bitdash` | Caches and stores |

## Tests

```bash
cd backend && go test ./...
cd frontend && npx playwright install chromium && npm run test:e2e
```

The e2e config starts the Go backend (`:8099`, data in `frontend/.e2e-data`) and a Vite dev server (`:5174`), or reuses ones already running (override with `E2E_BACKEND_PORT` / `E2E_FRONTEND_PORT`). The map tests go through the live bitjita.com map, so they need network access.

## Dashboard backups

Each visitor's folder keeps earlier versions in `dashboard-backups/` (one every 10 minutes and before any save that removes blocks, last 200). To restore one:

```bash
docker compose exec bitdash ls /var/lib/bitdash/dashboards/<visitor>/dashboard-backups
docker compose exec bitdash cp /var/lib/bitdash/dashboards/<visitor>/dashboard-backups/<file> /var/lib/bitdash/dashboards/<visitor>/dashboard.json
```

The visitor ID is the `bitdash_visitor` cookie. Do not run `docker compose down -v` — it deletes the volume with all dashboards.
