import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { QuoteResult } from './QuoteResult'
import type { QuoteResult as QuoteResultType } from '@shared/types'

// Захват пропсов GeometryViewer: вьювер сам идёт в shared-слой (там WebGL,
// в jsdom не строится и тестируется отдельно), здесь проверяем только, что
// QuoteResult пробрасывает в него ровно то, что получил/вывел из solver.
const viewerProps = vi.hoisted(() => ({ current: {} as Record<string, unknown> }))
vi.mock('@shared/viewer/GeometryViewer', () => ({
  GeometryViewer: (p: Record<string, unknown>) => {
    viewerProps.current = p
    return 'mock-3d-viewer'
  },
}))

const mesh = () => ({
  Vertices: [
    { X: 0, Y: 0, Z: 0 },
    { X: 900, Y: 0, Z: 0 },
    { X: 0, Y: 0, Z: 2700 },
  ],
  Triangles: [[0, 1, 2]] as Array<[number, number, number]>,
})

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
    // severity показывается по-русски, а не сырым кодом API.
    expect(screen.getByText(/Ошибка:/)).toBeInTheDocument()
    expect(screen.queryByText('Предварительная цена')).not.toBeInTheDocument()
  })

  it('подписи severity переводятся на русский', () => {
    const mixed: QuoteResultType = {
      validation: {
        valid: false,
        blocking: false,
        issues: [
          { code: 'GEO-CLEARANCE', severity: 'error', element: 'clearance', message: 'просвет мал', fix: 'увеличьте' },
          { code: 'GEO-ROOM', severity: 'warning', element: 'room', message: 'не помещается', fix: 'уменьшите' },
          { code: 'GEO-COMFORT', severity: 'info', element: 'comfort', message: 'шаг 640', fix: 'скорректируйте' },
        ],
      },
    }
    render(<QuoteResult quote={mixed} />)
    expect(screen.getByText(/Ошибка:/)).toBeInTheDocument()
    expect(screen.getByText(/Предупреждение:/)).toBeInTheDocument()
    expect(screen.getByText(/Информация:/)).toBeInTheDocument()
  })

  it('персистентные варианты рисуются галереей: активный крупно и «Выбран»', () => {
    const onApply = vi.fn()
    const blocked: QuoteResultType = {
      validation: { valid: false, blocking: true, issues: [] },
    }
    const variations = [
      { id: 'Угол 30°', title: 'Угол 30°', description: 'в норме', config: {}, fits: true, summary: 'Угол 30,2°' },
      { id: 'Угол 35°', title: 'Угол 35°', description: 'в норме', config: {}, fits: true, summary: 'Угол 35,3°' },
    ]
    render(
      <QuoteResult
        quote={blocked}
        onApplyVariation={onApply}
        variations={variations}
        activeVariationId="Угол 35°"
      />,
    )
    // Галерея: активный вариант один + уменьшенные остальные.
    expect(screen.getByText('Выбран')).toBeInTheDocument()
    const btns = screen.getAllByRole('button', { name: /Угол/ })
    expect(btns.length).toBe(2)
    // Клик по уменьшенной карточке меняет активный вариант местами.
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    expect(onApply).toHaveBeenCalledWith(expect.objectContaining({ id: 'Угол 30°' }))
  })

  it('показывает 3D-модель при наличии меша', async () => {
    const withMesh: QuoteResultType = {
      ...okQuote,
      mesh: mesh(),
    }
    render(<QuoteResult quote={withMesh} />)
    expect(screen.getByText('3D-модель')).toBeInTheDocument()
    expect(await screen.findByText('mock-3d-viewer')).toBeInTheDocument()
  })

  it('пробрасывает во вьювер heightMM и габариты помещения прямого марша', async () => {
    const withRoom: QuoteResultType = {
      ...okQuote,
      flight: {
        ...okQuote.flight!,
        room_width_mm: 3000,
        room_length_mm: 4200,
      },
      room_mesh: mesh(),
      mesh: mesh(),
    }
    render(<QuoteResult quote={withRoom} heightMM={2750} />)
    await screen.findByText('mock-3d-viewer')
    await waitFor(() => {
      expect(viewerProps.current).toMatchObject({
        flight: 'straight',
        roomWidth: 3000,
        roomLength: 4200,
        heightMM: 2750,
      })
    })
    // Меш помещения передаётся (стены строятся по его периметру).
    expect(viewerProps.current.roomMesh).toBeTruthy()
    expect(viewerProps.current.direction).toBeUndefined()
  })

  it('БЕЗ габаритов помещения в итоге вьювер получает roomWidth/roomLength undefined', async () => {
    render(<QuoteResult quote={{ ...okQuote, mesh: mesh() }} heightMM={2700} />)
    await screen.findByText('mock-3d-viewer')
    await waitFor(() => {
      expect(viewerProps.current.heightMM).toBe(2700)
      expect(viewerProps.current).toHaveProperty('roomWidth', undefined)
      expect(viewerProps.current).toHaveProperty('roomLength', undefined)
    })
  })

  it('не показывает 3D без меша', () => {
    render(<QuoteResult quote={okQuote} />)
    expect(screen.queryByText('3D-модель')).not.toBeInTheDocument()
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

  it('блок GEO-ANGLE с variations: рендерит «Варианты решения» и применяет вариант', () => {
    const onApply = vi.fn()
    const variation = (id: string, summary: string): any => ({
      id,
      title: id,
      description: 'Сделать угол наклона в норме 30–45°.',
      config: { flight: 'straight', heightMM: '3000', widthMM: '1000', stepHeightMM: '166.67', comfortStepMM: '620' },
      fits: true,
      summary,
    })
    const blocked: QuoteResultType = {
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
            variations: [variation('Угол 30°', 'Угол 30,2°, 18 ступ., h 167 мм, b 620 мм'), variation('Угол 35°', 'Угол 35,3°, 16 ступ., h 188 мм, b 640 мм')],
          },
        ],
      },
    }
    render(<QuoteResult quote={blocked} onApplyVariation={onApply} />)
    expect(screen.getByText(/Варианты решения \(выберите подходящий\)/)).toBeInTheDocument()
    const btns = screen.getAllByRole('button', { name: /Угол/ })
    expect(btns.length).toBe(2)
    fireEvent.click(btns[0])
    expect(onApply).toHaveBeenCalledWith(expect.objectContaining({ id: 'Угол 30°' }))
  })

  it('L-образный марш: варианты несут площадку и применяются', () => {
    const onApply = vi.fn()
    const blocked: QuoteResultType = {
      validation: {
        valid: false,
        blocking: true,
        issues: [
          {
            code: 'GEO-ANGLE',
            severity: 'error',
            element: 'angle',
            message: 'угол вне нормы',
            param: 'Число ступеней',
            variations: [
              {
                id: 'Угол 32°',
                title: 'Угол 32°',
                description: 'Сделать угол наклона в норме.',
                config: { flight: 'l_shape', heightMM: '3000', widthMM: '1000', landingWidthMM: '1000', landingDepthMM: '1500', lowerStepCountMM: '9', stepHeightMM: '166.67', comfortStepMM: '600' },
                fits: true,
                summary: 'Угол 32,0°, 18 ступ., h 167 мм, b 600 мм',
              },
            ],
          },
        ],
      },
    }
    render(<QuoteResult quote={blocked} onApplyVariation={onApply} />)
    expect(screen.getByText(/Варианты решения/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Угол 32°/ }))
    expect(onApply).toHaveBeenCalledWith(expect.objectContaining({ id: 'Угол 32°' }))
  })

  it('П-образный марш: варианты несут площадку и применяются', () => {
    const onApply = vi.fn()
    const blocked: QuoteResultType = {
      validation: {
        valid: false,
        blocking: true,
        issues: [
          {
            code: 'GEO-ANGLE',
            severity: 'error',
            element: 'angle',
            message: 'угол вне нормы',
            param: 'Число ступеней',
            variations: [
              {
                id: 'Угол 30°',
                title: 'Угол 30°',
                description: 'Сделать угол наклона в норме.',
                config: { flight: 'u_shape', heightMM: '3000', widthMM: '1000', landingWidthMM: '1000', landingDepthMM: '1500', lowerStepCountMM: '9', stepHeightMM: '166.67', comfortStepMM: '620' },
                fits: true,
                summary: 'Угол 30,2°, 18 ступ., h 167 мм, b 620 мм',
              },
            ],
          },
        ],
      },
    }
    render(<QuoteResult quote={blocked} onApplyVariation={onApply} />)
    expect(screen.getByText(/Варианты решения/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    expect(onApply).toHaveBeenCalledWith(expect.objectContaining({ id: 'Угол 30°' }))
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