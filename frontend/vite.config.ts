import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // Проксирование API к Go-бэкенду (FE-0011 API Client).
      // STAIR_API_PROXY_URL позволяет переопределить адрес (по умолчанию :8080).
      '/api': {
        target: process.env.STAIR_API_PROXY_URL ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
