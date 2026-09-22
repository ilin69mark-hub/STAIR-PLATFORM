import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      // Общий модуль (shared): типы, форматтеры, схемы, 3D-вьювер —
      // переиспользуются панелью и клиентским сайтом (store).
      '@shared': fileURLToPath(new URL('./shared/src', import.meta.url)),
    },
  },
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
      // S-132b: dev-прокси WebSocket realtime (upgrade) — зеркалит nginx
      // location /ws из прод-конфига (S-121). changeOrigin=false — тот же
      // Host для same-origin проверки (S-115).
      '/ws': {
        target: process.env.STAIR_API_PROXY_URL ?? 'http://localhost:8080',
        changeOrigin: false,
        ws: true,
      },
    },
  },
})