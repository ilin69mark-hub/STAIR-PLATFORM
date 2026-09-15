// STAIR PLATFORM — E2E (Playwright) клиентского сайта (store, :5174).
// webServer поднимает Vite (frontend-store) и Go API (:8080, реальная БД).
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e/tests',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // Серийная гонка: бэкенд дедуплицирует тяжёлые POST (расчёт quote) по
  // IP+метод+путь и параллельные e2e с одного IP получали 429
  // «duplicate request in progress» (флаки). Одного воркера достаточно.
  workers: 1,
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
