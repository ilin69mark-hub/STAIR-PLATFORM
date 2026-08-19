import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Клиентский сайт (store) компании: Vite + React 19 + TypeScript.
// Порт :5174 в dev; общий модуль @shared переиспользуется из панели.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@shared': fileURLToPath(new URL('../frontend/shared/src', import.meta.url)),
    },
  },
  server: {
    port: 5174,
    proxy: {
      '/api': {
        target: process.env.STAIR_API_PROXY_URL ?? 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
})
