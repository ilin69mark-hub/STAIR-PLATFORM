import { fileURLToPath, URL } from 'node:url'
import { defineConfig, type PluginOption } from 'vite'
import react from '@vitejs/plugin-react'
import { sentryVitePlugin } from '@sentry/vite-plugin'

// S-140: sourcemaps генерируются всегда (build.sourcemap), заливка в Sentry —
// только при наличии SENTRY_AUTH_TOKEN (+ SENTRY_ORG/SENTRY_PROJECT). CI без
// секретов зелёный: без токена плагин в сборку не включается (конфиг —
// «выключатель»; реальную заливку карт человек подключит после S-118).
function sentryPlugins(): PluginOption[] {
  const authToken = process.env.SENTRY_AUTH_TOKEN
  const org = process.env.SENTRY_ORG
  const project = process.env.SENTRY_PROJECT
  if (!authToken || !org || !project) return []
  return [
    sentryVitePlugin({
      authToken,
      org,
      project,
      sourcemaps: {
        assets: ['./dist/assets/**'],
        filesToDeleteAfterUpload: './dist/assets/*.map',
      },
    }),
  ]
}

// Клиентский сайт (store) компании: Vite + React 19 + TypeScript.
// Порт :5174 в dev; общий модуль @shared переиспользуется из панели.
export default defineConfig({
  plugins: [react(), ...sentryPlugins()],
  build: {
    // S-140: карты для отладки ошибок (заливаются при SENTRY_AUTH_TOKEN).
    sourcemap: true,
  },
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
