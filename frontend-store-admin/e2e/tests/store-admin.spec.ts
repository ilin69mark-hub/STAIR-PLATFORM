import { expect, test, type Page } from '@playwright/test'

// E2E панели магазина (волна 0): вход администратора, разделы, правка прайса и
// настроек. Требует STORE_ADMIN_E2E_EMAIL/PASSWORD с ролью admin — без них
// тесты пропускаются, чтобы CI оставался зелёным без секретов.

const email = process.env.STORE_ADMIN_E2E_EMAIL
const password = process.env.STORE_ADMIN_E2E_PASSWORD
const hasAdmin = Boolean(email && password)

// Вход через форму панели: так e2e проверяет реальный путь пользователя
// (cookie выставляется ответом страницы, гонок с context.request нет).
async function login(page: Page) {
  await page.goto('/')
  await page.getByLabel('Email').fill(email as string)
  await page.getByLabel('Пароль').fill(password as string)
  await page.getByRole('button', { name: 'Войти' }).click()
  await expect(page.getByRole('heading', { name: 'Обзор' })).toBeVisible()
}

async function csrfToken(page: Page): Promise<string> {
  const cookie = (await page.context().cookies()).find((item) => item.name === 'csrf_admin')
  if (!cookie) throw new Error('csrf_admin cookie не найдена')
  return cookie.value
}

test.describe('панель магазина', () => {
  test.skip(!hasAdmin, 'нужен STORE_ADMIN_E2E_EMAIL/PASSWORD (роль admin)')

  test('вход и разделы панели доступны администратору', async ({ page }) => {
    await login(page)
    await expect(page.getByRole('heading', { name: 'Обзор' })).toBeVisible()
    for (const title of ['Цены', 'Настройки', 'Заказы', 'Метрики']) {
      await expect(page.getByRole('button', { name: title })).toBeVisible()
    }
  })

  test('прайс материалов редактируется и витрина видит новую цену', async ({ page }) => {
    await login(page)
    await page.getByRole('button', { name: 'Цены' }).click()
    await expect(page.getByRole('heading', { name: 'Цены материалов' })).toBeVisible()

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
    await login(page)
    const originalResponse = await page.request.get('/api/v1/admin/store/settings', {
      headers: { 'X-App-Origin': 'admin' },
    })
    expect(originalResponse.ok()).toBeTruthy()
    const original = (await originalResponse.json()) as Record<string, unknown>
    const csrf = await csrfToken(page)

    try {
      await page.getByRole('button', { name: 'Настройки' }).click()
      const phone = page.getByLabel('Телефон')
      await phone.fill('+7 900 555-11-22')
      await page.getByRole('button', { name: 'Сохранить настройки' }).click()
      await expect(page.getByText('Настройки сохранены')).toBeVisible()

      const publicSettings = await page.request.get('/api/v1/public/store-settings')
      expect(publicSettings.ok()).toBeTruthy()
      const body = (await publicSettings.json()) as { contacts: { phone: string } }
      expect(body.contacts.phone).toBe('+7 900 555-11-22')
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
})
