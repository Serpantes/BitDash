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
