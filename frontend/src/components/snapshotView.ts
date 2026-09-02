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

export function schematicOf(s: Snapshot, approachMM?: number): SolverSchematic | null {
  // Свободное пространство перед первой ступенью (EDR-0023): предпочитаем
  // явный ввод калькулятора (approachMM), иначе — сдвиг модели (bbox.min.x).
  const approachOf = (): number | undefined => {
    if (approachMM != null && approachMM > 0) return approachMM
    const b = s.measurement?.BoundingBox?.Min.X
    return b != null && b > 1 ? b : undefined
  }
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
        // Свободное пространство перед первой ступенью равно сдвигу модели
        // от стены (BoundingBox.Min.X), задаваемому approachSpace (EDR-0023).
        approachSpace: approachOf(),
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
        railingLower: s.railing_lower,
        railingLanding: s.railing_landing,
        railingUpper: s.railing_upper,
        direction: u.direction as 'left' | 'right' | undefined,
        // Свободное пространство перед первой ступенью равно сдвигу модели
        // от стены (BoundingBox.Min.X), задаваемому approachSpace (EDR-0023).
        approachSpace: approachOf(),
      },
      railing: undefined,
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
        landingDepth: l.LandingDepth,
        roomWidth: l.RoomWidth,
        roomLength: l.RoomLength,
        // Вписываемость лестницы в помещение: габаритный бокс (мир, X — горизонталь,
        // Y — высота) сравнивается с заданным периметром (комната от начала координат).
        roomFits:
          (l.RoomWidth ?? 0) > 0 &&
          (l.RoomLength ?? 0) > 0 &&
          s.measurement?.BoundingBox != null &&
          s.measurement.BoundingBox.Max.X <= (l.RoomWidth ?? 0) + 1e-6 &&
          s.measurement.BoundingBox.Max.Y <= (l.RoomLength ?? 0) + 1e-6,
        lowerRun: l.LowerRun,
        upperRun: l.UpperRun,
        railingLower: s.railing_lower,
        railingLanding: s.railing_landing,
        railingUpper: s.railing_upper,
        direction: l.direction as 'left' | 'right' | undefined,
        // Свободное пространство перед первой ступенью равно сдвигу модели
        // от стены (BoundingBox.Min.X), задаваемому approachSpace (EDR-0023).
        approachSpace: approachOf(),
      },
      railing: undefined,
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
      solver: {
        // Свободное пространство перед первой ступенью равно сдвигу модели
        // от стены (BoundingBox.Min.X), который задаётся параметром approachSpace.
        approachSpace: approachOf(),
        // Габариты помещения для прямого марша (EDR-0023): чтобы прижать
        // ребро 1В к стене В, нужны размеры комнаты.
        roomWidth: f.RoomWidth,
        roomLength: f.RoomLength,
      },
      railing: s.railing,
    }
  }
  return null
}