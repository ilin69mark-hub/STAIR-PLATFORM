// E2E: П-образный марш (A1b): выбор типа марша, параметры площадки,
// расчёт и панель U-результата.

import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'
import { expectResultPanel } from '../helpers/resultPanel'

function uniqueName(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1e4)}`
}

test('П-образный марш: настройка → расчёт → панель U-результата', async ({ page }) => {
  const name = uniqueName('E2E-U')

  await register(page, uniqueEmail())

  // Создание проекта.
  await page.locator('#project-name').fill(name)
  await page.getByRole('button', { name: 'Создать' }).click()
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible()

  // Выбор П-образного марша: появляются поля площадки.
  await page.locator('#cfg-flight').selectOption('u_shape')
  await expect(page.locator('#cfg-landingWidthMM')).toBeVisible()
  await expect(page.locator('#cfg-lowerStepCountMM')).toBeVisible()
  await page.locator('#cfg-landingWidthMM').fill('1000')
  await page.locator('#cfg-lowerStepCountMM').fill('6')

  // Расчёт: показывается панель П-образного марша (Solver).
  await page.getByRole('button', { name: 'Рассчитать' }).click()
  await expect(page.getByText('Расчёт сохранён')).toBeVisible()
  await expect(page.getByText('П-образный марш (Solver)')).toBeVisible()
  await expect(page.getByText('6 / 9')).toBeVisible()

  // Остальные панели конвейера также присутствуют (за result-tabs).
  await expectResultPanel(page, 'Геометрия', 'Геометрия')
  await expectResultPanel(page, 'Производство', 'Производство')
  await expectResultPanel(page, 'Стоимость', 'Стоимость (RUB)')
})
