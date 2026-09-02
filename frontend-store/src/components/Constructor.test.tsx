import { fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Constructor } from './Constructor'
import { quoteApi } from '../api/store'
import { renderWithAuth } from '../test/render'
import type { QuoteResult, Variation } from '@shared/types'

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
  'Высота ступени (мм)': '180',
  'Толщина ступени (мм)': '40',
  'Просвет (мм)': '2000',
  'Высота перил (мм)': '900',
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
    expect(screen.getByLabelText('Материал')).toHaveValue('STEEL-S235')
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
    expect(screen.getByText('Комфортно: 150–190 мм')).toBeInTheDocument()
    expect(screen.getByText('Мин 2 / макс 60 мм')).toBeInTheDocument()
    expect(screen.getByText('Рекомендуем 900–1100 мм')).toBeInTheDocument()
  })

  it('подсказки ширины и высоты зависят от материала', async () => {
    await renderWithAuth(<Constructor />, null)
    // Сталь по умолчанию: макс. высота 6000 мм.
    expect(screen.getByText('Макс 6000 мм')).toBeInTheDocument()
    expect(screen.getByText('Макс 3000 мм')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Материал'), { target: { value: 'WOOD-OAK' } })
    // Дуб: макс. высота 4550 мм, толщина ступени 20–60 мм.
    expect(screen.getByText('Макс 4550 мм')).toBeInTheDocument()
    expect(screen.getByText('Мин 20 / макс 60 мм')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Материал'), { target: { value: 'ALUM-5083' } })
    expect(screen.getByText('Мин 2 / макс 60 мм')).toBeInTheDocument()
  })

  it('валидация учитывает пределы материала', async () => {
    await renderWithAuth(<Constructor />, null)
    fireEvent.change(screen.getByLabelText('Материал'), { target: { value: 'ALUM-5083' } })
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

  it('выбранный материал попадает в запрос расчёта', async () => {
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.change(screen.getByLabelText('Материал'), { target: { value: 'WOOD-OAK' } })
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

  it('шаг комфорта виден для прямого и L, скрыт для спирали', async () => {
    await renderWithAuth(<Constructor />, null)
    const flight = screen.getByLabelText('Тип лестницы')

    expect(screen.getByLabelText('Шаг комфорта (мм)')).toBeVisible()

    fireEvent.change(flight, { target: { value: 'l_shape' } })
    expect(screen.getByLabelText('Шаг комфорта (мм)')).toBeVisible()

    fireEvent.change(flight, { target: { value: 'spiral' } })
    expect(screen.queryByLabelText('Шаг комфорта (мм)')).not.toBeVisible()
    expect(screen.getByLabelText('Радиус (мм)')).toBeVisible()
  })

  it('подсвечивает пустые обязательные поля при старте', async () => {
    await renderWithAuth(<Constructor />, null)
    const errors = screen.getAllByText('Укажите значение')
    expect(errors.length).toBeGreaterThanOrEqual(6)
    const width = screen.getByLabelText('Ширина марша (мм)')
    expect(width).toHaveClass('field-invalid')
    // Скрытые поля не участвуют в валидации прямого марша.
    expect(screen.queryByLabelText('Ширина площадки (мм)')).not.toHaveClass('field-invalid')
  })

  it('ошибки пропадают после заполнения обязательных полей', async () => {
    await renderWithAuth(<Constructor />, null)
    fillValid()
    expect(screen.queryByText('Укажите значение')).not.toBeInTheDocument()
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
    expect(screen.getByLabelText('Высота ступени (мм)')).toHaveValue('166.7')
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

  it('вариация (straight) подставляет шаг комфорта и пересчитывает', async () => {
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
        expect.objectContaining({ step_height_mm: 166.67, comfort_step_mm: 620, flight: 'straight' }),
      ),
    )
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
          flight: 'l_shape', step_height_mm: 166.67, comfort_step_mm: 600,
          landing_width_mm: 1000, lower_step_count: 9,
        }),
      ),
    )
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
          flight: 'u_shape', step_height_mm: 166.67, comfort_step_mm: 620,
          landing_width_mm: 1000, lower_step_count: 9,
        }),
      ),
    )
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
          step_height_mm: 166.67, comfort_step_mm: 620, flight: 'straight', railing: 'both',
        }),
      ),
    )
    // пустые поля вариации не должны попасть в запрос
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastCall['railing']).toBe('both')
    expect(lastCall['direction']).toBeUndefined()
    expect(await screen.findByText(/Результат расчёта/)).toBeInTheDocument()
  })
})