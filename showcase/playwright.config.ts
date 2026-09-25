import { defineConfig, devices } from '@playwright/test'

// E2E витрины (этап 3). Требует живого Go-API на :8080 (каталог материалов
// и примеры считаются через него), который поднимается отдельно: конфиг
// витрины поднимает только Next.
export default defineConfig({
  testDir: './e2e/tests',
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? 'github' : 'line',
  use: {
    baseURL: process.env.SHOWCASE_URL ?? 'http://localhost:5176',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'npm run dev',
    url: process.env.SHOWCASE_URL ?? 'http://localhost:5176',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
})
