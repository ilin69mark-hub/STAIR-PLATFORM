import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { StairPlan, type PlanExtras, type PlanFlight, type PlanKind } from '@shared/schemes/StairPlan'
import cases from '../test/generated-cases-500.json'

interface GenCase {
  flight: string
  width_mm: number
  height_mm: number
  step_height_mm: number
  step_thickness_mm: number
  stringer_thickness_mm: number
  railing_height_mm: number
  railing: string
  approach_space_mm: number
  landing_width_mm: number
  landing_depth_mm: number
  lower_step_count: number
  outer_radius_mm: number
  direction: string
  comfort_step_mm: number
}

const all = cases as GenCase[]

function toPlan(c: GenCase): { flight: PlanFlight; kind: PlanKind; solver: PlanExtras } {
  const n = Math.max(2, Math.round(c.height_mm / c.step_height_mm))
  const h = c.height_mm / n
  const tread = Math.min(320, Math.max(240, c.comfort_step_mm - 2 * h))
  const run = tread * n
  const flight: PlanFlight = {
    StepCount: n,
    StepHeight: h,
    TreadDepth: tread,
    Run: run,
    Stringer: Math.hypot(run, c.height_mm),
    Angle: Math.atan2(c.height_mm, run),
    Width: c.width_mm,
    RailingHeight: c.railing_height_mm,
    Railing: c.railing,
  }
  const kind = c.flight as PlanKind
  const solver: PlanExtras = { approachSpace: c.approach_space_mm }
  if (c.flight === 'l_shape' || c.flight === 'u_shape') {
    const lower = c.lower_step_count > 0 ? c.lower_step_count : Math.floor(n / 2)
    solver.lowerStepCount = lower
    solver.upperStepCount = n - lower
    solver.landingWidth = c.landing_width_mm
    solver.landingDepth = c.landing_depth_mm
    solver.lowerRun = tread * lower
    solver.upperRun = tread * (n - lower)
    solver.railingLower = c.railing
    solver.railingLanding = c.railing
    solver.railingUpper = c.railing
    solver.direction = c.direction as 'left' | 'right'
  }
  if (c.flight === 'spiral') {
    solver.outerRadius = c.outer_radius_mm
    solver.columnRadius = Math.max(50, c.outer_radius_mm - c.width_mm)
    solver.angularStep = 360 / n
    solver.angularTotal = 360
  }
  return { flight, kind, solver }
}

describe('StairPlan.generated 500/flight', () => {
  it.each(all.map((c, i) => [`${c.flight}_${i}`, c] as const))('рендерит %s без обрезки', (_label, c) => {
    const { flight, kind, solver } = toPlan(c)
    const { container, unmount } = render(<StairPlan flight={flight} kind={kind} solver={solver} />)
    try {
      // SVG обязан быть, чертёж — внутри viewBox (не обрезан).
      const svg = container.querySelector('.scheme__svg')
      expect(svg).not.toBeNull()
      const vb = (svg?.getAttribute('viewBox') ?? '0 0 620 340').split(/\s+/).map(Number)
      const [vx, vy, vw, vh] = vb
      const bad: string[] = []
      container.querySelectorAll('.scheme__fill').forEach((r) => {
        const x = Number(r.getAttribute('x'))
        const y = Number(r.getAttribute('y'))
        const w = Number(r.getAttribute('width'))
        const hgt = Number(r.getAttribute('height'))
        if (x < vx || y < vy || x + w > vx + vw || y + hgt > vy + vh) bad.push(`rect ${x},${y},${w},${hgt}`)
      })
      expect(bad).toEqual([])
    } finally {
      unmount()
    }
  })
})
