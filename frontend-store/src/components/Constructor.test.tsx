import { fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Constructor } from './Constructor'
import { quoteApi } from '../api/store'
import { renderWithAuth } from '../test/render'
import type { QuoteResult } from '@shared/types'

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
    expect(screen.getByText('Мин 20 / макс 200 мм')).toBeInTheDocument()
    expect(screen.getByText('Рекомендуем 900–1100 мм')).toBeInTheDocument()
  })

  it('показывает знак справки с тултипом у просвета', async () => {
    await renderWithAuth(<Constructor />, null)
    const help = screen.getByRole('tooltip')
    expect(help).toHaveTextContent(/Просвет — вертикальное расстояние от ступени до перекрытия/)
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
})