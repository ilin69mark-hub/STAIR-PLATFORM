// STAIR PLATFORM — E2E (Playwright) клиентского сайта (store, :5174).
// webServer поднимает Vite (frontend-store) и Go API (:8080, реальная БД).
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e/tests',
// Матрица 2000 генерирует все тесты в одном файле: без fullyParallel они
  // шли бы серийно в одном воркере. Параллельность безопасна: дедупликация
  // бэкенда учитывает хэш тела (разные payload не сталкиваются в 429).
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // Дедупликация бэкенда учитывает хэш тела (dedup.go: DeduplicateByKey),
  // поэтому параллельные воркеры с разными payload не получают ложный 429
  // «duplicate request in progress». Повтор того же тела по-прежнему давится.
  workers: 4,
  reporter: process.env.CI
    ? [['list'], ['html', { open: 'never' }]]
    : 'list',
  use: {
    // Локальная разработка — Vite dev (:5174); готовый стек (Docker) — store :3000.
    baseURL: process.env.STORE_BASE_URL || 'http://localhost:5174',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      command: 'npm run dev',
      url: 'http://localhost:5174',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
  ],
})
