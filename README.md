# BitDash

Simple project: Go backend + Vue3 frontend.

- Backend: Go HTTP server that proxies requests to an external API and returns JSON.
- Frontend: Vue 3 + Vite dashboard that fetches `/api/data` and renders a chart.

Run backend:

```bash
cd backend
go run ./
```

Run frontend:

```bash
cd frontend
npm install
npm run dev
```

Configure external API URL via `EXTERNAL_API_URL` env var for backend (default in code is `https://api.example.com/data`).

E2E tests (Playwright):

```bash
cd frontend
npx playwright install chromium   # once
npm run test:e2e
```

The config starts the Go backend (`go run ./cmd/server` on `:8099`, data in `frontend/.e2e-data`) and a Vite dev server (`:5174`), or reuses ones already running on those ports (override with `E2E_BACKEND_PORT` / `E2E_FRONTEND_PORT`). The map tests go through the live bitjita.com/map proxy, so they need network access.

Dashboard blocks:

The pinned blocks and their layout are saved on the server in `DASHBOARD_FILE` (`/var/lib/bitdash/dashboard.json` in the `bitdash-data` Docker volume), not just in the browser, so clearing site data or opening the dashboard from another address or browser no longer resets it. localStorage is only a cache. Every save carries a revision, so a stale tab cannot overwrite newer blocks — it reloads with the saved version instead. Earlier versions are kept in `dashboard-backups/` next to the file (one every 10 minutes and before any save that removes blocks, last 200). To restore one:

```bash
docker compose exec bitdash ls /var/lib/bitdash/dashboard-backups
docker compose exec bitdash cp /var/lib/bitdash/dashboard-backups/<file> /var/lib/bitdash/dashboard.json
```

Do not run `docker compose down -v` — it deletes the volume with the dashboard.
