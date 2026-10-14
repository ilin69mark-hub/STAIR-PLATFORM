// E2E: блоки магазина в кабинете. Спек перенесён из удалённого отдельного
// приложения frontend-store-admin, потом переписан со вкладок на плитки: прайс
// и ставки уехали в «Цены и расчёт», контакты — в «Сайт и витрина», заказы и
// платежи — в «Продажи и деньги».
//
// Разделы находят по id секции плитки (#pricing-materials, #site-contacts),
// а не по вкладкам: у вкладок больше нет.
//
// Роль admin назначается только через БД (SEC-0004), поэтому админ создаётся
// регистрацией + promoteToAdmin — см. helpers/db.ts.
import { expect, test, type Page } from '@playwright/test'
import { login, register, uniqueEmail } from '../helpers/auth'
import { promoteToAdmin } from '../helpers/db'

const adminEmail = uniqueEmail()
const regularEmail = uniqueEmail()

async function loginAsAdmin(page: Page): Promise<void> {
  await login(page, adminEmail)
}

/** Открыть плитку по названию. */
async function openTile(page: Page, title: string): Promise<void> {
  await page.getByRole('button', { name: new RegExp(`^${title}`) }).click()
  await expect(page.getByRole('heading', { name: title })).toBeVisible()
}

async function csrfToken(page: Page): Promise<string> {
  const cookie = (await page.context().cookies()).find((item) => item.name === 'csrf_admin')
  if (!cookie) throw new Error('csrf_admin cookie не найдена')
  return cookie.value
}

test.describe.serial('кабинет: блоки магазина', () => {
  test.setTimeout(300_000)

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext()
    const page = await ctx.newPage()
    await register(page, adminEmail)
    await page.locator('#project-name').waitFor()
    promoteToAdmin(adminEmail)
    // Обычный пользователь для проверки отрицательного случая: плиток магазина
    // и «Команды» у него быть не должно — прав не выдавали.
    await register(page, regularEmail)
    await ctx.close()
  })

  test('админ открывает блоки магазина', async ({ page }) => {
    await loginAsAdmin(page)
    // Вернуться с плитки «Проекты» (куда приводит login) на приветственный
    // экран и убедиться, что все блоки на месте.
    await page.getByRole('button', { name: '← Проекты' }).click()
    for (const title of ['Сайт и витрина', 'Цены и расчёт', 'Продажи и деньги']) {
      await expect(page.getByRole('button', { name: new RegExp(`^${title}`) })).toBeVisible()
    }
  })

  test('прайс материалов редактируется и витрина видит новую цену', async ({ page }) => {
    await loginAsAdmin(page)
    await openTile(page, 'Цены и расчёт')
    // Ставки расчёта и цены материалов — в одной плитке, одним скроллом.
    await expect(page.getByLabel('Маржа, %')).toBeVisible()

    const input = page.getByLabel('Цена WOOD-OAK')
    const row = page.getByRole('row', { name: /WOOD-OAK/ })
    await input.fill('4321')
    await row.getByRole('button', { name: 'Сохранить' }).click()
    await expect(page.getByText('Цена WOOD-OAK сохранена')).toBeVisible()

    // Публичный каталог отдаёт цену магазина (кэш сброшен бэкендом).
    const materials = await page.request.get('/api/v1/public/materials')
    expect(materials.ok()).toBeTruthy()
    const catalog = (await materials.json()) as { code: string; price_per_kg_rub: number }[]
    expect(catalog.find((m) => m.code === 'WOOD-OAK')?.price_per_kg_rub).toBe(4321)

    // Возвращаем встроенную ставку, чтобы тест не менял прайс магазина.
    await row.getByRole('button', { name: 'Сбросить' }).click()
    await expect(page.getByText(/возвращена к встроенной ставке/)).toBeVisible()
  })

  test('настройки магазина сохраняются и видны в публичном ручке', async ({ page }) => {
    await loginAsAdmin(page)
    const originalResponse = await page.request.get('/api/v1/admin/store/settings', {
      headers: { 'X-App-Origin': 'admin' },
    })
    expect(originalResponse.ok()).toBeTruthy()
    const original = (await originalResponse.json()) as Record<string, unknown>
    const csrf = await csrfToken(page)

    try {
      await openTile(page, 'Сайт и витрина')
      const phone = page.getByLabel('Телефон')
      await phone.fill('+7 900 555-11-22')
      await page.locator('#site-contacts').getByRole('button', { name: 'Сохранить' }).click()
      await expect(page.getByText('Контакты и адрес сохранены')).toBeVisible()

      const publicSettings = await page.request.get('/api/v1/public/store-settings')
      expect(publicSettings.ok()).toBeTruthy()
      const body = (await publicSettings.json()) as { contacts: { phone: string } }
      expect(body.contacts.phone).toBe('+7 900 555-11-22')
      // Публичное зеркало не должно протекать внутренними полями расчёта.
      expect(JSON.stringify(body)).not.toContain('margin_percent')
    } finally {
      const restore = await page.request.put('/api/v1/admin/store/settings', {
        headers: {
          'Content-Type': 'application/json',
          'X-App-Origin': 'admin',
          'X-CSRF-Token': csrf,
        },
        data: original,
      })
      expect(restore.ok()).toBeTruthy()
    }
  })

  test('обычный пользователь видит только плитку «Проекты»', async ({ page }) => {
    // Прав не выдавали: блоки магазина и «Команда» не должны ни появиться,
    // ни открыться по прямой ссылке — их просто нет.
    await login(page, regularEmail)
    await page.getByRole('button', { name: '← Проекты' }).click()
    await expect(page.getByRole('button', { name: /^Проекты/ })).toBeVisible()
    for (const title of ['Сайт и витрина', 'Цены и расчёт', 'Продажи и деньги', 'Команда']) {
      await expect(page.getByRole('button', { name: new RegExp(`^${title}`) })).toHaveCount(0)
    }
  })
})