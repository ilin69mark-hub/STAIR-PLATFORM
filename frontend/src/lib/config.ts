// Конфигурация лестницы (BC-002): параметры формы и дефолтные значения.
// Значения в мм; flight — тип марша.

export const flightOptions = [
  { value: 'straight', label: 'Прямой марш' },
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
  return req
}