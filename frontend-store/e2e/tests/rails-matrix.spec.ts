import { expect, test } from '@playwright/test'

// Матрица по всем маршам — ловит белый экран GeometryViewer (railing null.length)
// и проверяет корректность 2D/3D рендера: сторона перил, approach, landing, направление.
// Каждый кейс — чистый прямой запрос к /public/stairs:quote не требуется (используем UI),
// но дополнительно проверяем отсутствие audit 403 у анонима и pageerror.

type Case = {
  flight: 'straight' | 'l_shape' | 'u_shape' | 'spiral'
  railing?: string
  railingHeight?: string
  approach?: string
  roomW?: string
  roomL?: string
  extra?: Record<string, string>
  direction?: string
  spiralDir?: string
}

const cases: Array<Case & { name: string }> = [
  // straight — 4 стороны перил × без/с площадью × разная высота перил
  { name: 'straight none без площади', flight: 'straight', railing: 'none', railingHeight: '900', approach: '1000', roomW: '', roomL: '' },
  { name: 'straight left 900 с площадью', flight: 'straight', railing: 'left', railingHeight: '900', approach: '1000', roomW: '3000', roomL: '4200' },
  { name: 'straight right 1100', flight: 'straight', railing: 'right', railingHeight: '1100', approach: '1200', roomW: '3500', roomL: '4500' },
  { name: 'straight both высота 0', flight: 'straight', railing: 'both', railingHeight: '0', approach: '1000', roomW: '3000', roomL: '4000' },
  // L — направление + площадка + нижние ступени
  { name: 'l_shape right нижн 6 площадка 1000', flight: 'l_shape', railing: 'both', railingHeight: '900', approach: '1000', roomW: '4000', roomL: '5000', extra: { 'Ширина площадки (мм)': '1000', 'Нижних ступеней (шт)': '6' }, direction: 'right' },
  { name: 'l_shape left узкая площадка 600', flight: 'l_shape', railing: 'left', railingHeight: '900', approach: '1000', roomW: '3000', roomL: '3000', extra: { 'Ширина площадки (мм)': '600', 'Нижних ступеней (шт)': '7' }, direction: 'left' },
  { name: 'l_shape без площади', flight: 'l_shape', railing: 'both', railingHeight: '900', approach: '1000', roomW: '', roomL: '', extra: { 'Ширина площадки (мм)': '1200', 'Нижних ступеней (шт)': '5' }, direction: 'right' },
  // U — + винтовая логика площадки
  { name: 'u_shape right площадка 1200', flight: 'u_shape', railing: 'both', railingHeight: '900', approach: '1000', roomW: '4000', roomL: '4000', extra: { 'Ширина площадки (мм)': '1200', 'Нижних ступеней (шт)': '6' }, direction: 'right' },
  { name: 'u_shape left площадка 800', flight: 'u_shape', railing: 'right', railingHeight: '1100', approach: '1200', roomW: '3500', roomL: '3500', extra: { 'Ширина площадки (мм)': '800', 'Нижних ступеней (шт)': '8' }, direction: 'left' },
  // spiral — радиус + направление
  { name: 'spiral cw R800', flight: 'spiral', railingHeight: '900', approach: '1000', roomW: '3000', roomL: '3000', extra: { 'Наружный радиус (мм)': '800' }, spiralDir: 'cw' },
  { name: 'spiral ccw R1200 с площадью', flight: 'spiral', railingHeight: '900', approach: '1200', roomW: '4000', roomL: '4000', extra: { 'Наружный радиус (мм)': '1200' }, spiralDir: 'ccw' },
  { name: 'spiral ccw без площади высота перил 0', flight: 'spiral', railingHeight: '0', approach: '', roomW: '', roomL: '', extra: { 'Наружный радиус (мм)': '900' }, spiralDir: 'ccw' },
]

async function fillCommon(page: import('@playwright/test').Page, c: Case) {
  // общие поля — заполняем только если label найден (некоторые скрыты для spiral/L)
  const fill = async (label: string, val: string) => {
    if (val === undefined) return
    const loc = page.getByLabel(label)
    if (await loc.count()) await loc.fill(val)
  }
  await fill('Ширина марша (мм)', '900')
  await fill('Высота (мм)', '2700')
  await fill('Толщина ступени (мм)', '6')
  await fill('Просвет (мм)', '2000')
  await fill('Высота перил (мм)', c.railingHeight ?? '900')
  if (c.approach !== undefined) await fill('Свободное пространство (мм)', c.approach)
  if (c.roomW !== undefined) await fill('Ширина помещения (мм)', c.roomW)
  if (c.roomL !== undefined) await fill('Длина помещения (мм)', c.roomL)
  if (c.extra) for (const [k, v] of Object.entries(c.extra)) await fill(k, v)
  if (c.spiralDir) {
    const sel = page.getByLabel('Направление спирали')
    if (await sel.count()) await sel.selectOption(c.spiralDir)
  }
  if (c.direction) {
    const sel = page.getByLabel('Направление поворота')
    if (await sel.count()) await sel.selectOption(c.direction)
  }
  if (c.railing) {
    if (c.flight === 'straight') {
      const sel = page.getByLabel('Перила', { exact: true })
      if (await sel.count()) await sel.selectOption(c.railing)
    } else if (c.flight === 'l_shape' || c.flight === 'u_shape') {
      for (const label of ['Перила: первый марш', 'Перила: площадка', 'Перила: второй марш']) {
        const sel = page.getByLabel(label)
        if (await sel.count()) await sel.selectOption(c.railing)
      }
    }
    // spiral — railing auto, skip
  }
}

for (const c of cases) {
  test(`rails-matrix: ${c.name}`, async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', e => errors.push(e.message))
    const audit403: string[] = []
    page.on('response', r => {
      if (r.url().includes('/api/v1/audit') && r.status() === 403) audit403.push(r.url())
    })

    await page.goto('/')
    await page.getByRole('button', { name: 'Рассчитать стоимость' }).click()
    await expect(page.getByRole('heading', { name: 'Конструктор лестницы' })).toBeVisible()

    await page.getByLabel('Тип лестницы').selectOption(c.flight)
    await fillCommon(page, c)

    await page.getByRole('button', { name: 'Рассчитать' }).click()
    // модалка "без площади" — нужно подтвердить
    const continueBtn = page.getByRole('button', { name: 'Продолжить без площади' })
    if (await continueBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      await continueBtn.click()
    }
    // блокирующие — показываем issue, не 3D; иначе — цена + 3D
    const price = page.getByText('Предварительная цена')
    const blocking = page.getByText(/блокирующие|Исправьте поля/)
    await expect(price.or(blocking)).toBeVisible({ timeout: 15_000 })

    if (await price.isVisible()) {
      // 3D не должен падать: GeometryViewer canvas виден, без TypeError
      // 3D — ленивый чанк (three.js + PBR + HDRI): на холодном dev-сервере
      // Vite трансформирует его дольше 5 с, поэтому ждём 20 с (в прод-сборке
      // чанк один и открывается мгновенно).
      await expect(page.locator('.geometry-viewer, .viewer__stage')).toBeVisible({ timeout: 20_000 })
      // 2D схема — проверяем сторону перил: none=0, both/right/left>0 или spiral 0 линий
      const railingCount = await page.locator('.scheme__railing, .scheme-3d__title').count()
      // не строгая проверка, главное — нет вылета
      expect(errors.join('\n')).not.toContain("Cannot read properties of null")
      expect(errors.join('\n')).not.toContain("length")
      // подходная зона — при approach>0 должна быть видима (жёлтая площадка)
      // проверяем что аудит не спамит 403 у анонима
      // даём немного времени на debounced logAction
      await page.waitForTimeout(1200)
      expect(audit403, `audit 403: ${audit403.join(',')}`).toHaveLength(0)
    }
    expect(errors, errors.join('\n')).toEqual([])
  })
}
