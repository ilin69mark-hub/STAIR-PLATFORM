// E2E: критический пользовательский workflow (MVP-09 + TEST-0022):
// Создать проект → Настроить → Рассчитать → Панели результата → Экспорт.

import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'
import { expectResultPanel } from '../helpers/resultPanel'

function uniqueName(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1e4)}`
}

test('полный критический workflow: регистрация → создание → расчёт → экспорт', async ({ page }) => {
  const name = uniqueName('E2E')

  // Регистрация нового пользователя (auth обязателен для /api/v1).
  await register(page, uniqueEmail())

  // Создание проекта.
  await page.locator('#project-name').fill(name)
  await page.getByRole('button', { name: 'Создать' }).click()

  // Открылась страница проекта.
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible()

  // Расчёт с дефолтной конфигурацией.
  await page.getByRole('button', { name: 'Рассчитать' }).click()
  await expect(page.getByText('Расчёт сохранён')).toBeVisible()

  // Панели результата конвейера (спрятаны за result-tabs, S-100):
  // «Марш (Solver)» виден по умолчанию, остальные — после клика по табу.
  await expect(page.getByText('Марш (Solver)')).toBeVisible()
  await expectResultPanel(page, 'Геометрия', 'Геометрия')
  await expectResultPanel(page, 'Производство', 'Производство')
  await expectResultPanel(page, 'Стоимость', 'Стоимость (RUB)')
  await expect(page.getByText('Итоговая цена')).toBeVisible()

  // Экспорт BOM (CSV) — кнопка в панели «Производство» (за табом).
  await expectResultPanel(page, 'Производство', 'Производство')
  const bomDownload = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Экспорт BOM (CSV)' }).click()
  const bomFile = await bomDownload
  expect(bomFile.suggestedFilename()).toContain('bom.csv')

  // Экспорт JSON-снапшота.
  const jsonDownload = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Экспорт JSON' }).click()
  const jsonFile = await jsonDownload
  expect(jsonFile.suggestedFilename()).toContain('project-export.json')

  // Возврат к списку: проект сохранился.
  await page.getByRole('button', { name: '← Проекты' }).click()
  await expect(page.getByText(name, { exact: true })).toBeVisible()
})
