import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        // Match slow /api/item-names (many upstream lookups); avoid proxy socket bailouts.
        timeout: 180_000,
        proxyTimeout: 180_000,
      },
    },
  },
})
