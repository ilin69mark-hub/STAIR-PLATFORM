import type { FullConfig } from '@playwright/test'

// Global setup панели магазина: готовит администратора tenant'а.
//
// Локальная БД общая с Go-тестами (STAIR_TEST_DATABASE_URL), а тесты чистят
// таблицы — зарегистрированный вручную админ исчезает. Поэтому перед прогоном
// мы идем в API сами: пробуем войти, при неудаче регистрируем и выдаём роль
// admin через прямой SQL-путь невозможно — поэтому берём демо-админа из
// окружения: STORE_ADMIN_E2E_EMAIL/PASSWORD. Если их не задали, тесты
// пропускаются (describe.skip) с понятным сообщением.
export default async function globalSetup(_config: FullConfig) {
  const api = process.env.STAIR_API_URL ?? 'http://localhost:8080'
  const email = process.env.STORE_ADMIN_E2E_EMAIL
  const password = process.env.STORE_ADMIN_E2E_PASSWORD
  if (!email || !password) return

  const login = await fetch(`${api}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-App-Origin': 'admin' },
    body: JSON.stringify({ email, password }),
  })
  if (login.ok) return

  const reg = await fetch(`${api}/api/v1/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-App-Origin': 'admin' },
    body: JSON.stringify({ email, password, name: 'Store Admin E2E' }),
  })
  if (!reg.ok && reg.status !== 409) {
    throw new Error(`store-admin e2e setup: не удалось войти/зарегистрировать ${email}: HTTP ${reg.status}`)
  }
}
