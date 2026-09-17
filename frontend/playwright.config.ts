// STAIR PLATFORM — E2E (Playwright) конфигурация (TEST-0022).
// webServer поднимает Vite (frontend, :5173) и Go API (:8080, реальная БД
// на localhost:5432 по умолчанию — как в make up). В CI API стартует из
// репозитория, БД — postgres service (миграции накатываются шагом джобы).
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e/tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI
    ? [['list'], ['html', { open: 'never' }]]
    : 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      command: 'npm run dev',
      url: 'http://localhost:5173',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
    {
      command:
        'STAIR_DATABASE_URL=postgres://stair:stair@localhost:5432/stair_platform?sslmode=disable STAIR_REGISTER_RATE_LIMIT=5000 STAIR_LOGIN_RATE_LIMIT=10000 STAIR_AUTH_RATE_LIMIT=10000 go run ./cmd/api',
      url: 'http://localhost:8080/health',
      cwd: '..',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
  ],
})
