import type { FullConfig } from '@playwright/test'

// Global setup витрины: готовит e2e-пользователя.
//
// Локальная БД общая с Go-тестами (STAIR_TEST_DATABASE_URL), а тесты чистят
// таблицы — зарегистрированный вручную пользователь исчезает. Поэтому перед
// прогоном мы идем в API сами: пробуем войти, при неудаче регистрируем.
export default async function globalSetup(_config: FullConfig) {
  const api = process.env.STAIR_API_URL ?? 'http://localhost:8080'
  const email = process.env.STORE_E2E_EMAIL ?? 'user@user.ru'
  const password = process.env.STORE_E2E_PASSWORD ?? 'user1234'

  const login = await fetch(`${api}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (login.ok) return

  const reg = await fetch(`${api}/api/v1/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, name: 'E2E User' }),
  })
  if (!reg.ok && reg.status !== 409) {
    throw new Error(`e2e global setup: не удалось завести пользователя ${email}: HTTP ${reg.status}`)
  }
}
