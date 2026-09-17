// E2E: админ-панель (Phase G, EDR-0016). Полный контур: вход админом →
// «Администрирование» → смена роли, политика безопасности, API-ключи.
// Админ создаётся напрямую в БД (SEC-0004: роль admin назначается только
// через БД/seed), поэтому после регистрации повышаем пользователя через psql
// в контейнере stair-platform-postgres (локальный стек make up).
import { expect, test } from '@playwright/test'
import { execSync } from 'node:child_process'
import { login, register, uniqueEmail } from '../helpers/auth'

const adminEmail = uniqueEmail()
const regularEmail = uniqueEmail()

function promoteToAdmin(email: string) {
  const sql = `UPDATE users SET role='admin' WHERE email='${email}' AND role='user' RETURNING id`
  const out = execSync(
    `docker exec stair-platform-postgres psql -U stair -d stair_platform -tAc "${sql}"`,
    { encoding: 'utf8' },
  ).trim()
  if (!out) throw new Error(`Пользователь ${email} не найден — не удалось повысить права`)
}

test.describe.serial('админ-панель', () => {
  test.setTimeout(300_000)

  test.beforeAll(async ({ browser }) => {
    // Подчищаем ключи прежних запусков (имена E2E-CI-*) — спек сам себя
    // убирает ключи через отзыв, но оборванные прогоны могут наследить.
    execSync(
      `docker exec stair-platform-postgres psql -U stair -d stair_platform -tAc "DELETE FROM api_keys WHERE name LIKE 'E2E-CI-%'"`,
      { encoding: 'utf8' },
    )
    const ctx = await browser.newContext()
    const page = await ctx.newPage()
    await register(page, adminEmail)
    await page.locator('#project-name').waitFor()
    promoteToAdmin(adminEmail)
    await ctx.close()
  })

  test('админ открывает панель и видит разделы', async ({ page }) => {
    await login(page, adminEmail)
    await page.getByRole('button', { name: 'Администрирование' }).click()
    await expect(page.getByRole('heading', { name: 'Администрирование' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Пользователи' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'API-ключи' })).toBeVisible()
    await expect(page.locator('.toc__list')).toContainText('Экспорт')
    await expect(page.locator('.toc__list')).toContainText('API-ключи')
  })

  test('админ меняет роль обычного пользователя', async ({ page, browser }) => {
    // Регистрируем рядового пользователя в отдельном контексте, чтобы его
    // сессия не «просочилась» в контекст теста (иначе login будет на списке).
    const otherCtx = await browser.newContext()
    const other = await otherCtx.newPage()
    await register(other, regularEmail)
    await otherCtx.close()

    await login(page, adminEmail)
    await page.getByRole('button', { name: 'Администрирование' }).click()

    const roleSelect = page.locator(`select[aria-label="Роль ${regularEmail}"]`)
    await expect(roleSelect).toBeVisible()
    await expect(roleSelect).toHaveValue('user')
    await roleSelect.selectOption('admin')

    await expect(page.getByText('Пользователь обновлён')).toBeVisible()
    await expect(roleSelect).toHaveValue('admin')
  })

  test('админ обновляет политику безопасности', async ({ page }) => {
    await login(page, adminEmail)
    await page.getByRole('button', { name: 'Администрирование' }).click()

    const minLen = page.locator('#min-len')
    await expect(minLen).toBeVisible()
    await minLen.fill('10')
    await page.getByRole('button', { name: 'Сохранить политику' }).click()
    await expect(page.getByText('Политика безопасности сохранена')).toBeVisible()
    await expect(minLen).toHaveValue('10')

    // Возвращаем дефолт, чтобы не менять политику для остальных спеков.
    await minLen.fill('8')
    await page.getByRole('button', { name: 'Сохранить политику' }).click()
    await expect(page.getByText('Политика безопасности сохранена')).toBeVisible()
    await expect(minLen).toHaveValue('8')
  })

  test('админ создаёт и отзывает API-ключ', async ({ page }) => {
    await login(page, adminEmail)
    await page.getByRole('button', { name: 'Администрирование' }).click()

    const keys = page.locator('#api-keys')
    await expect(keys).toBeVisible()

    // Создание: токен показывается сразу после создания.
    const keyName = `E2E-CI-${Date.now()}`
    await keys.locator('input[placeholder="Имя ключа (например, CI)"]').fill(keyName)
    await keys.locator('input[placeholder="Scopes через запятую (users.list)"]').fill('users.list,data.export')
    await keys.locator('button[type="submit"]').click()

    await expect(keys.locator('.token-code')).toBeVisible()
    const token = (await keys.locator('.token-code').textContent())?.trim() ?? ''
    expect(token.length).toBeGreaterThan(10)

    // Токен одноразовый: после перезагрузки его уже нет, в списке ключ есть.
    await page.reload()
    await expect(page.getByRole('button', { name: 'Администрирование' })).toBeVisible()
    await page.getByRole('button', { name: 'Администрирование' }).click()
    const keyRow = keys.locator('.comment', { hasText: keyName })
    await expect(keyRow).toBeVisible()
    await expect(keys.locator('.token-code')).toHaveCount(0)
    await expect(keyRow).not.toContainText('sk-')

    // Отзыв: ключ помечается «Отозван», кнопка отзыва исчезает.
    await keyRow.getByRole('button', { name: 'Отозвать' }).click()
    await expect(page.getByText('API-ключ отозван')).toBeVisible()
    await expect(keyRow.locator('.comment__date')).toContainText('Отозван')
    await expect(keyRow.getByRole('button', { name: 'Отозвать' })).toHaveCount(0)
  })
})