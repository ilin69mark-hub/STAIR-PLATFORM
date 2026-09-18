// E2E (store): S-P6 — невписываемый прямой марш. RU-подсказки room_fit
// (элемент «Помещение»), галерея вариантов C (спиральная) и превью по клику
// (стабильные русские тексты). Требует живого API на :8080 (docker compose).

import { expect, test } from '@playwright/test'

async function openConstructor(page: import('@playwright/test').Page) {
  await page.goto('/')
  await page.getByRole('button', { name: 'Рассчитать стоимость' }).click()
  await expect(page.getByRole('heading', { name: 'Конструктор лестницы' })).toBeVisible()
}

test('невписываемый прямой марш: RU-таблица, вариант C: спиральная и превью по клику', async ({ page }) => {
  await openConstructor(page)

  // Прямая 1100×2700, помещение 4500×900 (ширина×длина) — марш не вписывается.
  // Шаг подбирается автоматически (h=168, комфорт 630), просвет 2000 валиден.
  // NB: 900×2600 блокируется раньше по проступи (259 < 260) — галерея не появится.
  await page.getByLabel('Ширина марша (мм)').fill('1100')
  await page.getByLabel('Высота (мм)').fill('2700')
  await page.getByLabel('Толщина ступени (мм)').fill('6')
  await page.getByLabel('Просвет (мм)').fill('2000')
  await page.getByLabel('Высота перил (мм)').fill('900')
  await page.getByLabel('Ширина помещения (мм)').fill('4500')
  await page.getByLabel('Длина помещения (мм)').fill('900')

  await page.getByRole('button', { name: 'Рассчитать' }).click()

  // Элемент «Помещение» с русским текстом рекомендации.
  await expect(page.getByText('Помещение', { exact: true })).toBeVisible()
  await expect(page.getByText(/Лестница не помещается в помещение 4500×900/)).toBeVisible()

  // Галерея вариантов: title содержит «C:», применяется по клику.
  await expect(page.getByText('Выберите вариант — применится как превью:')).toBeVisible()
  const optionCards = page.locator('.variation-picker__item')
  await expect(optionCards.first()).toBeVisible()
  await expect(page.locator('.variation-picker__title', { hasText: 'C:' }).first()).toBeVisible()

  // Клик по карточке применяет вариант как превью: активная помечается
  // «Выбран» и становится крупной (main). В store-лендинге нет отдельного
  // заголовка «Превью варианта» (он только в админке ProjectDetail).
  // Клик по карточке применяет вариант как превью: активная помечается
  // «Выбран» и становится крупной (main). Кликаем именно спиральную «C:».
  await page.locator('.variation-picker__item', { hasText: 'C: спиральная' }).first().click()
  await expect(page.getByText('Выбран')).toBeVisible()
  await expect(page.locator('.variation-gallery__main')).toContainText('Спираль')
})
