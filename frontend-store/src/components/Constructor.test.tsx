import type { ReactNode } from 'react'
import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Constructor } from './Constructor'
import { quoteApi } from '../api/store'
import { renderWithAuth } from '../test/render'
import type { QuoteResult, Variation } from '@shared/types'

// Захват пропсов GeometryViewer: QuoteResult рендерится настоящим, а вьювер
// мокаем (WebGL в jsdom не строится). Проверяем связку «Высота (мм) в
// калькуляторе → QuoteResult → вьювер».
const viewerProps = vi.hoisted(() => ({ current: {} as Record<string, unknown> }))
vi.mock('@shared/viewer/GeometryViewer', () => ({
  GeometryViewer: (p: Record<string, unknown>) => {
    viewerProps.current = p
    return <div data-testid="mock-3d-viewer">{p.overlay as ReactNode}</div>
  },
}))

const mesh3d = {
  Vertices: [
    { X: 0, Y: 0, Z: 0 },
    { X: 900, Y: 0, Z: 0 },
    { X: 0, Y: 0, Z: 2700 },
  ],
  Triangles: [[0, 1, 2]] as Array<[number, number, number]>,
}

// blockedVariations — blocking-ответ с GEO-ANGLE, несущим одну вариацию
// (имитация того, что отдаёт бэкенд через variation.ForAngle).
function blockedVariations(cfg: Record<string, string>): QuoteResult {
  const v: Variation = {
    id: 'Угол 30°',
    title: 'Угол 30°',
    description: 'Сделать угол наклона в норме 30–45°.',
    config: cfg,
    fits: true,
    summary: 'Угол 30,2°, 18 ступ., h 167 мм, b 620 мм',
  }
  return {
    validation: {
      valid: false,
      blocking: true,
      issues: [
        {
          code: 'GEO-ANGLE',
          severity: 'error',
          element: 'angle',
          message: 'угол вне нормы',
          guide: 'Угол наклона 26,7° вне нормы (30–45°). Измените число ступеней.',
          param: 'Число ступеней',
          variations: [v],
        },
      ],
    },
  }
}

// pickMaterial — выбор материала в карточках-превью (role=radio).
const MATERIAL_LABELS: Record<string, RegExp> = {
  'STEEL-S235': /Сталь S235/,
  'STEEL-CORTEN': /Кортэн/,
  'ALUM-5083': /Алюминий/,
  'WOOD-OAK': /^Дуб/,
  'WOOD-WALNUT': /Орех/,
  'WOOD-ASH': /Ясень/,
  'WOOD-SOFT': /Сосна/,
}
function pickMaterial(code: string) {
  fireEvent.click(screen.getByRole('radio', { name: MATERIAL_LABELS[code] }))
}

const okQuote: QuoteResult = {
  validation: { valid: true, blocking: false, issues: [] },
  flight: {
    step_count: 15,
    step_height_mm: 180,
    tread_depth_mm: 270,
    run_mm: 4050,
    stringer_mm: 4867.49,
    angle_deg: 33.69,
    width_mm: 900,
    step_thickness_mm: 40,
    railing_height_mm: 900,
    riser: true,
    stringer_thickness_mm: 50,
  },
  geometry: { solid_count: 1, volume_mm3: 1e7, surface_area_mm2: 2e5, bbox: { min: { x: 0, y: 0, z: 0 }, max: { x: 900, y: 2700, z: 500 } } },
  pricing: { currency: 'RUB', material_rub: 1, machine_rub: 2, labor_rub: 3, overhead_rub: 4, production_cost_rub: 10, margin_rub: 5, discount_rub: 0, pre_tax_rub: 15, tax_rub: 3, final_price_rub: 18, lines: [] },
}

const blockedWithAdvice: QuoteResult = {
  validation: {
    valid: false,
    blocking: true,
    issues: [
      {
        code: 'GEO-ANGLE',
        severity: 'error',
        element: 'angle',
        message: 'угол вне нормы',
        guide: 'Угол наклона 29,7° вне нормы (30–45°). Измените число ступеней.',
        param: 'Число ступеней',
        suggestions: [{ step_count: 18, step_height_mm: 166.7, tread_depth_mm: 266.7, angle_deg: 32 }],
      },
    ],
  },
}

const validValues: Record<string, string> = {
  'Ширина марша (мм)': '900',
  'Высота (мм)': '2700',
  'Толщина ступени (мм)': '6',
  'Просвет (мм)': '2000',
  'Высота перил (мм)': '900',
  'Ширина помещения (мм)': '3000',
  'Длина помещения (мм)': '4200',
}

function fillValid() {
  for (const [label, value] of Object.entries(validValues)) {
    fireEvent.change(screen.getByLabelText(label), { target: { value } })
  }
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('Constructor', () => {
  it('показывает форму конструктора', async () => {
    await renderWithAuth(<Constructor />, null)
    expect(screen.getByText('Конструктор лестницы')).toBeInTheDocument()
    expect(screen.getByLabelText('Тип лестницы')).toBeInTheDocument()
    expect(screen.getByText('Прямой марш')).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /Сталь S235/ })).toHaveAttribute('aria-checked', 'true')
  })

  it('загружается с пустыми полями', async () => {
    await renderWithAuth(<Constructor />, null)
    for (const label of Object.keys(validValues)) {
      expect(screen.getByLabelText(label)).toHaveValue('')
    }
  })

  it('чекбокс подступенка: включён по умолчанию и передаётся в запрос', async () => {
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    const checkbox = screen.getByLabelText('Подступень') as HTMLInputElement
    expect(checkbox.checked).toBe(true)

    fireEvent.click(checkbox)
    expect((screen.getByLabelText('Подступень') as HTMLInputElement).checked).toBe(false)

    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => {
      expect(spy).toHaveBeenCalledWith(expect.objectContaining({ riser: false }))
    })
  })

  it('показывает подсказки для ступени и перил', async () => {
    await renderWithAuth(<Constructor />, null)
    expect(screen.getByText('Мин 3 / макс 8 мм')).toBeInTheDocument()
    expect(screen.getByText('Рекомендуем 900–1100 мм')).toBeInTheDocument()
  })

  it('подсказки ширины и высоты зависят от материала', async () => {
    await renderWithAuth(<Constructor />, null)
    // Сталь по умолчанию: макс. высота 6000 мм.
    expect(screen.getByText('Макс 6000 мм')).toBeInTheDocument()
    expect(screen.getByText('Макс 3000 мм')).toBeInTheDocument()

    pickMaterial('WOOD-OAK')
    // Дуб: макс. высота 4550 мм, толщина ступени 20–60 мм.
    expect(screen.getByText('Макс 4550 мм')).toBeInTheDocument()
    expect(screen.getByText('Мин 20 / макс 60 мм')).toBeInTheDocument()

    pickMaterial('ALUM-5083')
    expect(screen.getByText('Мин 2 / макс 60 мм')).toBeInTheDocument()
  })

  it('3D: перетаскивание ступени меняет высоту и сразу пересчитывает', async () => {
    const okQuote3D: QuoteResult = { ...okQuote, mesh: mesh3d }
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote3D)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(screen.getByTestId('mock-3d-viewer')).toBeInTheDocument())

    act(() => {
      ;(viewerProps.current.onDragHeight as (h: number) => void)(3100)
    })
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ height_mm: 3100 })),
    )
    expect(screen.getByLabelText('Высота (мм)')).toHaveValue('3100')
  })

  it('3D: горизонтальный drag меняет шаг комфорта и пересчитывает', async () => {
    const okQuote3D: QuoteResult = { ...okQuote, mesh: mesh3d }
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote3D)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(screen.getByTestId('mock-3d-viewer')).toBeInTheDocument())

    act(() => {
      ;(viewerProps.current.onDragComfortStep as (v: number) => void)(640)
    })
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ comfort_step_mm: 640 })),
    )
  })

  it('3D: у прямого марша кнопки «Развернуть» нет (у API нет направления)', async () => {
    const okQuote3D: QuoteResult = { ...okQuote, mesh: mesh3d }
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote3D)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(screen.getByTestId('mock-3d-viewer')).toBeInTheDocument())
    act(() => {
      ;(viewerProps.current.onSelectPart as (p: unknown) => void)({ solid: 2, role: 'tread' })
    })
    expect(screen.getByText('Ступень 3')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Развернуть' })).not.toBeInTheDocument()
  })

  it('пресет «Скандинавский дуб» подставляет материал, толщину и перила', async () => {
    await renderWithAuth(<Constructor />, null)
    fireEvent.click(screen.getByRole('button', { name: /Скандинавский дуб/ }))
    expect(screen.getByRole('radio', { name: /^Дуб/ })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByLabelText('Толщина ступени (мм)')).toHaveValue('40')
    expect(screen.getByLabelText('Перила', { selector: '#cfg-railing' })).toHaveValue('both')
  })

  it('смена материала на дерево подтягивает толщину ступени в допуск (иначе 422)', async () => {
    await renderWithAuth(<Constructor />, null)
    const thickness = screen.getByLabelText('Толщина ступени (мм)')
    fireEvent.change(thickness, { target: { value: '6' } })
    expect(thickness).toHaveValue('6')
    pickMaterial('WOOD-OAK')
    expect(screen.getByLabelText('Толщина ступени (мм)')).toHaveValue('40')
    pickMaterial('STEEL-S235')
    expect(screen.getByLabelText('Толщина ступени (мм)')).toHaveValue('6')
  })

  it('валидация учитывает пределы материала', async () => {
    await renderWithAuth(<Constructor />, null)
    pickMaterial('ALUM-5083')
    fireEvent.change(screen.getByLabelText('Высота (мм)'), { target: { value: '5000' } })
    expect(await screen.findByText('Не более 4550')).toBeInTheDocument()
  })

  it('показывает знак справки с тултипом у просвета', async () => {
    await renderWithAuth(<Constructor />, null)
    const help = screen.getAllByRole('tooltip').find((el) =>
      el.textContent?.includes('Просвет — вертикальное расстояние'),
    )
    expect(help).toBeDefined()
  })

  it('прямой марш: один выбор перил с тултипом-подсказкой', async () => {
    await renderWithAuth(<Constructor />, null)
    const railing = screen.getByRole('combobox', { name: 'Перила' }) as HTMLSelectElement
    expect(railing).toBeVisible()
    expect(railing.value).toBe('both')
    // Тултип объясняет, как определять стороны перил.
    const tip = screen.getAllByRole('tooltip').find((el) =>
      el.textContent?.includes('Слева от вас — левые перила'),
    )
    expect(tip).toBeDefined()
    // Направления — только для маршей с площадкой/спирали.
    expect(screen.queryByRole('combobox', { name: 'Направление поворота' })).toBeNull()
    expect(screen.queryByRole('combobox', { name: 'Направление спирали' })).toBeNull()
  })

  it('L-образная: перила по сегментам и направление поворота', async () => {
    await renderWithAuth(<Constructor />, null)
    fireEvent.change(screen.getByLabelText('Тип лестницы'), { target: { value: 'l_shape' } })
    // Прямой «Перила» скрывается, появляются сегменты.
    expect(screen.queryByRole('combobox', { name: 'Перила' })).toBeNull()
    expect(
      (screen.getByRole('combobox', { name: 'Перила: первый марш' }) as HTMLSelectElement).value,
    ).toBe('both')
    expect(screen.getByRole('combobox', { name: 'Перила: площадка' })).toBeVisible()
    expect(screen.getByRole('combobox', { name: 'Перила: второй марш' })).toBeVisible()
    const dir = screen.getByRole('combobox', {
      name: 'Направление поворота',
    }) as HTMLSelectElement
    expect(dir).toBeVisible()
    expect(dir.value).toBe('left')
    expect(screen.queryByRole('combobox', { name: 'Направление спирали' })).toBeNull()

    fireEvent.change(dir, { target: { value: 'right' } })
    expect(
      (screen.getByRole('combobox', { name: 'Направление поворота' }) as HTMLSelectElement)
        .value,
    ).toBe('right')
  })

  it('спираль: направление закрутки и авто-перила от него', async () => {
    await renderWithAuth(<Constructor />, null)
    fireEvent.change(screen.getByLabelText('Тип лестницы'), { target: { value: 'spiral' } })
    const dir = screen.getByRole('combobox', { name: 'Направление спирали' }) as HTMLSelectElement
    expect(dir).toBeVisible()
    // Перила автоматически: против часовой → слева (CONF-SPIRAL-RAILING).
    expect((screen.getByRole('textbox', { name: 'Перила' }) as HTMLInputElement).value).toBe('Слева')
    fireEvent.change(dir, { target: { value: 'cw' } })
    expect((screen.getByRole('textbox', { name: 'Перила' }) as HTMLInputElement).value).toBe('Справа')
  })

  it('не вызывает расчёт при ошибках валидации', async () => {
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    const width = screen.getByLabelText('Ширина марша (мм)')
    fireEvent.change(width, { target: { value: '100' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText('Исправьте поля формы перед расчётом')).toBeInTheDocument()
    expect(spy).not.toHaveBeenCalled()
  })

  it('выполняет расчёт и показывает результат с ценой', async () => {
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
    expect(screen.getByText('Предварительная цена')).toBeInTheDocument()
    await waitFor(() =>
      expect(quoteApi.calculate).toHaveBeenCalledWith(
        expect.objectContaining({ width_mm: 900, height_mm: 2700, flight: 'straight', material: 'STEEL-S235' }),
      ),
    )
  })

  it('высота поля «Высота (мм)» доходит до вьювера и пересчёт меняет её', async () => {
    const okQuote3D: QuoteResult = {
      ...okQuote,
      flight: { ...okQuote.flight!, room_width_mm: 3000, room_length_mm: 4200 },
      room_mesh: mesh3d,
      mesh: mesh3d,
    }
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote3D)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Результат расчёта/)
    await waitFor(() => {
      expect(viewerProps.current).toMatchObject({
        flight: 'straight',
        heightMM: 2700,
        roomWidth: 3000,
        roomLength: 4200,
      })
    })

    // Правка высоты и повторный расчёт — вьювер получает новое значение.
    fireEvent.change(screen.getByLabelText('Высота (мм)'), { target: { value: '2750' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(viewerProps.current.heightMM).toBe(2750))
    expect(viewerProps.current.flight).toBe('straight')
  })

  it('выбранный материал попадает в запрос расчёта', async () => {
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    pickMaterial('WOOD-OAK')
    // Толщина ступени 6 мм допустима для стали, для дуба — нет (min 20).
    fireEvent.change(screen.getByLabelText('Толщина ступени (мм)'), { target: { value: '30' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
    await waitFor(() =>
      expect(quoteApi.calculate).toHaveBeenCalledWith(
        expect.objectContaining({ material: 'WOOD-OAK' }),
      ),
    )
    expect(await screen.findByText('Материал: Дуб')).toBeInTheDocument()
  })

  it('L-образный тип показывает поля площадки', async () => {
    await renderWithAuth(<Constructor />, null)
    const flight = screen.getByLabelText('Тип лестницы')
    fireEvent.change(flight, { target: { value: 'l_shape' } })
    expect(screen.getByLabelText('Ширина площадки (мм)')).toBeInTheDocument()
    expect(screen.getByLabelText('Нижних ступеней (шт)')).toBeInTheDocument()
  })

  it('прямой марш скрывает поля площадки и радиус', async () => {
    await renderWithAuth(<Constructor />, null)
    expect(screen.queryByLabelText('Ширина площадки (мм)')).not.toBeVisible()
    expect(screen.queryByLabelText('Нижних ступеней (шт)')).not.toBeVisible()
    expect(screen.queryByLabelText('Радиус (мм)')).not.toBeVisible()
  })

  it('высота ступени и шаг комфорта скрыты: рассчитываются автоматически', async () => {
    await renderWithAuth(<Constructor />, null)
    expect(screen.queryByLabelText('Высота ступени (мм)')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Шаг комфорта (мм)')).not.toBeInTheDocument()

    const flight = screen.getByLabelText('Тип лестницы')
    fireEvent.change(flight, { target: { value: 'l_shape' } })
    expect(screen.queryByLabelText('Шаг комфорта (мм)')).not.toBeInTheDocument()

    fireEvent.change(flight, { target: { value: 'spiral' } })
    expect(screen.queryByLabelText('Шаг комфорта (мм)')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Радиус (мм)')).toBeVisible()
  })

  it('подсвечивает пустые обязательные поля после ввода', async () => {
    await renderWithAuth(<Constructor />, null)
    // Ошибки показываются после первого изменения поля (touched-подход):
    // сперва вводим значение ширины, затем очищаем — форма уже touched.
    const width = screen.getByLabelText('Ширина марша (мм)')
    fireEvent.change(width, { target: { value: '900' } })
    fireEvent.change(width, { target: { value: '' } })
    const errors = screen.getAllByText('Укажите значение')
    expect(errors.length).toBeGreaterThanOrEqual(5)
    expect(width).toHaveClass('field-invalid')
    // Скрытые поля не участвуют в валидации прямого марша.
    expect(screen.queryByLabelText('Ширина площадки (мм)')).not.toHaveClass('field-invalid')
  })

  it('ошибки пропадают после заполнения обязательных полей', async () => {
    await renderWithAuth(<Constructor />, null)
    fillValid()
    expect(screen.queryByText('Укажите значение')).not.toBeInTheDocument()
  })

  it('без габаритов помещения спрашивает; «Продолжить без площади» продолжает', async () => {
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    // Очищаем габариты помещения, чтобы сработал запрос подтверждения.
    fireEvent.change(screen.getByLabelText('Ширина помещения (мм)'), { target: { value: '' } })
    fireEvent.change(screen.getByLabelText('Длина помещения (мм)'), { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText(/Корректный расчёт/)).toBeInTheDocument()
    expect(spy).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Продолжить без площади' }))
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()

    // Выбор запоминается на сессию: повторный расчёт не спрашивает снова.
    fireEvent.change(screen.getByLabelText('Высота (мм)'), { target: { value: '2800' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(2))
  })

  it('«Внести данные площади» подсвечивает и фокусирует комнатные поля', async () => {
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.change(screen.getByLabelText('Ширина помещения (мм)'), { target: { value: '' } })
    fireEvent.change(screen.getByLabelText('Длина помещения (мм)'), { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Корректный расчёт/)

    fireEvent.click(screen.getByRole('button', { name: 'Внести данные площади' }))
    expect(screen.queryByText(/Корректный расчёт/)).not.toBeInTheDocument()
    expect(screen.getByLabelText('Ширина помещения (мм)')).toHaveClass('field-invalid')
    expect(screen.getByLabelText('Длина помещения (мм)')).toHaveClass('field-invalid')
    expect(screen.getAllByText(/Укажите (ширину|длину) помещения/)).toHaveLength(2)
    expect(spy).not.toHaveBeenCalled()

    fireEvent.change(screen.getByLabelText('Ширина помещения (мм)'), { target: { value: '3000' } })
    expect(screen.queryByText('Укажите ширину помещения')).not.toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Длина помещения (мм)'), { target: { value: '4200' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
  })

  it('кнопка «Применить» подставляет значения и снимает блокировку', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(blockedWithAdvice)
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)

    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт остановлен/)

    fireEvent.click(screen.getByRole('button', { name: 'Применить эти значения' }))

    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ step_height_mm: 166.7 })),
    )
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
    // Высота ступени больше не показывается полем ввода — применяется скрытый таргет.
    expect(screen.queryByLabelText('Высота ступени (мм)')).not.toBeInTheDocument()
  })

  it('спираль: «Применить» подставляет ширину и радиус и снимает блокировку', async () => {
    const blockedSpiral: QuoteResult = {
      validation: {
        valid: false,
        blocking: true,
        issues: [
          {
            code: 'GEO-SPIRAL-TREAD',
            severity: 'error',
            element: 'configuration',
            message: 'проступь у колонны < 100 мм',
            guide: 'Проступь 20 мм вне нормы (нужно ≥ 100 мм). Уменьшите ширину марша.',
            param: 'Радиус спирали',
            suggestions: [
              { step_count: 32, step_height_mm: 187.5, tread_depth_mm: 265, angle_deg: 35.3, outer_radius_mm: 1773, width_mm: 1263 },
            ],
          },
        ],
      },
    }
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(blockedSpiral)
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)

    const flight = screen.getByLabelText('Тип лестницы')
    fireEvent.change(flight, { target: { value: 'spiral' } })
    fillValid()
    fireEvent.change(screen.getByLabelText('Радиус (мм)'), { target: { value: '3100' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт остановлен/)

    fireEvent.click(screen.getByRole('button', { name: 'Применить эти значения' }))

    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({ width_mm: 1263, outer_radius_mm: 1773, flight: 'spiral' }),
      ),
    )
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
    expect(screen.getByLabelText('Ширина марша (мм)')).toHaveValue('1263')
    expect(screen.getByLabelText('Радиус (мм)')).toHaveValue('1773')
  })

  it('«Спасти расчёт» одним кликом применяет ближайший вариант и пересчитывает', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({ flight: 'straight', heightMM: '3000', widthMM: '1000', stepHeightMM: '166.67' }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт остановлен/)

    const rescue = screen.getByRole('button', { name: 'Спасти расчёт' })
    fireEvent.click(rescue)

    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({ step_height_mm: 166.67, flight: 'straight' }),
      ),
    )
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Спасти расчёт' })).not.toBeInTheDocument()
  })

  it('вариация (straight) подставляет высоту ступени и пересчитывает', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({ flight: 'straight', heightMM: '3000', widthMM: '1000', stepHeightMM: '166.67', comfortStepMM: '620' }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт остановлен/)
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({ step_height_mm: 166.67, flight: 'straight' }),
      ),
    )
    // Шаг комфорта из варианта не применяется — дефолт 630 на бэкенде.
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastCall['comfort_step_mm']).toBeUndefined()
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
  })

  it('вариация (l_shape) меняет тип марша и подставляет площадку', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({
          flight: 'l_shape', heightMM: '3000', widthMM: '1000', landingWidthMM: '1000',
          landingDepthMM: '1500', lowerStepCountMM: '9', stepHeightMM: '166.67', comfortStepMM: '600',
        }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт остановлен/)
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({
          flight: 'l_shape', step_height_mm: 166.67,
          landing_width_mm: 1000, lower_step_count: 9,
        }),
      ),
    )
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastCall['comfort_step_mm']).toBeUndefined()
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
  })

  it('вариация (u_shape) меняет тип марша и подставляет площадку', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({
          flight: 'u_shape', heightMM: '3000', widthMM: '1000', landingWidthMM: '1000',
          landingDepthMM: '1500', lowerStepCountMM: '9', stepHeightMM: '166.67', comfortStepMM: '620',
        }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт остановлен/)
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({
          flight: 'u_shape', step_height_mm: 166.67,
          landing_width_mm: 1000, lower_step_count: 9,
        }),
      ),
    )
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastCall['comfort_step_mm']).toBeUndefined()
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
  })

  it('вариация не затирает выбор перил пустыми полями (clobbering)', async () => {
    // Бэкенд отдаёт вариацию со всеми полями ConfigForm, в т.ч. пустыми
    // (railing/direction не заданы для прямого марша). До фикса эти пустые
    // строки перезаписывали выбор пользователя → форма невалидна → пересчёт
    // падал. После фикса пустые значения пропускаются.
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({
          flight: 'straight', heightMM: '3000', widthMM: '1000', stepHeightMM: '166.67',
          comfortStepMM: '620', railing: '', direction: '',
        }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт остановлен/)
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({
          step_height_mm: 166.67, flight: 'straight', railing: 'both',
        }),
      ),
    )
    // пустые поля вариации не должны попасть в запрос; шаг комфорта — дефолт
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastCall['railing']).toBe('both')
    expect(lastCall['direction']).toBeUndefined()
    expect(lastCall['comfort_step_mm']).toBeUndefined()
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
  })

  it('исходный марш остаётся в галерее и возвращается по клику', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({
          flight: 'l_shape', heightMM: '3000', widthMM: '1000',
          landingWidthMM: '1000', landingDepthMM: '1500', lowerStepCountMM: '9',
          stepHeightMM: '166.67',
        }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт остановлен/)

    // Галерея: исходный «Прямой марш» (выбран) + альтернатива от бэкенда.
    expect(screen.getByText('Выбран')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Прямой марш/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Угол 30°/ })).toBeInTheDocument()

    // Выбираем L-образный вариант — он становится снапшотом, прямой остаётся.
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ flight: 'l_shape' })),
    )
    expect(screen.getByRole('button', { name: /Прямой марш/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /L-образный марш/ })).toBeInTheDocument()

    // Возврат к исходному прямому маршу — полное восстановление конфига.
    fireEvent.click(screen.getByRole('button', { name: /Прямой марш/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({ width_mm: 900, height_mm: 2700, flight: 'straight' }),
      ),
    )
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
  })
})
// ---- Живая валидация при вводе (S-P5) ----
// Мок ответа публичного :validate: спираль, радиус меньше ширины марша.
const liveBlockedSpiral = {
  valid: false,
  blocking: true,
  issues: [
    {
      code: 'GEO-SPIRAL-RADIUS',
      severity: 'error',
      element: 'configuration',
      message: 'Радиус спирали не превышает ширину марша',
      param: 'Радиус спирали',
      guide: 'Наружный радиус спирали должен быть больше ширины марша.',
      suggestions: [
        { step_count: 18, step_height_mm: 150, tread_depth_mm: 290, angle_deg: 31.3, outer_radius_mm: 1050, width_mm: 900 },
      ],
    },
  ],
}

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))
const LIVE_DEBOUNCE = 700

describe('Constructor · живая валидация (S-P5)', () => {
  it('вызывает :validate при изменении полей и показывает баннер блокировки', async () => {
    const validateSpy = vi.spyOn(quoteApi, 'validate').mockResolvedValue(liveBlockedSpiral)
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)

    fillValid()
    fireEvent.change(screen.getByLabelText('Тип лестницы'), { target: { value: 'spiral' } })
    fireEvent.change(screen.getByLabelText('Радиус (мм)'), { target: { value: '800' } })

    await waitFor(() => expect(validateSpy).toHaveBeenCalledTimes(1), { timeout: 2500 })
    // Тело запроса: спираль с заполненными габаритами.
    const body = validateSpy.mock.calls[0][0] as Record<string, unknown>
    expect(body.flight).toBe('spiral')
    expect(body.width_mm).toBe(900)
    expect(body.outer_radius_mm).toBe(800)

    // Баннер блокировки с guide и подсветка подсвеченного поля.
    // Текст guide должен появиться дважды: как ошибка поля «Радиус (мм)»
    // и в баннере блокировки (S-P5).
    await waitFor(() => {
      const matches = screen.getAllByText(/Наружный радиус спирали должен быть больше ширины марша/)
      expect(matches.length).toBeGreaterThanOrEqual(2)
    }, { timeout: 2500 })
    expect(validateSpy).toHaveBeenCalledTimes(1)

    // Кнопка «Применить» из баннера: вариант советника подставляется в форму
    // и запускается полный расчёт с новым радиусом/шириной.
    fireEvent.click(screen.getByText(/Применить: 18 ступ/))
    await waitFor(() => expect(quoteApi.calculate).toHaveBeenCalledTimes(1), { timeout: 2500 })
    const calcBody = (quoteApi.calculate as ReturnType<typeof vi.fn>).mock.calls[0][0] as Record<string, unknown>
    expect(calcBody.flight).toBe('spiral')
    expect(calcBody.width_mm).toBe(900)
    expect(calcBody.outer_radius_mm).toBe(1050)
  })

  it('не дёргает :validate, пока форма имеет локальные ошибки', async () => {
    const validateSpy = vi.spyOn(quoteApi, 'validate').mockResolvedValue(liveBlockedSpiral)
    await renderWithAuth(<Constructor />, null)
    // Высота пустая — локальная ошибка «Укажите значение».
    fireEvent.change(screen.getByLabelText('Высота (мм)'), { target: { value: '9999' } })
    await sleep(LIVE_DEBOUNCE + 200)
    expect(validateSpy).not.toHaveBeenCalled()
  })
})
