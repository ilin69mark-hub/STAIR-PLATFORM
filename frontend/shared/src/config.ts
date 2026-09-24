// Конфигурация лестницы (BC-002): параметры формы и дефолтные значения.
// Значения в мм; flight — тип марша.

import type { Rates } from './types'

export const flightOptions = [
  { value: 'straight', label: 'Прямой марш' },
  { value: 'l_shape', label: 'L-образная (с площадкой)' },
  { value: 'u_shape', label: 'П-образная (с площадкой)' },
  { value: 'spiral', label: 'Спиральная (винтовая)' },
] as const

export type Flight = (typeof flightOptions)[number]['value']

// Доступные материалы конструктора (коды каталога MFG-0005). Совпадают
// с материалами DefaultMaterialRegistry и ставками RatesForm.
export const materialOptions = [
  { value: 'STEEL-S235', label: 'Сталь S235', minThicknessMM: 2, maxThicknessMM: 60, density: 7850 },
  { value: 'STEEL-CORTEN', label: 'Кортэн', minThicknessMM: 2, maxThicknessMM: 60, density: 7850 },
  { value: 'ALUM-5083', label: 'Алюминий 5083', minThicknessMM: 2, maxThicknessMM: 60, density: 2700 },
  { value: 'WOOD-OAK', label: 'Дуб', minThicknessMM: 20, maxThicknessMM: 60, density: 700 },
  { value: 'WOOD-WALNUT', label: 'Орех', minThicknessMM: 20, maxThicknessMM: 60, density: 640 },
  { value: 'WOOD-ASH', label: 'Ясень', minThicknessMM: 20, maxThicknessMM: 60, density: 690 },
  { value: 'WOOD-SOFT', label: 'Сосна', minThicknessMM: 20, maxThicknessMM: 60, density: 520 },
] as const

export type MaterialCode = (typeof materialOptions)[number]['value']

export function materialLabel(code: string): string {
  return materialOptions.find((m) => m.value === code)?.label ?? code
}

/** URL PBR-превью материала (реальная текстура вместо цветного квадратика). */
export function materialSwatch(code: string): string {
  return `/static-assets/pbr/${code}/color.jpg`
}

/** Толщина ступени по умолчанию для материала: металл 6 мм (стальной лист),
 *  дерево — 40 мм (доска/фанера под ступень). */
export function materialThicknessMM(code: string): number {
  const m = materialOptions.find((o) => o.value === code)
  if (!m) return 40
  return code.startsWith('WOOD-') ? 40 : 6
}

/** Допустимый диапазон толщины ступени для кода материала (MFG-каталог). */
export function thicknessRangeMM(code: string): { min: number; max: number } {
  const m = materialOptions.find((o) => o.value === code)
  if (!m) return { min: 2, max: 60 }
  return { min: m.minThicknessMM, max: m.maxThicknessMM }
}

/**
 * Толщина ступни, допустимая для материала. Текущее значение сохраняется,
 * если оно в допуске; иначе берётся пресет материала, вписанный в диапазон.
 * Нужна при смене материала: дерево требует ≥ 20 мм, а дефолт стора — 6 мм
 * (стальной лист), иначе бэкенд отвечает 422 MFG-MATERIAL.
 */
export function fitThicknessMM(code: string, current: string): string {
  // Правило поля точнее каталожного: у стали выпуск ступени 3–8 мм,
  // хотя сам каталожный диапазон 2–60 мм.
  const rule = rulesFor('stepThicknessMM', code as MaterialCode)
  const cat = thicknessRangeMM(code)
  const min = rule.min ?? cat.min
  const max = rule.max ?? cat.max
  const t = Number(current)
  if (Number.isFinite(t) && t >= min && t <= max) return current
  const preset = materialThicknessMM(code)
  return String(Math.min(max, Math.max(min, preset)))
}

export interface StylePreset {
  id: string
  label: string
  hint: string
  values: Partial<ConfigForm>
}

export const stylePresets: StylePreset[] = [
  {
    id: 'nordic-oak',
    label: 'Скандинавский дуб',
    hint: 'Дуб 40 мм · перила с двух сторон',
    values: { material: 'WOOD-OAK', stepThicknessMM: '40', railing: 'both' },
  },
  {
    id: 'walnut-loft',
    label: 'Орех в лофте',
    hint: 'Орех 40 мм · перила слева',
    values: { material: 'WOOD-WALNUT', stepThicknessMM: '40', railing: 'left' },
  },
  {
    id: 'steel-studio',
    label: 'Сталь',
    hint: 'Сталь 6 мм · перила с двух сторон',
    values: { material: 'STEEL-S235', stepThicknessMM: '6', railing: 'both' },
  },
  {
    id: 'corten',
    label: 'Кортэн',
    hint: 'Кортэн 6 мм · без перил',
    values: { material: 'STEEL-CORTEN', stepThicknessMM: '6', railing: 'none' },
  },
  {
    id: 'aluminium-glass',
    label: 'Алюминий',
    hint: 'Алюминий 6 мм · перила с двух сторон',
    values: { material: 'ALUM-5083', stepThicknessMM: '6', railing: 'both' },
  },
]

// ---- Перила (CONF-RAILING) ----
// Сторона отсчитывается от первой ступени по ходу подъёма: слева от
// смотрящего вперёд — левые перила, справа — правые. Для маршей с
// площадкой (L/П) выбор делается по сегментам: первый марш → площадка →
// второй марш.
export const railingOptions = [
  { value: 'none', label: 'Без перил' },
  { value: 'left', label: 'Слева' },
  { value: 'right', label: 'Справа' },
  { value: 'both', label: 'С двух сторон' },
] as const

export type RailingSide = (typeof railingOptions)[number]['value']

export function railingLabel(code: string): string {
  return railingOptions.find((r) => r.value === code)?.label ?? code
}

// ---- Направление (CONF-DIRECTION / CONF-SPIRAL-DIRECTION) ----
export const directionOptions = [
  { value: 'left', label: 'Влево' },
  { value: 'right', label: 'Вправо' },
] as const

export const spiralDirectionOptions = [
  { value: 'cw', label: 'По часовой' },
  { value: 'ccw', label: 'Против часовой' },
] as const

export type SpiralDirection = (typeof spiralDirectionOptions)[number]['value']

// railingForSpiral — автоматическая сторона перил спирали (CONF-SPIRAL-RAILING):
// по часовой — справа, против часовой — слева. Перила всегда с одной стороны.
export function railingForSpiral(dir: SpiralDirection): RailingSide {
  return dir === 'cw' ? 'right' : 'left'
}

export interface ConfigForm {
  widthMM: string
  heightMM: string
  flight: Flight
  material: MaterialCode
  stepHeightMM: string
  stringerThicknessMM: string
  stepThicknessMM: string
  riser: boolean
  clearanceMM: string
  railingHeightMM: string
  comfortStepMM: string
  landingWidthMM: string
  landingDepthMM: string
  roomWidthMM: string
  roomLengthMM: string
  approachSpaceMM: string
  lowerStepCountMM: string
  outerRadiusMM: string
  railing: RailingSide
  railingLower: RailingSide
  railingLanding: RailingSide
  railingUpper: RailingSide
  direction: (typeof directionOptions)[number]['value']
  spiralDirection: SpiralDirection
}

export const defaultConfig: ConfigForm = {
  widthMM: '900',
  heightMM: '2700',
  flight: 'straight',
  material: 'STEEL-S235',
  stepHeightMM: '180',
  stringerThicknessMM: '50',
  stepThicknessMM: '6',
  riser: true,
  clearanceMM: '80',
  railingHeightMM: '900',
  comfortStepMM: '',
  landingWidthMM: '1000',
  landingDepthMM: '1000',
  roomWidthMM: '',
  roomLengthMM: '',
  approachSpaceMM: '1000',
  lowerStepCountMM: '6',
  outerRadiusMM: '800',
  railing: 'both',
  railingLower: 'both',
  railingLanding: 'both',
  railingUpper: 'both',
  direction: 'left',
  spiralDirection: 'ccw',
}

// ---- Поля формы по типу марша (BC-002) ----
// Конструктор показывает только поля, относящиеся к выбранному маршу.
// Спираль считает шаг комфорта сама (S = 2h + b_walk), поэтому comfortStepMM
// для неё не показывается и не отправляется.

const commonFields: Array<keyof ConfigForm> = [
  'widthMM',
  'heightMM',
  'stepHeightMM',
  'stringerThicknessMM',
  'stepThicknessMM',
  'clearanceMM',
  'railingHeightMM',
  'comfortStepMM',
]

export const flightFields: Record<Flight, Array<keyof ConfigForm>> = {
  straight: [...commonFields, 'railing', 'roomWidthMM', 'roomLengthMM', 'approachSpaceMM'],
  l_shape: [...commonFields, 'landingWidthMM', 'landingDepthMM', 'roomWidthMM', 'roomLengthMM', 'lowerStepCountMM', 'railingLower', 'railingLanding', 'railingUpper', 'direction', 'approachSpaceMM'],
  u_shape: [...commonFields, 'landingWidthMM', 'landingDepthMM', 'roomWidthMM', 'roomLengthMM', 'lowerStepCountMM', 'railingLower', 'railingLanding', 'railingUpper', 'direction', 'approachSpaceMM'],
  spiral: [
    'widthMM',
    'heightMM',
    'stepHeightMM',
    'stringerThicknessMM',
    'stepThicknessMM',
    'clearanceMM',
    'railingHeightMM',
    'outerRadiusMM',
    'roomWidthMM',
    'roomLengthMM',
    'spiralDirection',
    'approachSpaceMM',
  ],
}

// ---- Ставки цены (PRC) ----

export interface RatesForm {
  steel: string
  corten: string
  alum: string
  wood: string
  walnut: string
  ash: string
  soft: string
  machinePerHour: string
  laborPerHour: string
  overheadPct: string
  marginPct: string
  discountPct: string
  taxPct: string
}

export const defaultRates: RatesForm = {
  steel: '',
  corten: '',
  alum: '',
  wood: '',
  walnut: '',
  ash: '',
  soft: '',
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
    'STEEL-CORTEN': num(f.corten),
    'ALUM-5083': num(f.alum),
    'WOOD-OAK': num(f.wood),
    'WOOD-WALNUT': num(f.walnut),
    'WOOD-ASH': num(f.ash),
    'WOOD-SOFT': num(f.soft),
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

// Базовые правила — нормы, не зависящие от материала. Материал-зависимые
// пределы (ширина/высота/толщины) задаются в materialLimits и сливаются
// через rulesFor.
export const fieldRules: Record<keyof ConfigForm, FieldRule> = {
  widthMM: { min: 300 },
  heightMM: { min: 600 },
  flight: {},
  material: {},
  stepHeightMM: { min: 150, max: 200 },
  stringerThicknessMM: { min: 30 }, // норматив GEO-STRINGER-THICKNESS
  stepThicknessMM: {},
  riser: {},
  clearanceMM: { min: 0, max: 5000, hint: '≥2000, иначе предупреждение' },
  railingHeightMM: { min: 900, max: 2000 },
  comfortStepMM: { min: 600, max: 640, hint: 'шаг комфорта 600–640' },
  landingWidthMM: { min: 600, max: 3000, hint: 'Wp ≥ ширины марша' },
  landingDepthMM: { min: 600, max: 5000, hint: 'глубина площадки (вдоль нижнего марша), ≥ ширины' },
  roomWidthMM: { min: 0, max: 8000, hint: 'ширина помещения (X), 0 — без проверки' },
  roomLengthMM: { min: 0, max: 8000, hint: 'длина помещения (Y), 0 — без проверки' },
  approachSpaceMM: { min: 1000, max: 1200, hint: 'свободное пространство перед первой ступенью (норма 1000–1200 мм)' },
  lowerStepCountMM: { min: 1, max: 100 },
  outerRadiusMM: { min: 500, max: 5000, hint: 'R > W (радиус марша)' },
  railing: {},
  railingLower: {},
  railingLanding: {},
  railingUpper: {},
  direction: {},
  spiralDirection: {},
}

// Материал-зависимые пределы (синхронизированы с каталогом MFG-0005 и
// эневлопом листов MFG-0012 на бэкенде):
// - толщина ступени: выпуск материала (сталь 3–8, алюминий 2–60, дуб 20–60);
// - косоур — толщины выпуска материала (до 60 мм);
// - ширина марша — лист для проступей (3000 мм для всех материалов);
// - высота подъёма — крупнейший лист для косоура (сталь 6000, алюм/дуб 4550).
export const materialLimits: Record<
  MaterialCode,
  Partial<Record<keyof ConfigForm, FieldRule>>
> = {
  'STEEL-S235': {
    widthMM: { max: 3000 },
    heightMM: { max: 6000 },
    stepThicknessMM: { min: 3, max: 8 },
    stringerThicknessMM: { max: 60 },
  },
  'ALUM-5083': {
    widthMM: { max: 3000 },
    heightMM: { max: 4550 },
    stepThicknessMM: { min: 2, max: 60 },
    stringerThicknessMM: { max: 60 },
  },
  'STEEL-CORTEN': {
    widthMM: { max: 3000 },
    heightMM: { max: 6000 },
    stepThicknessMM: { min: 3, max: 8 },
    stringerThicknessMM: { max: 60 },
  },
  'WOOD-OAK': {
    widthMM: { max: 3000 },
    heightMM: { max: 4550 },
    stepThicknessMM: { min: 20, max: 60 },
    stringerThicknessMM: { max: 60 },
  },
  'WOOD-WALNUT': {
    widthMM: { max: 3000 },
    heightMM: { max: 4550 },
    stepThicknessMM: { min: 20, max: 60 },
    stringerThicknessMM: { max: 60 },
  },
  'WOOD-ASH': {
    widthMM: { max: 3000 },
    heightMM: { max: 4550 },
    stepThicknessMM: { min: 20, max: 60 },
    stringerThicknessMM: { max: 60 },
  },
  'WOOD-SOFT': {
    widthMM: { max: 3000 },
    heightMM: { max: 4550 },
    stepThicknessMM: { min: 20, max: 60 },
    stringerThicknessMM: { max: 60 },
  },
}

// rulesFor возвращает правило поля для конкретного материала: базовая норма
// сливается с материал-зависимым пределом (для не зависящих от материала
// полей материал игнорируется).
export function rulesFor(key: keyof ConfigForm, material: MaterialCode): FieldRule {
  return { ...fieldRules[key], ...materialLimits[material]?.[key] }
}

export type FieldErrors = Partial<Record<keyof ConfigForm, string>>

export function validateForm(f: ConfigForm): FieldErrors {
  const errors: FieldErrors = {}
  for (const [key] of Object.entries(fieldRules) as Array<
    [keyof ConfigForm, FieldRule]
  >) {
    const rule = rulesFor(key, f.material)
    if (rule.min === undefined && rule.max === undefined) continue
    // Поля маршей с площадкой значимы только для l_shape/u_shape (EDR-0005/0006).
    if (
      key === 'landingWidthMM' ||
      key === 'landingDepthMM' ||
      key === 'lowerStepCountMM'
    ) {
      if (f.flight !== 'l_shape' && f.flight !== 'u_shape') continue
    }
    // Параметры помещения значимы для всех типов марша (прямой, L, П,
    // спираль) — fit-check выполняется геометрией для любого типа и
    // обрабатываются ниже как необязательные (пустое = без проверки).
    // Наружный радиус значим только для спирали (EDR-0007).
    if (key === 'outerRadiusMM' && f.flight !== 'spiral') continue
    // Свободное пространство перед первой ступенью значимо для ВСЕХ типов
    // марша (EDR-0023): прямой, L, П, спираль. Параметр обязателен — пустое
    // значение блокирует расчёт (ошибка «Укажите значение»), а не трактуется
    // как дефолт 1000 мм (дефолт подставляется только в toRequest/при
    // применении варианта, см. Constructor/ProjectDetail).
    if (key === 'approachSpaceMM') {
      // значимо для всех типов — проверяем как обычное обязательное поле
    }
    // Шаг комфорта и габариты помещения — необязательные (пустое = не задано).
    if (key === 'comfortStepMM' && f.flight === 'spiral') continue
    const optional =
      key === 'comfortStepMM' || key === 'roomWidthMM' || key === 'roomLengthMM'
    const raw = f[key]
    if (String(raw).trim() === '') {
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
    material: f.material,
    step_height_mm: Number(f.stepHeightMM),
    stringer_thickness_mm: Number(f.stringerThicknessMM),
    step_thickness_mm: Number(f.stepThicknessMM),
    riser: f.riser,
    clearance_mm: Number(f.clearanceMM),
    railing_height_mm: Number(f.railingHeightMM),
  }
  // Шаг комфорта передаётся кроме спирали (она считает его сама, EDR-0007).
  if (f.comfortStepMM.trim() !== '' && f.flight !== 'spiral') {
    req.comfort_step_mm = Number(f.comfortStepMM)
  }
  // Параметры маршей с площадкой передаются только для l_shape/u_shape (EDR-0005/0006).
  if (f.flight === 'l_shape' || f.flight === 'u_shape') {
    req.landing_width_mm = Number(f.landingWidthMM)
    req.landing_depth_mm = Number(f.landingDepthMM)
    req.lower_step_count = Number(f.lowerStepCountMM)
  }
  if (f.flight === 'l_shape' || f.flight === 'straight' || f.flight === 'u_shape' || f.flight === 'spiral') {
    req.room_width_mm = Number(f.roomWidthMM)
    req.room_length_mm = Number(f.roomLengthMM)
  }
  // Свободное пространство перед первой ступенью — для ВСЕХ типов марша
  // (EDR-0023): прямой, L, П, спираль. Пустое значение трактуется как
  // дефолт 1000 мм (как и в геометрии при approach == 0).
  {
    const a = (f.approachSpaceMM ?? '').trim()
    req.approach_space_mm = a === '' ? 1000 : Number(a)
  }
  // Наружный радиус передаётся только для спирали (EDR-0007).
  if (f.flight === 'spiral') {
    req.outer_radius_mm = Number(f.outerRadiusMM)
  }
  // Перила и направления (CONF-RAILING/DIRECTION/SPIRAL): прямой марш —
  // одна сторона; марши с площадкой — по сегментам + поворот площадки;
  // спираль — только направление (перила вычисляются на сервере из
  // направления закрутки, CONF-SPIRAL-RAILING).
  if (f.flight === 'straight') {
    req.railing = f.railing
  }
  if (f.flight === 'l_shape' || f.flight === 'u_shape') {
    req.railing_lower = f.railingLower
    req.railing_landing = f.railingLanding
    req.railing_upper = f.railingUpper
    req.direction = f.direction
  }
  if (f.flight === 'spiral') {
    req.spiral_direction = f.spiralDirection
  }
  return req
}