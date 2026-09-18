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
    // strictPort: 5174 занят Docker-контейнером админки (admin:5174->80) —
    // без strictPort Vite молча уезжает на 5175, а Playwright webServer.url
    // пингует 5174 (уже «жив» админкой) и подхватывает не тот dev-сервер.
    port: 5175,
    strictPort: true,
    proxy: {
      '/api': {
        target: process.env.STAIR_API_PROXY_URL ?? 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
})
