import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Панель управления магазином (волна 0 «store admin»): Vite + React 19 + TS.
// Порт :5177 в dev — отдельный от admin (:5173) и store (:5175).
export default defineConfig({
  plugins: [react()],
  build: {
    sourcemap: true,
  },
  resolve: {
    alias: {
      '@shared': fileURLToPath(new URL('../frontend/shared/src', import.meta.url)),
    },
  },
  server: {
    port: 5177,
    strictPort: true,
    proxy: {
      '/api': {
        target: process.env.STAIR_API_PROXY_URL ?? 'http://localhost:8080',
        changeOrigin: false,
      },
      '/static-assets': {
        target: process.env.STAIR_API_PROXY_URL ?? 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
})
