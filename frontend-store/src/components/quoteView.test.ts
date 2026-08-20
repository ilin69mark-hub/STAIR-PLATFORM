import { describe, expect, it } from 'vitest'
import { quoteToFlight, solverOf } from './quoteView'
import type { QuoteResult } from '@shared/types'

const straight: QuoteResult = {
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
    railing: 'right',
  },
  geometry: { solid_count: 1, volume_mm3: 1e7, surface_area_mm2: 2e5, bbox: { min: { x: 0, y: 0, z: 0 }, max: { x: 900, y: 2700, z: 500 } } },
  pricing: { currency: 'RUB', material_rub: 1, machine_rub: 2, labor_rub: 3, overhead_rub: 4, production_cost_rub: 10, margin_rub: 5, discount_rub: 0, pre_tax_rub: 15, tax_rub: 3, final_price_rub: 18, lines: [] },
}

describe('quoteToFlight', () => {
  it('конвертирует snake_case DTO в camelCase марш с радианами', () => {
    const f = quoteToFlight(straight.flight!)
    expect(f.StepCount).toBe(15)
    expect(f.StepHeight).toBe(180)
    expect(f.TreadDepth).toBe(270)
    expect(f.Run).toBe(4050)
    expect(f.Stringer).toBeCloseTo(4867.49)
    expect(f.Angle).toBeCloseTo((33.69 * Math.PI) / 180)
    expect(f.Railing).toBe('right')
  })

  it('сторона прямого марша наследуется в view', () => {
    const s = solverOf(straight)
    expect(s.flight?.Railing).toBe('right')
  })
})

describe('solverOf', () => {
  it('прямой марш', () => {
    const s = solverOf(straight)
    expect(s.flight?.StepCount).toBe(15)
    expect(s.lowerStepCount).toBeUndefined()
    expect(s.outerRadius).toBeUndefined()
  })

  it('L-образный: площадка и два марша', () => {
    const q: QuoteResult = {
      ...straight,
      flight: undefined,
      lshape: {
        step_count: 12,
        lower_step_count: 6,
        upper_step_count: 6,
        step_height_mm: 180,
        tread_depth_mm: 270,
        angle_deg: 33.69,
        lower_height_mm: 1080,
        upper_height_mm: 2160,
        lower_run_mm: 1620,
        upper_run_mm: 1620,
        lower_stringer_mm: 1942.8,
        upper_stringer_mm: 1942.8,
        landing_width_mm: 900,
        width_mm: 900,
        step_thickness_mm: 40,
        railing_height_mm: 900,
        riser: true,
        stringer_thickness_mm: 50,
      },
    }
    const s = solverOf(q)
    expect(s.lowerStepCount).toBe(6)
    expect(s.upperStepCount).toBe(6)
    expect(s.landingWidth).toBe(900)
    expect(s.flight?.Run).toBeCloseTo(4140)
  })

  it('L-образный: перила отсутствуют, если все сегменты без перил', () => {
    const q: QuoteResult = {
      ...straight,
      flight: undefined,
      lshape: {
        step_count: 12,
        lower_step_count: 6,
        upper_step_count: 6,
        step_height_mm: 180,
        tread_depth_mm: 270,
        angle_deg: 33.69,
        lower_height_mm: 1080,
        upper_height_mm: 2160,
        lower_run_mm: 1620,
        upper_run_mm: 1620,
        lower_stringer_mm: 1942.8,
        upper_stringer_mm: 1942.8,
        landing_width_mm: 900,
        width_mm: 900,
        step_thickness_mm: 40,
        railing_height_mm: 900,
        riser: true,
        stringer_thickness_mm: 50,
        railing_lower: 'none',
        railing_landing: 'none',
        railing_upper: 'none',
      },
    }
    const s = solverOf(q)
    expect(s.flight?.Railing).toBe('none')
  })

  it('L-образный: смешанный выбор перил показывает схему как обычно', () => {
    const q: QuoteResult = {
      ...straight,
      flight: undefined,
      lshape: {
        step_count: 12,
        lower_step_count: 6,
        upper_step_count: 6,
        step_height_mm: 180,
        tread_depth_mm: 270,
        angle_deg: 33.69,
        lower_height_mm: 1080,
        upper_height_mm: 2160,
        lower_run_mm: 1620,
        upper_run_mm: 1620,
        lower_stringer_mm: 1942.8,
        upper_stringer_mm: 1942.8,
        landing_width_mm: 900,
        width_mm: 900,
        step_thickness_mm: 40,
        railing_height_mm: 900,
        riser: true,
        stringer_thickness_mm: 50,
        railing_lower: 'none',
        railing_landing: 'both',
        railing_upper: 'none',
      },
    }
    const s = solverOf(q)
    expect(s.flight?.Railing).toBeUndefined()
  })

  it('спираль: объект с радиусом и комфортом', () => {
    const q: QuoteResult = {
      ...straight,
      flight: undefined,
      spiral: {
        step_count: 14,
        step_height_mm: 192.86,
        outer_radius_mm: 800,
        column_radius_mm: 80,
        walk_radius_mm: 440,
        inner_tread_mm: 100,
        walk_tread_mm: 300,
        outer_tread_mm: 500,
        angle_deg: 38.1,
        angular_step_deg: 25.71,
        arc_length_mm: 7037.2,
        comfort_step_mm: 685,
        angular_total_deg: 360,
        width_mm: 900,
        step_thickness_mm: 40,
        railing_height_mm: 900,
        riser: true,
        stringer_thickness_mm: 50,
      },
    }
    const s = solverOf(q)
    expect(s.outerRadius).toBe(800)
    expect(s.comfortStep).toBe(685)
    expect(s.flight?.StepCount).toBe(14)
  })

  it('пустой результат при отсутствии маршей', () => {
    const q: QuoteResult = { validation: { valid: false, blocking: true, issues: [] } }
    expect(solverOf(q)).toEqual({})
  })
})