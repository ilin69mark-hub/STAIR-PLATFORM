// Преобразование публичного quote (snake_case DTO) в отображаемые
// структуры: марш, геометрию и цену.

import type {
  QuoteFlight,
  QuoteLShape,
  QuoteResult,
  QuoteSpiral,
  QuoteUShape,
} from '@shared/types'

export type PlanKind = 'straight' | 'l_shape' | 'u_shape' | 'spiral'

export interface FlightView {
  StepCount: number
  StepHeight: number
  TreadDepth: number
  Run: number
  Stringer: number
  Angle: number
  Width: number
  StepThickness?: number
  RailingHeight?: number
  Riser?: boolean
  StringerThickness?: number
  // Эффективная сторона перил (CONF-RAILING): 'none' скрывает перила на
  // схеме, отсутствие — legacy (рисуем по высоте).
  Railing?: string
  // Габариты помещения для прижима прямого марша (EDR-0023, 1В→В).
  RoomWidth?: number
  RoomLength?: number
}

export interface SolverView {
  kind?: PlanKind
  flight?: FlightView
  lowerStepCount?: number
  upperStepCount?: number
  landingWidth?: number
  landingDepth?: number
  roomWidth?: number
  roomLength?: number
  lowerRun?: number
  upperRun?: number
  lowerStringer?: number
  upperStringer?: number
  // Перила по сегментам (CONF-RAILING): для плана L/U передаём сторону каждого
  // сегмента, отрисовка ведётся по контуру (StairPlan).
  railingLower?: string
  railingLanding?: string
  railingUpper?: string
  // Направление поворота (CONF-DIRECTION): 'left' | 'right' (план зеркалится).
  direction?: 'left' | 'right'
  outerRadius?: number
  columnRadius?: number
  walkRadius?: number
  innerTread?: number
  walkTread?: number
  outerTread?: number
  angularStep?: number
  angularTotal?: number
  arcLength?: number
  comfortStep?: number
  // approachSpace — свободное пространство перед первой ступенью прямого
  // марша (мм). Равно сдвигу модели от стены (bbox.min.x); зона рисуется
  // перед первой ступенью на плане (EDR-0023).
  approachSpace?: number
}

export function quoteToFlight(f: QuoteFlight): FlightView {
  return {
    StepCount: f.step_count,
    StepHeight: f.step_height_mm,
    TreadDepth: f.tread_depth_mm,
    Run: f.run_mm,
    Stringer: f.stringer_mm,
    Angle: (f.angle_deg * Math.PI) / 180,
    Width: f.width_mm,
    StepThickness: f.step_thickness_mm,
    RailingHeight: f.railing_height_mm,
    Riser: f.riser,
    StringerThickness: f.stringer_thickness_mm,
    Railing: f.railing,
    // Габариты помещения для прижима прямого марша (EDR-0023, 1В→В).
    RoomWidth: f.room_width_mm,
    RoomLength: f.room_length_mm,
  }
}

export function solverOf(q: QuoteResult, approachMM?: number): SolverView {
  // Свободное пространство перед первой ступенью (EDR-0023): берём явно из
  // ввода калькулятора (approachMM), иначе — из сдвига модели бэкенда
  // (bbox.min.x). Дефолт 1000 мм задаётся в конфиге калькулятора.
  const approachOf = (): number | undefined => {
    if (approachMM != null && approachMM > 0) return approachMM
    const b = q.geometry?.bbox?.min?.x
    return b != null && b > 1 ? b : undefined
  }
  // Сначала более конкретные типы: API всегда шлёт поле flight (для L/U/спирали
  // оно нулевое), поэтому проверка flight первой съедала бы все не-прямые типы.
  if (q.spiral) {
    const s: QuoteSpiral = q.spiral
    const f: FlightView = {
      StepCount: s.step_count,
      StepHeight: s.step_height_mm,
      TreadDepth: s.walk_tread_mm,
      Run: s.arc_length_mm,
      Stringer: 0,
      Angle: (s.angle_deg * Math.PI) / 180,
      Width: s.width_mm,
      StepThickness: s.step_thickness_mm,
      RailingHeight: s.railing_height_mm,
      Riser: s.riser,
      StringerThickness: s.stringer_thickness_mm,
      Railing: s.railing,
    }
    return {
      kind: 'spiral',
      flight: f,
      outerRadius: s.outer_radius_mm,
      columnRadius: s.column_radius_mm,
      walkRadius: s.walk_radius_mm,
      innerTread: s.inner_tread_mm,
      walkTread: s.walk_tread_mm,
      outerTread: s.outer_tread_mm,
      angularStep: s.angular_step_deg,
      angularTotal: s.angular_total_deg,
      arcLength: s.arc_length_mm,
      comfortStep: s.comfort_step_mm,
      // Свободное пространство перед первой ступенью равно сдвигу модели
      // от стены (bbox.min.x), задаваемому approachSpace (EDR-0023).
      approachSpace: approachOf(),
    }
  }
  if (q.ushape) {
    const u: QuoteUShape = q.ushape
    const f: FlightView = {
      StepCount: u.step_count,
      StepHeight: u.step_height_mm,
      TreadDepth: u.tread_depth_mm,
      Run: (u.lower_run_mm + u.upper_run_mm + u.landing_width_mm) * 2,
      Stringer: u.lower_stringer_mm,
      Angle: (u.angle_deg * Math.PI) / 180,
      Width: u.width_mm,
      StepThickness: u.step_thickness_mm,
      RailingHeight: u.railing_height_mm,
      Riser: u.riser,
      StringerThickness: u.stringer_thickness_mm,
      Railing: undefined,
    }
    return {
      kind: 'u_shape',
      flight: f,
      lowerStepCount: u.lower_step_count,
      upperStepCount: u.upper_step_count,
      landingWidth: u.landing_width_mm,
      lowerRun: u.lower_run_mm,
      upperRun: u.upper_run_mm,
      lowerStringer: u.lower_stringer_mm,
      upperStringer: u.upper_stringer_mm,
      railingLower: u.railing_lower,
      railingLanding: u.railing_landing,
      railingUpper: u.railing_upper,
      direction: u.direction as 'left' | 'right' | undefined,
      // Свободное пространство перед первой ступенью равно сдвигу модели
      // от стены (bbox.min.x), задаваемому approachSpace (EDR-0023).
      approachSpace: approachOf(),
    }
  }
  if (q.lshape) {
    const l: QuoteLShape = q.lshape
    const f: FlightView = {
      StepCount: l.step_count,
      StepHeight: l.step_height_mm,
      TreadDepth: l.tread_depth_mm,
      Run: l.lower_run_mm + l.upper_run_mm + l.landing_width_mm,
      Stringer: l.lower_stringer_mm,
      Angle: (l.angle_deg * Math.PI) / 180,
      Width: l.width_mm,
      StepThickness: l.step_thickness_mm,
      RailingHeight: l.railing_height_mm,
      Riser: l.riser,
      StringerThickness: l.stringer_thickness_mm,
      Railing: undefined,
    }
    return {
      kind: 'l_shape',
      flight: f,
      lowerStepCount: l.lower_step_count,
      upperStepCount: l.upper_step_count,
      landingWidth: l.landing_width_mm,
      landingDepth: l.landing_depth_mm,
      roomWidth: l.room_width_mm,
      roomLength: l.room_length_mm,
      lowerRun: l.lower_run_mm,
      upperRun: l.upper_run_mm,
      lowerStringer: l.lower_stringer_mm,
      upperStringer: l.upper_stringer_mm,
      railingLower: l.railing_lower,
      railingLanding: l.railing_landing,
      railingUpper: l.railing_upper,
      direction: l.direction as 'left' | 'right' | undefined,
      // Свободное пространство перед первой ступенью равно сдвигу модели
      // от стены (bbox.min.x), задаваемому approachSpace (EDR-0023).
      approachSpace: approachOf(),
    }
  }
  if (q.flight) {
    const f = quoteToFlight(q.flight)
    // Свободное пространство перед первой ступенью равно сдвигу модели от
    // стены (bbox.min.x), задаваемому approachSpace (EDR-0023).
    return { kind: 'straight', flight: f, approachSpace: approachOf(), roomWidth: f.RoomWidth, roomLength: f.RoomLength }
  }
  return {}
}