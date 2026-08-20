import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { QuoteResult } from './QuoteResult'
import type { QuoteResult as QuoteResultType } from '@shared/types'

vi.mock('@shared/viewer/GeometryViewer', () => ({
  GeometryViewer: () => 'mock-3d-viewer',
}))

const okQuote: QuoteResultType = {
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

const blocked: QuoteResultType = {
  validation: {
    valid: false,
    blocking: true,
    issues: [
      { code: 'GEO-CLEARANCE', severity: 'error', element: 'clearance', message: 'просвет мал', fix: 'увеличьте' },
    ],
  },
}

describe('QuoteResult', () => {
  it('показывает марш, геометрию и предварительную цену', () => {
    render(<QuoteResult quote={okQuote} />)
    expect(screen.getByText(/Результат расчёта/)).toBeInTheDocument()
    expect(screen.getByText('15')).toBeInTheDocument()
    expect(screen.getByText('Предварительная цена')).toBeInTheDocument()
    expect(screen.getByText('18,00 ₽')).toBeInTheDocument()
  })

  it('блокирующий расчёт: показывает нарушения и не даёт цену', () => {
    render(<QuoteResult quote={blocked} />)
    expect(screen.getByText(/Расчёт остановлен/)).toBeInTheDocument()
    expect(screen.getByText(/просвет мал/)).toBeInTheDocument()
    expect(screen.queryByText('Предварительная цена')).not.toBeInTheDocument()
  })

  it('показывает 3D-модель при наличии меша', async () => {
    const withMesh: QuoteResultType = {
      ...okQuote,
      mesh: {
        Vertices: [
          { X: 0, Y: 0, Z: 0 },
          { X: 900, Y: 0, Z: 0 },
          { X: 0, Y: 0, Z: 2700 },
        ],
        Triangles: [
          [0, 1, 2],
        ],
      },
    }
    render(<QuoteResult quote={withMesh} />)
    expect(screen.getByText('3D-модель')).toBeInTheDocument()
    expect(await screen.findByText('mock-3d-viewer')).toBeInTheDocument()
  })

  it('не показывает 3D без меша', () => {
    render(<QuoteResult quote={okQuote} />)
    expect(screen.queryByText('3D-модель')).not.toBeInTheDocument()
  })

  it('«без перил» прячет перила на схеме профиля', () => {
    const noRail: QuoteResultType = {
      ...okQuote,
      flight: { ...okQuote.flight!, railing: 'none' },
    }
    const { container } = render(<QuoteResult quote={noRail} />)
    expect(container.querySelector('.scheme__caption')?.textContent).not.toContain('перила')
    expect(container.querySelectorAll('.scheme__railing').length).toBe(0)
  })

  it('переключает 2D-схему: профиль и вид сверху', () => {
    render(<QuoteResult quote={okQuote} />)
    // По умолчанию профиль.
    expect(screen.getByRole('img', { name: 'Боковой профиль прямого марша' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Вид сверху' }))
    expect(screen.getByRole('img', { name: 'Вид сверху (план) лестницы' })).toBeInTheDocument()
    expect(screen.queryByRole('img', { name: 'Боковой профиль прямого марша' })).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Профиль' }))
    expect(screen.getByRole('img', { name: 'Боковой профиль прямого марша' })).toBeInTheDocument()
  })

  it('L-образный марш: только план, без вкладки «Профиль»', () => {
    const lQuote: QuoteResultType = {
      validation: { valid: true, blocking: false, issues: [] },
      lshape: {
        step_count: 15,
        lower_step_count: 6,
        upper_step_count: 9,
        step_height_mm: 180,
        tread_depth_mm: 270,
        angle_deg: 30,
        lower_height_mm: 1080,
        upper_height_mm: 1620,
        lower_run_mm: 1620,
        upper_run_mm: 2430,
        lower_stringer_mm: 1870.9,
        upper_stringer_mm: 2805.9,
        landing_width_mm: 1000,
        width_mm: 900,
        step_thickness_mm: 40,
        railing_height_mm: 900,
        riser: true,
        stringer_thickness_mm: 50,
      },
    }
    render(<QuoteResult quote={lQuote} />)
    // Нет мёртвой вкладки «Профиль»…
    expect(screen.queryByRole('button', { name: 'Профиль' })).not.toBeInTheDocument()
    // …а схема по умолчанию — план L.
    expect(screen.getByRole('img', { name: 'Вид сверху (план) лестницы' })).toBeInTheDocument()
    expect(screen.getByText(/L₁ 1 620 мм/)).toBeInTheDocument()
  })

  it('П-образный марш: только план, без вкладки «Профиль»', () => {
    const uQuote: QuoteResultType = {
      validation: { valid: true, blocking: false, issues: [] },
      ushape: {
        step_count: 15,
        lower_step_count: 6,
        upper_step_count: 9,
        step_height_mm: 180,
        tread_depth_mm: 270,
        angle_deg: 30,
        lower_height_mm: 1080,
        upper_height_mm: 1620,
        lower_run_mm: 1620,
        upper_run_mm: 2430,
        lower_stringer_mm: 1870.9,
        upper_stringer_mm: 2805.9,
        landing_width_mm: 1000,
        width_mm: 900,
        step_thickness_mm: 40,
        railing_height_mm: 900,
        riser: true,
        stringer_thickness_mm: 50,
      },
    }
    render(<QuoteResult quote={uQuote} />)
    expect(screen.queryByRole('button', { name: 'Профиль' })).not.toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Вид сверху (план) лестницы' })).toBeInTheDocument()
    expect(screen.getByText(/L₁ 1 620 мм/)).toBeInTheDocument()
  })

  it('показывает объяснение, что поправить, и кнопку применения варианта', () => {
    const onApply = vi.fn()
    const suggestion = {
      step_count: 18,
      step_height_mm: 166.7,
      tread_depth_mm: 266.7,
      angle_deg: 32,
    }
    const blockedAdvice: QuoteResultType = {
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
            suggestions: [suggestion],
          },
        ],
      },
    }
    render(<QuoteResult quote={blockedAdvice} onApplySuggestion={onApply} />)

    expect(screen.getByText(/Угол наклона 29,7°/)).toBeInTheDocument()
    expect(screen.getByText('Что поправить: Число ступеней')).toBeInTheDocument()
    expect(screen.getByText(/18 ступ\./)).toBeInTheDocument()
    expect(screen.getByText(/h 166,7 мм/)).toBeInTheDocument()
    expect(screen.getByText(/проступь 266,7 мм/)).toBeInTheDocument()
    expect(screen.getByText(/угол 32\.0°/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Применить эти значения' }))
    expect(onApply).toHaveBeenCalledWith(suggestion)
  })

  it('спиральный марш показывает варианты с радиусом и применяет их', () => {
    const onApply = vi.fn()
    const blockedSpiral: QuoteResultType = {
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
              {
                step_count: 32,
                step_height_mm: 187.5,
                tread_depth_mm: 265,
                angle_deg: 35.3,
                outer_radius_mm: 1773,
                width_mm: 1263,
              },
            ],
          },
        ],
      },
      spiral: {
        step_count: 18,
        step_height_mm: 166.7,
        outer_radius_mm: 900,
        width_mm: 900,
        column_radius_mm: 100,
        walk_radius_mm: 633.33,
        inner_tread_mm: 87.27,
        walk_tread_mm: 266.7,
        outer_tread_mm: 314.16,
        angle_deg: 32,
        angular_step_deg: 20,
        arc_length_mm: 5654.87,
        comfort_step_mm: 685,
        angular_total_deg: 360,
        step_thickness_mm: 40,
        railing_height_mm: 900,
        riser: true,
        stringer_thickness_mm: 50,
      },
    }
    render(<QuoteResult quote={blockedSpiral} onApplySuggestion={onApply} />)

    expect(screen.getByText(/32 ступ\./)).toBeInTheDocument()
    expect(screen.getByText(/ширина 1[^\d]263 мм/)).toBeInTheDocument()
    expect(screen.getByText(/радиус 1[^\d]773 мм/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Применить эти значения' }))
    expect(onApply).toHaveBeenCalledWith(blockedSpiral.validation.issues[0].suggestions?.[0])
  })
})