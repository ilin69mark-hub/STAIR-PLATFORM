// E2E: Спиральный марш (A1c, EDR-0007): выбор типа марша, наружный радиус,
// расчёт и панель спирального результата.

import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'
import { expectResultPanel } from '../helpers/resultPanel'

function uniqueName(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1e4)}`
}

test('Спиральный марш: настройка → расчёт → панель спирального результата', async ({ page }) => {
  // Тип марша скрыт в форме, пока SPIRAL_ENABLED = false (S-152, EDR-0007).
  // Код движка, каталог и тест не удалены — вернутся вместе со спиралью.
  test.skip(true, 'Спиральный марш отключён (S-152): нет опции в #cfg-flight')

  const name = uniqueName('E2E-SP')

  await register(page, uniqueEmail())

  // Создание проекта.
  await page.locator('#project-name').fill(name)
  await page.getByRole('button', { name: 'Создать' }).click()
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible()

  // Выбор спирального марша: появляется поле наружного радиуса.
  await page.locator('#cfg-flight').selectOption('spiral')
  await expect(page.locator('#cfg-outerRadiusMM')).toBeVisible()
  await page.locator('#cfg-widthMM').fill('500')
  await page.locator('#cfg-outerRadiusMM').fill('800')

  // Расчёт: показывается панель спирального марша (Solver).
  await page.getByRole('button', { name: 'Рассчитать' }).click()
  await expect(page.getByText('Расчёт сохранён')).toBeVisible()
  await expect(page.getByText('Спиральный марш (Solver)')).toBeVisible()
  await expect(page.getByText('Радиус колонны r')).toBeVisible()
  await expect(page.getByText('Наружный радиус R', { exact: true })).toBeVisible()

  // Остальные панели конвейера также присутствуют (за result-tabs).
  await expectResultPanel(page, 'Геометрия', 'Геометрия')
  await expectResultPanel(page, 'Производство', 'Производство')
  await expectResultPanel(page, 'Стоимость', 'Стоимость (RUB)')
})
