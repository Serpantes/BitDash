import { defineConfig, devices } from '@playwright/test'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.dirname(fileURLToPath(import.meta.url))
const dataDir = path.join(root, '.e2e-data')
const BACKEND_PORT = Number(process.env.E2E_BACKEND_PORT || 8099)
const FRONTEND_PORT = Number(process.env.E2E_FRONTEND_PORT || 5174)
const BACKEND_URL = `http://127.0.0.1:${BACKEND_PORT}`
const reuse = !process.env.CI

export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  expect: { timeout: 20_000 },
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    ...devices['Desktop Chrome'],
    baseURL: `http://127.0.0.1:${FRONTEND_PORT}`,
    viewport: { width: 1400, height: 900 },
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: [
    {
      command: 'go run ./cmd/server',
      cwd: path.join(root, '../backend'),
      port: BACKEND_PORT,
      reuseExistingServer: reuse,
      timeout: 180_000,
      env: {
        PORT: String(BACKEND_PORT),
        BITJITA_CACHE_DIR: path.join(dataDir, 'bitjita-cache'),
        XP_HISTORY_DIR: path.join(dataDir, 'xp-history'),
        DASHBOARD_DIR: path.join(dataDir, 'dashboards'),
        ITEM_NAME_FILE: path.join(dataDir, 'item-names.json'),
      },
    },
    {
      command: `npx vite --port ${FRONTEND_PORT} --strictPort`,
      cwd: root,
      url: `http://127.0.0.1:${FRONTEND_PORT}/`,
      reuseExistingServer: reuse,
      timeout: 60_000,
      env: { VITE_BACKEND_URL: BACKEND_URL },
    },
  ],
})
