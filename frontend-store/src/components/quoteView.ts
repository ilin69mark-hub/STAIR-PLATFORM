// Преобразование публичного quote (snake_case DTO) в отображаемые
// структуры: марш, геометрию и цену.

import type {
  QuoteFlight,
  QuoteLShape,
  QuoteResult,
  QuoteSpiral,
  QuoteUShape,
} from '@shared/types'

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
}

export interface SolverView {
  flight?: FlightView
  lowerStepCount?: number
  upperStepCount?: number
  landingWidth?: number
  lowerRun?: number
  upperRun?: number
  lowerStringer?: number
  upperStringer?: number
  outerRadius?: number
  columnRadius?: number
  walkRadius?: number
  innerTread?: number
  walkTread?: number
  outerTread?: number
  angularStep?: number
  arcLength?: number
  comfortStep?: number
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
  }
}

export function solverOf(q: QuoteResult): SolverView {
  if (q.flight) {
    const f = quoteToFlight(q.flight)
    return { flight: f }
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
    }
    return {
      flight: f,
      lowerStepCount: l.lower_step_count,
      upperStepCount: l.upper_step_count,
      landingWidth: l.landing_width_mm,
      lowerRun: l.lower_run_mm,
      upperRun: l.upper_run_mm,
      lowerStringer: l.lower_stringer_mm,
      upperStringer: l.upper_stringer_mm,
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
    }
    return {
      flight: f,
      lowerStepCount: u.lower_step_count,
      upperStepCount: u.upper_step_count,
      landingWidth: u.landing_width_mm,
      lowerRun: u.lower_run_mm,
      upperRun: u.upper_run_mm,
      lowerStringer: u.lower_stringer_mm,
      upperStringer: u.upper_stringer_mm,
    }
  }
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
    }
    return {
      flight: f,
      outerRadius: s.outer_radius_mm,
      columnRadius: s.column_radius_mm,
      walkRadius: s.walk_radius_mm,
      innerTread: s.inner_tread_mm,
      walkTread: s.walk_tread_mm,
      outerTread: s.outer_tread_mm,
      angularStep: s.angular_step_deg,
      arcLength: s.arc_length_mm,
      comfortStep: s.comfort_step_mm,
    }
  }
  return {}
}