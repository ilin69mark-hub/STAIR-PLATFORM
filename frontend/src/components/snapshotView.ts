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
  // Эффективная сторона перил (CONF-RAILING): 'none' скрывает перила на
  // схеме, отсутствие — legacy (рисуем по высоте).
  railing?: string
}

// segmentsRailing — эффективная сторона перил маршей с площадкой (L/U):
// «без перил» только когда все сегменты без перил; смешанный выбор профиль
// показывает как обычно.
function segmentsRailing(
  s: Snapshot,
): string | undefined {
  const all = [s.railing_lower, s.railing_landing, s.railing_upper]
  return all.every((v) => v !== undefined) && all.every((v) => v === 'none') ? 'none' : undefined
}

export function schematicOf(s: Snapshot): SolverSchematic | null {
  const extras = {
    StepThickness: s.step_thickness,
    RailingHeight: s.railing_height,
    Riser: s.riser,
    StringerThickness: s.stringer_thickness,
  }
  // Сначала более конкретные типы: бэкенд всегда шлёт поле flight (для
  // L/U/спирали оно нулевое), поэтому проверка flight первой съедала бы
  // все не-прямые типы (их план не отрисовывался).
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
      railing: s.railing,
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
        direction: u.direction as 'left' | 'right' | undefined,
      },
      railing: segmentsRailing(s),
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
        direction: l.direction as 'left' | 'right' | undefined,
      },
      railing: segmentsRailing(s),
    }
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
      railing: s.railing,
    }
  }
  return null
}