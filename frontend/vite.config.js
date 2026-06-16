import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_BACKEND_URL || 'http://127.0.0.1:8090',
        changeOrigin: true,
        ws: true,
        // Match slow /api/item-names (many upstream lookups); avoid proxy socket bailouts.
        timeout: 180_000,
        proxyTimeout: 180_000,
      },
      '/map-embed': {
        target: process.env.VITE_BACKEND_URL || 'http://127.0.0.1:8090',
        changeOrigin: true,
      },
    },
  },
})
