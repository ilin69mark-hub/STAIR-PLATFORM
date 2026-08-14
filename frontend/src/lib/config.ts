// Конфигурация лестницы (BC-002): параметры формы и дефолтные значения.
// Значения в мм; flight — тип марша.

import type { Rates } from '../api/types'

export const flightOptions = [
  { value: 'straight', label: 'Прямой марш' },
  { value: 'l_shape', label: 'L-образная (с площадкой)' },
] as const

export type Flight = (typeof flightOptions)[number]['value']

export interface ConfigForm {
  widthMM: string
  heightMM: string
  flight: Flight
  stepHeightMM: string
  stringerThicknessMM: string
  stepThicknessMM: string
  clearanceMM: string
  railingHeightMM: string
  comfortStepMM: string
  landingWidthMM: string
  lowerStepCountMM: string
}

export const defaultConfig: ConfigForm = {
  widthMM: '900',
  heightMM: '2700',
  flight: 'straight',
  stepHeightMM: '180',
  stringerThicknessMM: '50',
  stepThicknessMM: '40',
  clearanceMM: '80',
  railingHeightMM: '900',
  comfortStepMM: '',
  landingWidthMM: '1000',
  lowerStepCountMM: '6',
}

// ---- Ставки цены (PRC) ----

export interface RatesForm {
  steel: string
  alum: string
  wood: string
  machinePerHour: string
  laborPerHour: string
  overheadPct: string
  marginPct: string
  discountPct: string
  taxPct: string
}

export const defaultRates: RatesForm = {
  steel: '',
  alum: '',
  wood: '',
  machinePerHour: '',
  laborPerHour: '',
  overheadPct: '',
  marginPct: '',
  discountPct: '',
  taxPct: '',
}

export function toRatesRequest(f: RatesForm): Rates | undefined {
  const num = (v: string) => (v.trim() === '' ? undefined : Number(v))
  const out: Rates = {}
  const mats: Record<string, number | undefined> = {
    'STEEL-S235': num(f.steel),
    'ALUM-5083': num(f.alum),
    'WOOD-OAK': num(f.wood),
  }
  const matObj = Object.fromEntries(
    Object.entries(mats).filter(([, v]) => v !== undefined),
  ) as Rates['material_per_kg_rub']
  if (matObj && Object.keys(matObj).length > 0) out.material_per_kg_rub = matObj

  const m = num(f.machinePerHour)
  if (m !== undefined) out.machine_per_hour_rub = m
  const l = num(f.laborPerHour)
  if (l !== undefined) out.labor_per_hour_rub = l
  const oh = num(f.overheadPct)
  if (oh !== undefined) out.overhead_percent = oh
  const mg = num(f.marginPct)
  if (mg !== undefined) out.margin_percent = mg
  const d = num(f.discountPct)
  if (d !== undefined) out.discount_percent = d
  const t = num(f.taxPct)
  if (t !== undefined) out.tax_percent = t

  return Object.keys(out).length > 0 ? out : undefined
}

// ---- Клиентская валидация формы (диапазоны синхронизированы с constraint.StandardProfile) ----

export interface FieldRule {
  min?: number
  max?: number
  hint?: string
}

export const fieldRules: Record<keyof ConfigForm, FieldRule> = {
  widthMM: { min: 300, max: 3000 },
  heightMM: { min: 600, max: 6000 },
  flight: {},
  stepHeightMM: { min: 150, max: 200 },
  stringerThicknessMM: { min: 30, max: 500 },
  stepThicknessMM: { min: 20, max: 200 },
  clearanceMM: { min: 0, max: 5000, hint: '≥2000, иначе предупреждение' },
  railingHeightMM: { min: 900, max: 2000 },
  comfortStepMM: { min: 600, max: 640, hint: 'шаг комфорта 600–640' },
  landingWidthMM: { min: 600, max: 3000, hint: 'Wp ≥ ширины марша' },
  lowerStepCountMM: { min: 1, max: 100 },
}

export type FieldErrors = Partial<Record<keyof ConfigForm, string>>

export function validateForm(f: ConfigForm): FieldErrors {
  const errors: FieldErrors = {}
  for (const [key, rule] of Object.entries(fieldRules) as Array<
    [keyof ConfigForm, FieldRule]
  >) {
    if (rule.min === undefined && rule.max === undefined) continue
    // Поля L-марша значимы только для l_shape.
    if (key === 'landingWidthMM' || key === 'lowerStepCountMM') {
      if (f.flight !== 'l_shape') continue
    }
    const optional = key === 'comfortStepMM'
    const raw = f[key]
    if (raw.trim() === '') {
      if (!optional) errors[key] = 'Укажите значение'
      continue
    }
    const v = Number(raw)
    if (Number.isNaN(v)) {
      errors[key] = 'Введите число'
      continue
    }
    if (rule.min !== undefined && v < rule.min) {
      errors[key] = `Не менее ${rule.min}`
    } else if (rule.max !== undefined && v > rule.max) {
      errors[key] = `Не более ${rule.max}`
    }
  }
  return errors
}

// toRequest преобразует форму в формат API (snake_case, числа в мм).
export function toRequest(f: ConfigForm): Record<string, unknown> {
  const req: Record<string, unknown> = {
    width_mm: Number(f.widthMM),
    height_mm: Number(f.heightMM),
    flight: f.flight,
    step_height_mm: Number(f.stepHeightMM),
    stringer_thickness_mm: Number(f.stringerThicknessMM),
    step_thickness_mm: Number(f.stepThicknessMM),
    clearance_mm: Number(f.clearanceMM),
    railing_height_mm: Number(f.railingHeightMM),
  }
  if (f.comfortStepMM.trim() !== '') {
    req.comfort_step_mm = Number(f.comfortStepMM)
  }
  // Параметры L-марша передаются только для l_shape (EDR-0005).
  if (f.flight === 'l_shape') {
    req.landing_width_mm = Number(f.landingWidthMM)
    req.lower_step_count = Number(f.lowerStepCountMM)
  }
  return req
}