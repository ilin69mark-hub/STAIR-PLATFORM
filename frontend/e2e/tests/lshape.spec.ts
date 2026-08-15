// E2E: L-образный марш (A1a): выбор типа марша, параметры площадки,
// расчёт и панель L-результата.

import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'

function uniqueName(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1e4)}`
}

test('L-образный марш: настройка → расчёт → панель L-результата', async ({ page }) => {
  const name = uniqueName('E2E-L')

  await register(page, uniqueEmail())

  // Создание проекта.
  await page.locator('#project-name').fill(name)
  await page.getByRole('button', { name: 'Создать' }).click()
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible()

  // Выбор L-образного марша: появляются поля порожка.
  await page.locator('#cfg-flight').selectOption('l_shape')
  await expect(page.locator('#cfg-landingWidthMM')).toBeVisible()
  await expect(page.locator('#cfg-lowerStepCountMM')).toBeVisible()
  await page.locator('#cfg-landingWidthMM').fill('1000')
  await page.locator('#cfg-lowerStepCountMM').fill('6')

  // Расчёт: показывается панель L-образного марша (Solver).
  await page.getByRole('button', { name: 'Рассчитать' }).click()
  await expect(page.getByText('Расчёт сохранён')).toBeVisible()
  await expect(page.getByText('L-образный марш (Solver)')).toBeVisible()
  await expect(page.getByText('6 / 9')).toBeVisible()

  // Остальные панели конвейера также присутствуют.
  await expect(page.getByRole('heading', { name: 'Геометрия' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Производство' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Стоимость (RUB)' })).toBeVisible()
})