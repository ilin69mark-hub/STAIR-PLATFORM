// STAIR PLATFORM — E2E панели магазина (волна 0). webServer поднимает Vite
// (:5177) и Go API; тесты идут через vite proxy к реальному бэкенду.
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e/tests',
  globalSetup: './e2e/global-setup.ts',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: 'http://localhost:5177',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      command: 'npm run dev',
      url: 'http://localhost:5177',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
    {
      command: 'go run ./cmd/api',
      cwd: '..',
      url: 'http://localhost:8080/health',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: {
        STAIR_ENVIRONMENT: 'development',
        // E2E гоняет вход и расчёты десятками прогонов: поднимаем лимиты,
        // иначе login-limiter отвечает 429 и тесты падают не по делу.
        STAIR_LOGIN_RATE_LIMIT: '600',
        STAIR_REGISTER_RATE_LIMIT: '600',
        STAIR_QUOTE_RATE_LIMIT: '600',
        STAIR_VALIDATE_RATE_LIMIT: '600',
        STAIR_AUTH_RATE_LIMIT: '6000',
        STAIR_DATABASE_URL:
          process.env.STAIR_TEST_DATABASE_URL ??
          process.env.STAIR_DATABASE_URL ??
          'postgres://stair:stair@localhost:5432/stair_test?sslmode=disable',
        STAIR_CSRF_ALLOWED_ORIGINS: 'http://localhost:5177,http://localhost:5173,http://localhost:5175',
      },
    },
  ],
})
