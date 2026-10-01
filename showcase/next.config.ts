import type { NextConfig } from 'next'

// Витрина (этап 3) обращается к тому же Go-API, что и витрина-калькулятор
// frontend-store: /api/* проксируется на :8080, /static-assets/* отдаёт
// PBR-текстуры и HDRI этапа 1. В проде адрес API задаётся переменной
// STAIR_API_URL (по умолчанию http://localhost:8080).
const api = process.env.STAIR_API_URL ?? 'http://localhost:8080'

const nextConfig: NextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      { source: '/api/:path*', destination: `${api}/api/:path*` },
      { source: '/static-assets/:path*', destination: `${api}/static-assets/:path*` },
    ]
  },
}

export default nextConfig
