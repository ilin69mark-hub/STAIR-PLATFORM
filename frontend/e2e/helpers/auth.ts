// E2E: авторизация через UI (SEC-0003). Регистрируем уникального пользователя,
// т.к. БД персистентна (docker-compose) — повторный email даст 409.
import { expect, type Page } from '@playwright/test'

export function uniqueEmail(): string {
  return `e2e-${Date.now()}-${Math.floor(Math.random() * 1e4)}@example.com`
}

const PASSWORD = 'E2e-password-123'

/**
 * Открыть плитку «Проекты».
 *
 * После входа человек попадает на приветственный экран с плитками, а форма
 * создания проекта живёт внутри плитки «Проекты». Пока верхним уровнем были
 * вкладки, список проектов открывался сразу — поэтому почти каждый спек
 * начинался с ожидания #project-name и падал бы уже на первом шаге.
 */
export async function openProjectsTile(page: Page) {
  await page.getByRole('button', { name: /^Проекты/ }).click()
  await page.locator('#project-name').waitFor()
}

export async function register(page: Page, email: string) {
  await page.goto('/')
  await page.getByRole('button', { name: 'Регистрация' }).click()
  await page.locator('#auth-name').fill('E2E User')
  await page.locator('#auth-email').fill(email)
  await page.locator('#auth-password').fill(PASSWORD)
  await page.getByRole('button', { name: 'Зарегистрироваться' }).click()
  await expect(page.getByRole('heading', { name: 'STAIR PLATFORM' })).toBeVisible()
  await openProjectsTile(page)
}

export async function login(page: Page, email: string) {
  await page.goto('/')
  await page.locator('#auth-email').fill(email)
  await page.locator('#auth-password').fill(PASSWORD)
  await page.getByRole('button', { name: 'Войти' }).click()
  // После входа — приветственный экран; добираемся до плитки «Проекты».
  await page.getByRole('button', { name: /^Проекты/ }).waitFor()
  await openProjectsTile(page)
}

export { PASSWORD }
