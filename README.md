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

The config starts the Go backend (`go run ./cmd/server` on `:8099`, data in `frontend/.e2e-data`) and a Vite dev server (`:5174`), or reuses ones already running on those ports (override with `E2E_BACKEND_PORT` / `E2E_FRONTEND_PORT`). The map tests go through the live bitcraftmap.com proxy, so they need network access.
