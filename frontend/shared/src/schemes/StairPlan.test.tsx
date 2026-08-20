import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { StairPlan } from './StairPlan'
import type { PlanExtras, PlanFlight } from './StairPlan'

const base: PlanFlight = {
  StepCount: 15,
  StepHeight: 180,
  TreadDepth: 280,
  Run: 4200,
  Stringer: 4783,
  Angle: Math.PI / 4,
  Width: 900,
}

describe('StairPlan', () => {
  it('возвращает null без данных', () => {
    const { container } = render(
      <StairPlan flight={base} kind="straight" solver={{}} />,
    )
    expect(container).toBeTruthy()
  })

  it('рендерит план прямого марша с размерами', () => {
    render(<StairPlan flight={base} kind="straight" solver={{}} />)
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()
    expect(screen.getByText(/L 4 200 мм/)).toBeInTheDocument()
    expect(screen.getByText(/B 900 мм/)).toBeInTheDocument()
    expect(screen.getByRole('img')).toBeInTheDocument()
  })

  it('прямой план плотно кадрирует чертёж в viewBox', () => {
    const { container } = render(<StairPlan flight={base} kind="straight" solver={{}} />)
    const vb = (container.querySelector('.scheme__svg')?.getAttribute('viewBox') ?? '0 0 560 340')
      .split(/\s+/)
      .map(Number)
    // Узкий прямой марш не должен размазываться по пустому канвасу 560×340.
    expect(vb[3]).toBeLessThan(340)
  })

  it('прямой план показывает стрелку направления подъёма', () => {
    const { container } = render(<StairPlan flight={base} kind="straight" solver={{}} />)
    const dir = container.querySelector('line.scheme__dir')
    expect(dir).not.toBeNull()
    expect(dir?.getAttribute('marker-end')).toBe('url(#pln-dir)')
    // Стрелка от первой ступени (x=0) к верху марша (x=Run).
    expect(Number(dir?.getAttribute('x2'))).toBeGreaterThan(Number(dir?.getAttribute('x1')))
  })

  it('рендерит план L-образного марша', () => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    render(<StairPlan flight={base} kind="l_shape" solver={solver} />)
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()
    expect(screen.getByText(/L₁ 1 620 мм/)).toBeInTheDocument()
    expect(screen.getByText(/L₂ 2 430 мм/)).toBeInTheDocument()
  })

  it('рендерит план П-образного марша', () => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    render(<StairPlan flight={base} kind="u_shape" solver={solver} />)
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()
    expect(screen.getByText(/L₁ 1 620 мм/)).toBeInTheDocument()
    expect(screen.getByText(/L₂ 2 430 мм/)).toBeInTheDocument()
  })

  it('рендерит план спиральной лестницы', () => {
    const solver: PlanExtras = {
      outerRadius: 800,
      columnRadius: 300,
      angularStep: 24,
      angularTotal: 360,
    }
    render(<StairPlan flight={base} kind="spiral" solver={solver} />)
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()
    expect(screen.getByText(/R 800 мм/)).toBeInTheDocument()
    expect(screen.getByText('α 360°')).toBeInTheDocument()
    expect(screen.getByText('15 ступ.')).toBeInTheDocument()
  })
})