import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test, type Page } from '@playwright/test'

// Матрица 2000 e2e (500/марш × straight/l_shape/u_shape/spiral) из
// go run ./scripts/generate-variations.go -n 500.
// Каждый кейс — полный UI-цикл: конструктор → тип марша → поля из кейса →
// расчёт → цена (или блокировка) → 3D-вьювер без pageerror.
// Прогон: 2000×~15с ≈ 8.3ч серийно → шардируется в CI 8×250, workers:4
// (~16 мин/джоб): npx playwright test quote-matrix-generated --shard=k/8.
// Дедупликация бэкенда учитывает хэш тела, поэтому параллельные воркеры
// с разными payload не получают ложный 429 (dedup.go: DeduplicateByKey).

type GenCase = {
  flight: 'straight' | 'l_shape' | 'u_shape' | 'spiral'
  width_mm: number
  height_mm: number
  material: string
  step_thickness_mm: number
  stringer_thickness_mm: number
  clearance_mm: number
  railing_height_mm: number
  railing: string
  approach_space_mm: number
  room_width_mm: number
  room_length_mm: number
  landing_width_mm: number
  landing_depth_mm: number
  lower_step_count: number
  outer_radius_mm: number
  spiral_direction: string
  direction: string
}

const all = JSON.parse(
  readFileSync(join(dirname(fileURLToPath(import.meta.url)), '../../src/test/generated-cases-500.json'), 'utf8'),
) as GenCase[]

const num = (v: number) => String(Math.round(v * 1000) / 1000)

async function fillIfVisible(page: Page, label: string, val: string) {
  const loc = page.getByLabel(label)
  if ((await loc.count()) === 0) return
  if (!(await loc.first().isVisible().catch(() => false))) return
  await loc.first().fill(val)
}

async function selectIfVisible(page: Page, label: string, val: string, exact = false) {
  const loc = page.getByLabel(label, { exact })
  if ((await loc.count()) === 0) return
  if (!(await loc.first().isVisible().catch(() => false))) return
  await loc.first().selectOption(val)
}

async function fillCase(page: Page, c: GenCase) {
  await selectIfVisible(page, 'Тип лестницы', c.flight)
  await selectIfVisible(page, 'Материал', c.material)
  await fillIfVisible(page, 'Ширина марша (мм)', num(c.width_mm))
  await fillIfVisible(page, 'Высота (мм)', num(c.height_mm))
  await fillIfVisible(page, 'Толщина ступени (мм)', num(c.step_thickness_mm))
  await fillIfVisible(page, 'Толщина косоура (мм)', num(c.stringer_thickness_mm))
  await fillIfVisible(page, 'Просвет (мм)', num(c.clearance_mm))
  await fillIfVisible(page, 'Высота перил (мм)', num(c.railing_height_mm))
  await fillIfVisible(page, 'Свободное пространство перед маршем (мм)', num(c.approach_space_mm))
  await fillIfVisible(page, 'Ширина помещения (мм)', c.room_width_mm > 0 ? num(c.room_width_mm) : '')
  await fillIfVisible(page, 'Длина помещения (мм)', c.room_length_mm > 0 ? num(c.room_length_mm) : '')
  if (c.flight === 'straight') {
    await selectIfVisible(page, 'Перила', c.railing, true)
  } else if (c.flight === 'l_shape' || c.flight === 'u_shape') {
    await fillIfVisible(page, 'Ширина площадки (мм)', num(c.landing_width_mm))
    await fillIfVisible(page, 'Глубина площадки (мм)', num(c.landing_depth_mm))
    await fillIfVisible(page, 'Нижних ступеней (шт)', String(c.lower_step_count))
    await selectIfVisible(page, 'Направление поворота', c.direction)
    for (const label of ['Перила: первый марш', 'Перила: площадка', 'Перила: второй марш']) {
      await selectIfVisible(page, label, c.railing)
    }
  } else {
    await fillIfVisible(page, 'Радиус (мм)', num(c.outer_radius_mm))
    await selectIfVisible(page, 'Направление спирали', c.spiral_direction)
  }
}

all.forEach((c, i) => {
  test(`matrix ${c.flight} #${i}`, async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(String(e?.message ?? e)))
    const audit403: string[] = []
    page.on('response', r => {
      if (r.url().includes('/api/v1/audit') && r.status() === 403) audit403.push(r.url())
    })

    // Хэш-роут открывает конструктор напрямую, без лендинга.
    await page.goto('/#constructor')
    await expect(page.getByRole('heading', { name: 'Конструктор лестницы' })).toBeVisible()

    await fillCase(page, c)

    await page.getByRole('button', { name: 'Рассчитать' }).click()
    // Модалка "без площади" — подтвердить расчёт без проверки вписываемости.
    const continueBtn = page.getByRole('button', { name: 'Продолжить без площади' })
    if (await continueBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      await continueBtn.click()
    }
    // Кейсы валидны на бэкенде (SolveChecked без блокировок), но фронт может
    // быть строже (перила 0, узкая площадка) — принимаем цену или блокировку,
    // главное: нет белого экрана и pageerror.
    const price = page.getByText('Предварительная цена')
    const blocking = page.getByText(/блокирующие|Исправьте поля/)
    await expect(price.or(blocking)).toBeVisible({ timeout: 15_000 })

    if (await price.isVisible()) {
      // 3D вьювер ленивый (three.js по требованию): под параллельной
      // нагрузкой (4 воркера) монтирование занимает секунды.
      await expect(page.locator('.geometry-viewer, .viewer__stage')).toBeVisible({ timeout: 15_000 })
    }
    expect(audit403, `audit 403: ${audit403.join(',')}`).toHaveLength(0)
    expect(errors, errors.join('\n')).toEqual([])
  })
})
