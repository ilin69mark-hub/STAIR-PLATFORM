import { expect, test } from '@playwright/test'

// E2E S3-2 (EDR-0040 Session Management): «сессия протухла → защищённый
// запрос вернул 401 → централизованный интерцептор → редирект на логин» для
// клиентского сайта (store).
//
// Честное «протухание» без ожидания реального истечения cookie: логинимся
// через реальный UI (#cabinet → AuthForm), затем перезаписываем httpOnly
// session-cookie «session» (store — без суффикса, см. auth.go sessionCookieFor)
// невалидным токеном прямо в браузерном контексте. Ин-апп навигация лендинг →
// #cabinet заново монтирует Cabinet, который делает GET /api/v1/orders с
// мёртвой cookie → 401 → fireUnauthorized() → user=null → AuthForm снова
// показывает логин.

const PASSWORD = 'E2e-password-123'

function uniqueEmail(): string {
  return `e2e-session-${Date.now()}-${Math.floor(Math.random() * 1e4)}@example.com`
}

test('сессия протухла → 401 → редирект на логин (store)', async ({ page, context }) => {
  const email = uniqueEmail()

  // Регистрация через реальный UI (store-клиент).
  await page.goto('/#cabinet')
  await page.getByRole('button', { name: 'Регистрация' }).click()
  await page.locator('#auth-name').fill('E2E Session')
  await page.locator('#auth-email').fill(email)
  await page.locator('#auth-password').fill(PASSWORD)
  await page.getByRole('button', { name: 'Войти в кабинет' }).click()
  await expect(page.getByRole('heading', { name: 'Мои заказы' })).toBeVisible({ timeout: 15_000 })

  // «Протухаем» сессию: перезаписываем httpOnly session-cookie мусорным
  // токеном (имя «session» — суффикс только у admin-приложения).
  await context.addCookies([
    {
      name: 'session',
      value: 'expired-invalid-secret-token',
      domain: 'localhost',
      path: '/',
      httpOnly: true,
    },
  ])

  // Ин-апп триггер защищённого запроса: лендинг → #cabinet (remount Cabinet
  // → GET /api/v1/orders с мёртвой cookie → 401 → fireUnauthorized).
  await page.locator('.store-brand').click()
  await page.locator('.store-nav a[href="#cabinet"]').click()

  // UI вернулся на логин: AuthForm (#auth-email) + «Личный кабинет».
  await expect(page.locator('#auth-email')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Личный кабинет' })).toBeVisible()
})