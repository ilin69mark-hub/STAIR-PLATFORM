// E2E (admin): S-P6 — неблокирующее нарушение room_fit (невписываемость в
// помещение) в RU-таблице валидации: колонки без «Код», элемент «Помещение»,
// русская рекомендация про готовые варианты. Блок вариантов решения
// присутствует (превью по клику реализует та же панель).

import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'

async function createProject(page: import('@playwright/test').Page, name: string) {
  await page.locator('#project-name').fill(name)
  await page.getByRole('button', { name: 'Создать' }).click()
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible()
}

async function authAndCreate(page: import('@playwright/test').Page, name: string) {
  await register(page, uniqueEmail())
  await createProject(page, name)
}

test('невписываемый прямой марш: RU-таблица без «Код», элемент «Помещение», блок вариантов', async ({ page }) => {
  const name = `E2E-roomfit-${Date.now()}`
  await authAndCreate(page, name)

  await page.locator('#cfg-widthMM').fill('1100')
  await page.locator('#cfg-heightMM').fill('2700')
  // Автошаг (как в store-пройденном конфиге): вручную не ставим, чтобы
  // room_fit был первым неблокирующим нарушением и появилась галерея.
  await page.locator('#cfg-comfortStepMM').fill('630')
  await page.locator('#cfg-clearanceMM').fill('2000')
  await page.locator('#cfg-roomWidthMM').fill('4500')
  await page.locator('#cfg-roomLengthMM').fill('900')

  await page.getByRole('button', { name: 'Рассчитать' }).click()

  // Не блокирует — только предупреждение.
  await expect(page.getByRole('cell', { name: 'Предупреждение' }).first()).toBeVisible({ timeout: 20_000 })

  await page.getByRole('button', { name: 'Рассчитать' }).click()

  // Не блокирует — только предупреждение.
  await expect(page.getByRole('cell', { name: 'Предупреждение' }).first()).toBeVisible({ timeout: 20_000 })

  // RU-заголовки таблицы, колонки «Код» нет.
  for (const h of ['Важность', 'Элемент', 'Сообщение', 'Что поправить', 'Рекомендация']) {
    await expect(page.getByRole('columnheader', { name: h })).toBeVisible()
  }
  await expect(page.getByRole('columnheader', { name: 'Код' })).toHaveCount(0)

  // Элемент «Помещение» и русский текст рекомендации.
  await expect(page.getByRole('cell', { name: 'Помещение' }).first()).toBeVisible()
  await expect(page.getByText(/Лестница не помещается в помещение 4500×900/)).toBeVisible()
  await expect(page.getByText(/Уменьшите габариты лестницы|выберите один из готовых вариантов/)).toBeVisible()

  // Блок вариантов решения — это tablist: RU-вкладки, выбран «Марш».
  await expect(page.getByRole('heading', { name: 'Варианты решения (выберите подходящий)' })).toBeVisible()
  for (const tab of ['Марш', 'Геометрия', 'Производство', 'Стоимость']) {
    await expect(page.getByRole('tab', { name: tab }).first()).toBeVisible()
  }
  await expect(page.getByRole('tab', { name: 'Марш' }).first()).toHaveAttribute('aria-selected', 'true')
})
