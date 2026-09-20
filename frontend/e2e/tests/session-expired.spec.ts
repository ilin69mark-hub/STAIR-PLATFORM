import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'

// E2E S3-2 (EDR-0040 Session Management): «сессия протухла → защищённый
// запрос вернул 401 → централизованный интерцептор → редирект на логин» для
// админ-приложения (admin :5173 dev под Playwright).
//
// Честное «протухание» без ожидания реального истечения cookie: регистрируемся
// через реальный UI (helper.register), затем перезаписываем httpOnly
// session-cookie «session_admin» (суффикс _admin — у admin только свой
// namespace, см. auth.go sessionCookieFor) невалидным токеном прямо в
// браузерном контексте. Ин-апп триггер защищённого запроса — создание проекта
// (POST /api/v1/projects) с мёртвой cookie → 401 → fireUnauthorized() →
// user=null → App рендерит AuthPage (логин).

test('сессия протухла → 401 → редирект на логин (admin)', async ({ page, context }) => {
  const email = uniqueEmail()
  await register(page, email)
  await expect(page.locator('#project-name')).toBeVisible()

  // «Протухаем» сессию: перезаписываем httpOnly session-cookie мусорным
  // токеном (admin-приложение — имя «session_admin»).
  await context.addCookies([
    {
      name: 'session_admin',
      value: 'expired-invalid-secret-token',
      domain: 'localhost',
      path: '/',
      httpOnly: true,
    },
  ])

  // Ин-апп триггер защищённого запроса: создание проекта — POST /api/v1/projects
  // с мёртвой cookie → 401 → fireUnauthorized.
  await page.locator('#project-name').fill('E2E истёкшая сессия')
  await page.getByRole('button', { name: 'Создать проект' }).click()

  // UI вернулся на логин: AuthPage (#auth-email).
  await expect(page.locator('#auth-email')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'STAIR PLATFORM' })).toBeVisible()
})