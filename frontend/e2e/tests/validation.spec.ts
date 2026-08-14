// E2E: валидация и blocking-результат конвейера (TEST-0022, EDR-0003).

import { expect, test } from '@playwright/test'

async function createProject(page: import('@playwright/test').Page, name: string) {
  await page.goto('/')
  await page.locator('#project-name').fill(name)
  await page.getByRole('button', { name: 'Создать' }).click()
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible()
}

test('пустое обязательное поле блокирует кнопку расчёта', async ({ page }) => {
  const name = `E2E-val-${Date.now()}`
  await createProject(page, name)

  await page.locator('#cfg-widthMM').fill('')
  await expect(page.getByText('Укажите значение')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Рассчитать' })).toBeDisabled()
  await expect(page.getByText(/Исправьте нечисловые или пустые поля/)).toBeVisible()
})

test('blocking-вход останавливает конвейер без панелей результата', async ({ page }) => {
  const name = `E2E-block-${Date.now()}`
  await createProject(page, name)

  // Шаг ступени вне допустимого диапазона (10 мм) — блокирует конвейер.
  await page.locator('#cfg-stepHeightMM').fill('10')
  await page.getByRole('button', { name: 'Рассчитать' }).click()

  await expect(
    page.getByText('Конвейер остановлен: обнаружены блокирующие нарушения.'),
  ).toBeVisible()
  await expect(page.getByText('Марш (Solver)')).not.toBeVisible()
  await expect(page.getByText('Стоимость (RUB)')).not.toBeVisible()
})
