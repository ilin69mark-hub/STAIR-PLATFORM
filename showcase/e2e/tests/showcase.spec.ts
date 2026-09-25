import { expect, test } from '@playwright/test'

// E2E витрины (этап 3). Витрина — серверные страницы: проверяем, что данные
// приходят с Go-API, материалы видны по-русски, 3D-сцены строятся, а
// калькулятор считает. Требует живого API на :8080.

test('главная: каталог материалов и 3D-пример считаются API', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(String(e?.message ?? e)))

  await page.goto('/')

  // Каталог: семь материалов с русскими названиями и ценой за кг.
  await expect(page.getByRole('heading', { name: 'Материалы' })).toBeVisible()
  for (const name of ['Сталь S235', 'Кортэн', 'Алюминий 5083', 'Дуб', 'Орех', 'Ясень', 'Сосна']) {
    await expect(page.getByText(name, { exact: true }).first()).toBeVisible()
  }
  await expect(page.getByText(/₽\/кг/).first()).toBeVisible()

  // 3D-пример: сервер отдал меш, клиентский остров построил сцену.
  const scene = page.locator('.scene').first()
  await expect(scene).toBeVisible()
  await expect(scene.locator('canvas')).toBeVisible({ timeout: 20_000 })

  expect(errors, `ошибки страницы: ${errors.join('; ')}`).toHaveLength(0)
})

test('спираль не упоминается на витрине (S-152)', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('body')).not.toContainText('спирал', { timeout: 10_000 })
  await expect(page.locator('body')).not.toContainText('Спирал')
})

test('страница материалов: таблица, цены и переход в карточку', async ({ page }) => {
  await page.goto('/materials')
  await expect(page.getByRole('heading', { name: 'Материалы', level: 1 })).toBeVisible()
  const rows = page.locator('.spec-table tbody tr')
  await expect(rows).toHaveCount(7)
  await expect(rows.first()).toContainText('кг/м³')

  await page.getByRole('link', { name: /Орех/ }).first().click()
  await expect(page).toHaveURL(/\/materials\/WOOD-WALNUT/)
  await expect(page.getByRole('heading', { name: 'Орех', level: 1 })).toBeVisible()
  await expect(page.getByText('640 кг/м³')).toBeVisible()
  await expect(page.getByText('20–60 мм')).toBeVisible()
  // Кнопка ведёт в калькулятор с выбранным материалом.
  await expect(page.getByRole('link', { name: /Рассчитать из/ })).toBeVisible()
})

test('калькулятор на витрине считает и показывает 3D', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(String(e?.message ?? e)))

  await page.goto('/calculator')
  await page.getByRole('heading', { name: 'Конструктор лестницы' }).waitFor({ timeout: 20_000 })

  // Тип марша: спирали быть не должно (S-152).
  const flightOptions = await page
    .getByLabel('Тип лестницы')
    .locator('option')
    .allTextContents()
  expect(flightOptions).toEqual(['Прямой марш', 'L-образная (с площадкой)', 'П-образная (с площадкой)'])

  await page.getByLabel('Ширина марша (мм)').fill('900')
  await page.getByLabel('Высота (мм)').fill('2700')
  await page.getByLabel('Толщина ступени (мм)').fill('6')
  await page.getByLabel('Просвет (мм)').fill('2000')
  await page.getByLabel('Высота перил (мм)').fill('900')
  await page.getByLabel('Ширина помещения (мм)').fill('3000')
  await page.getByLabel('Длина помещения (мм)').fill('4200')
  await page.getByRole('button', { name: 'Рассчитать' }).click()

  await expect(page.getByText('Предварительная цена')).toBeVisible({ timeout: 20_000 })
  await expect(page.locator('.viewer__stage canvas')).toBeVisible({ timeout: 20_000 })
  expect(errors, `ошибки страницы: ${errors.join('; ')}`).toHaveLength(0)
})

test('SEO: sitemap собирает материалы из каталога, robots закрывает /api', async ({ request }) => {
  const sitemap = await request.get('/sitemap.xml')
  expect(sitemap.status()).toBe(200)
  const xml = await sitemap.text()
  expect(xml).toContain('/materials')
  expect(xml).toContain('/materials/WOOD-WALNUT')
  expect(xml).toContain('/calculator')

  const robots = await request.get('/robots.txt')
  expect(robots.status()).toBe(200)
  expect(await robots.text()).toContain('Disallow: /api/')
})

test('страница 404 без каталога отдаёт понятный текст', async ({ page }) => {
  await page.goto('/materials/WOOD-NOPE')
  await expect(page.getByText('Материал не найден')).toBeVisible()
})

// ---- Этап 4: оплата инженерных услуг ----

test('услуги: прайс с сервера виден гостю, оплата — после входа', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(String(e?.message ?? e)))

  await page.goto('/services')
  // Каталог услуг приходит с /api/v1/public/payment-tiers: две карточки с
  // серверными ценами — их видит и гость.
  const cards = page.locator('[data-tier]')
  await expect(cards).toHaveCount(2, { timeout: 15_000 })
  await expect(cards.filter({ hasText: 'Выезд инженера и замер' })).toBeVisible()
  await expect(cards.filter({ hasText: 'Проект и рабочая документация' })).toBeVisible()
  await expect(page.getByText('900').first()).toBeVisible()
  await expect(page.getByText('1 800').first()).toBeVisible()

  // Гость платит не может — вместо кнопки оплаты вход.
  await expect(page.getByRole('button', { name: 'Войти и оплатить', exact: true }).first()).toBeVisible()
  await expect(page.getByRole('button', { name: 'Оплатить', exact: true })).toHaveCount(0)
  expect(errors, `ошибки страницы: ${errors.join('; ')}`).toHaveLength(0)
})

test('услуги: вход раскрывает оплату, checkout отдаёт URL страницы PSP', async ({ page }) => {
  await page.goto('/services')
  await expect(page.getByRole('button', { name: 'Войти и оплатить', exact: true }).first()).toBeVisible({
    timeout: 15_000,
  })

  // Сначала выбираем услугу — только после этого раскрывается форма входа.
  await page.getByRole('button', { name: 'Войти и оплатить', exact: true }).first().click()
  await expect(page.getByTestId('services-auth')).toBeVisible()
  await page.getByLabel('Email').fill(process.env.STORE_E2E_EMAIL ?? 'user@user.ru')
  await page.getByLabel('Пароль').fill(process.env.STORE_E2E_PASSWORD ?? 'user1234')
  await page.getByRole('button', { name: 'Войти', exact: true }).click()

  const pay = page.getByRole('button', { name: 'Оплатить', exact: true }).first()
  await expect(pay).toBeVisible({ timeout: 15_000 })
  const [request] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/v1/public/services/checkout')),
    pay.click(),
  ])
  expect(request.status()).toBe(201)
  const body = (await request.json()) as { checkout_url?: string; amount_minor?: number }
  expect(body.checkout_url).toBeTruthy()
  expect(body.amount_minor).toBeGreaterThan(0)
})
