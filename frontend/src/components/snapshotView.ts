// Преобразование снапшота результата (admin Snapshot) в вид для чертёжных
// схем: вид сверху (StairPlan). Ширина марша не входит в результаты Solver,
// для схемы используется типовое значение по умолчанию (900 мм).

import type { Snapshot } from '@shared/types'
import type { PlanExtras, PlanFlight, PlanKind } from '@shared/schemes/StairPlan'

const DEFAULT_WIDTH = 900

export interface SolverSchematic {
  kind: PlanKind
  flight: PlanFlight
  solver: PlanExtras
}

export function schematicOf(s: Snapshot): SolverSchematic | null {
  const extras = {
    StepThickness: s.step_thickness,
    RailingHeight: s.railing_height,
    Riser: s.riser,
    StringerThickness: s.stringer_thickness,
  }
  if (s.flight) {
    const f = s.flight
    return {
      kind: 'straight',
      flight: {
        StepCount: f.StepCount,
        StepHeight: f.StepHeight,
        TreadDepth: f.TreadDepth,
        Run: f.Run,
        Stringer: f.Stringer,
        Angle: f.Angle,
        Width: DEFAULT_WIDTH,
        ...extras,
      },
      solver: {},
    }
  }
  if (s.lshape) {
    const l = s.lshape
    return {
      kind: 'l_shape',
      flight: {
        StepCount: l.StepCount,
        StepHeight: l.StepHeight,
        TreadDepth: l.TreadDepth,
        Run: l.LowerRun + l.UpperRun + l.LandingWidth,
        Stringer: l.LowerStringer,
        Angle: l.Angle,
        Width: DEFAULT_WIDTH,
        ...extras,
      },
      solver: {
        lowerStepCount: l.LowerStepCount,
        upperStepCount: l.UpperStepCount,
        landingWidth: l.LandingWidth,
        lowerRun: l.LowerRun,
        upperRun: l.UpperRun,
      },
    }
  }
  if (s.ushape) {
    const u = s.ushape
    return {
      kind: 'u_shape',
      flight: {
        StepCount: u.StepCount,
        StepHeight: u.StepHeight,
        TreadDepth: u.TreadDepth,
        Run: (u.LowerRun + u.UpperRun + u.LandingWidth) * 2,
        Stringer: u.LowerStringer,
        Angle: u.Angle,
        Width: DEFAULT_WIDTH,
        ...extras,
      },
      solver: {
        lowerStepCount: u.LowerStepCount,
        upperStepCount: u.UpperStepCount,
        landingWidth: u.LandingWidth,
        lowerRun: u.LowerRun,
        upperRun: u.UpperRun,
      },
    }
  }
  if (s.spiral) {
    const sp = s.spiral
    return {
      kind: 'spiral',
      flight: {
        StepCount: sp.StepCount,
        StepHeight: sp.StepHeight,
        TreadDepth: sp.WalkTread,
        Run: sp.ArcLength,
        Stringer: 0,
        Angle: sp.Angle,
        Width: DEFAULT_WIDTH,
        ...extras,
      },
      solver: {
        outerRadius: sp.OuterRadius,
        columnRadius: sp.ColumnRadius,
        walkRadius: sp.WalkRadius,
        innerTread: sp.InnerTread,
        outerTread: sp.OuterTread,
        angularStep: (sp.AngularStep * 180) / Math.PI,
        angularTotal: (sp.AngularTotal * 180) / Math.PI,
        arcLength: sp.ArcLength,
      },
    }
  }
  return null
}