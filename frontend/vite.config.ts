import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // Проксирование API к Go-бэкенду (FE-0011 API Client).
      // changeOrigin=false сохраняет Host браузера (localhost:5173), чтобы
      // проверка Origin/Referer CSRF (EDR-0014 §3.3) проходила в dev/proxy.
      // STAIR_API_PROXY_URL позволяет переопределить адрес (по умолчанию :8080).
      '/api': {
        target: process.env.STAIR_API_PROXY_URL ?? 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
})
