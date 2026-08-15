// E2E: AI-ассистенты (Phase D, EDR-0036): четыре вкладки запрашивают
// рекомендации ассистентов по текущей конфигурации проекта.

import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'

function uniqueName(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1e4)}`
}

async function createProject(page: import('@playwright/test').Page): Promise<void> {
  await register(page, uniqueEmail())
  await page.locator('#project-name').fill(uniqueName('AI'))
  await page.getByRole('button', { name: 'Создать' }).click()
  await expect(page.getByRole('heading', { name: /^AI-/, exact: false })).toBeVisible()
}

test('AI-ассистент: design по умолчанию и переключение вкладок', async ({ page }) => {
  await createProject(page)

  // Вкладка Design активна по умолчанию, есть приоритет.
  const designTab = page.getByRole('tab', { name: /Проектирование/ })
  await expect(designTab).toBeVisible()
  const priority = page.getByLabel(/Приоритет/)
  await expect(priority).toBeVisible()
  await priority.selectOption('comfort')

  // Запрос ассистента: структурный ответ с рекомендацией и рейтингом.
  await page.getByRole('button', { name: 'Спросить ассистента' }).click()
  await expect(page.getByText(/Рейтинг рекомендации/)).toBeVisible({ timeout: 20000 })

  // Переключение на Инжиниринг — запрос без приоритета.
  await page.getByRole('tab', { name: /Инжиниринг/ }).click()
  await expect(page.getByLabel(/Приоритет/)).toHaveCount(0)
  await page.getByRole('button', { name: 'Спросить ассистента' }).click()
  await expect(page.getByText(/Рейтинг рекомендации/)).toBeVisible({ timeout: 20000 })
})

test('AI-ассистент: производство и цены отвечают структурным ответом', async ({ page }) => {
  await createProject(page)

  await page.getByRole('tab', { name: /Производство/ }).click()
  await page.getByRole('button', { name: 'Спросить ассистента' }).click()
  await expect(page.getByText(/Рейтинг рекомендации/)).toBeVisible({ timeout: 20000 })

  await page.getByRole('tab', { name: /Ценообразование/ }).click()
  await page.getByRole('button', { name: 'Спросить ассистента' }).click()
  await expect(page.getByText(/Рейтинг рекомендации/)).toBeVisible({ timeout: 20000 })
})