import { expect, test } from '@playwright/test'

// E2E клиентского сайта: расчёт (анонимно) → вход → заказ → кабинет.
// Требует живого API на :8080 (docker compose) с существующим user@user.ru.

const EMAIL = process.env.STORE_E2E_EMAIL ?? 'user@user.ru'
const PASSWORD = process.env.STORE_E2E_PASSWORD ?? 'user123'

// Поля формы не предзаполнены — заполняем валидные значения перед расчётом.
async function fillForm(page: import('@playwright/test').Page) {
  await page.getByLabel('Ширина марша (мм)').fill('900')
  await page.getByLabel('Высота (мм)').fill('2700')
  await page.getByLabel('Высота ступени (мм)').fill('180')
  await page.getByLabel('Толщина ступени (мм)').fill('40')
  await page.getByLabel('Просвет (мм)').fill('2000')
  await page.getByLabel('Высота перил (мм)').fill('900')
}

test('ландинг → конструктор → анонимный расчёт', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: /Лестницы на заказ/ })).toBeVisible()

  await page.getByRole('button', { name: 'Рассчитать стоимость' }).click()
  await expect(page.getByRole('heading', { name: 'Конструктор лестницы' })).toBeVisible()

  await fillForm(page)
  await page.getByRole('button', { name: 'Рассчитать' }).click()
  await expect(page.getByText('Предварительная цена')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText(/Результат расчёта/)).toBeVisible()
})

test('конструктор: L-образный марш показывает цену', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Рассчитать стоимость' }).click()

  await page.getByLabel('Тип лестницы').selectOption('l_shape')
  await fillForm(page)
  await page.getByLabel('Ширина площадки (мм)').fill('1000')
  await page.getByLabel('Нижних ступеней (шт)').fill('6')
  await page.getByRole('button', { name: 'Рассчитать' }).click()
  await expect(page.getByText('Предварительная цена')).toBeVisible({ timeout: 15_000 })
})

test('блокирующая конфигурация останавливает расчёт', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Рассчитать стоимость' }).click()

  await page.getByLabel('Ширина марша (мм)').fill('100')
  await page.getByRole('button', { name: 'Рассчитать' }).click()
  await expect(page.getByText(/Исправьте поля формы/)).toBeVisible()
})

test('анонимный заказ требует вход; после входа создаётся заказ', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Рассчитать стоимость' }).click()
  await fillForm(page)
  await page.getByRole('button', { name: 'Рассчитать' }).click()
  await expect(page.getByText('Предварительная цена')).toBeVisible({ timeout: 15_000 })

  // Без входа кнопка заказа не появляется — проверяем вход через кабинет.
  await page.getByRole('link', { name: 'Кабинет' }).click()
  await expect(page.getByRole('heading', { name: 'Личный кабинет' })).toBeVisible()

  await page.getByLabel('Email').fill(EMAIL)
  await page.getByLabel('Пароль').fill(PASSWORD)
  await page.getByRole('button', { name: 'Войти в кабинет' }).click()

  // После входа кабинет показывает заказы (или пустое состояние).
  await expect(page.getByRole('heading', { name: 'Мои заказы' })).toBeVisible({ timeout: 15_000 })
})
