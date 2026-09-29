// Конфигурация лестницы (BC-002): параметры формы и дефолтные значения.
// Значения в мм; flight — тип марша.

import type { Rates } from './types'

// Спиральный марш временно отключён: нормы EDR-0007 противоречивы, ни одна
// конфигурация не проходит проверку проступи (S-152). Код движка, каталог и
// тесты НЕ удалены — вернём вместе с исправлением норм, достаточно убрать
// флаг ниже.
export const SPIRAL_ENABLED = false

const allFlightOptions = [
  { value: 'straight', label: 'Прямой марш' },
  { value: 'l_shape', label: 'L-образная (с площадкой)' },
  { value: 'u_shape', label: 'П-образная (с площадкой)' },
  { value: 'spiral', label: 'Спиральная (винтовая)' },
] as const

export const flightOptions = SPIRAL_ENABLED
  ? allFlightOptions
  : allFlightOptions.filter((o) => o.value !== 'spiral')

export type Flight = (typeof allFlightOptions)[number]['value']

/** Все типы марша, включая отключённые (для разбора снимков и проектов). */
export const allFlightTypes = allFlightOptions.map((o) => o.value) as Flight[]

// Доступные материалы конструктора (коды каталога MFG-0005). Совпадают
// с материалами DefaultMaterialRegistry и ставками RatesForm.
export const materialOptions = [
  { value: 'STEEL-S235', label: 'Сталь S235', minThicknessMM: 2, maxThicknessMM: 60, density: 7850 },
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
// DOM-001: тип поворота площадки. «Площадка» — прямоугольная промежуточная
// площадка (поведение по умолчанию). «Поворотные ступени» — вместо площадки
// nw треугольных ступеней, разворачивающих марш на 180°.
export const turnKindOptions = [
  { value: 'platform', label: 'Площадка' },
  { value: 'winder', label: 'Поворотные ступени' },
] as const

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
  // DOM-001 (2026-09-26): тип поворота площадки и число поворотных ступеней.
  // Раньше бэкенд умел принимать turn_kind/winder_count (см. calculateRequest),
  // но форма их не могла задать — функциональность поворотных ступеней была
  // недостижима из интерфейса, а после расчёта параметры терялись при
  // сохранении ревизии (DOM-003).
  turnKind: (typeof turnKindOptions)[number]['value']
  winderCountMM: string
  // TreadMaterial — материал СТУПЕНЕЙ, отдельно от каркаса (material).
  // Нужно, потому что у дерева минимальная толщина 20 мм, а стальной косоур
  // бывает 6–10 мм: одним полем материал оба случая не описать. Пустая
  // строка → наследуется от material (вся лестница из одного материала).
  treadMaterial: string
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
  turnKind: 'platform',
  winderCountMM: '',
  // Пусто = материал ступеней наследуется от material. Дефолт нейтральный
  // СОЗНАТЕЛЬНО: он используется админкой и как база для тестов, и
  // «лестница из одного материала» не должна ломаться. Продуктовое
  // значение (металлокаркас + дуб) задаёт конструктор витрины в emptyConfig.
  treadMaterial: '',
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
  // Материал ступеней нужен любому типу марша: ступени есть везде. Стоит
  // рядом с толщиной ступени, потому что ограничен именно ей.
  'treadMaterial',
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
    // Материал ступеней у спирали тоже свой: винтовые марши делают и в стали,
    // и с деревянными ступенями.
    'treadMaterial',
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
  // DOM-001: минимум 3 поворотные ступени на 180° поворота (устойчивость
  // марша, EDR-0006 §7); верхняя граница — как у lowerStepCountMM.
  turnKind: {},
  winderCountMM: { min: 3, max: 100, hint: 'поворотных ступеней (шт)' },
  // Материал ступеней ограничений не имеет сам по себе: пределы толщины
  // берутся из materialForField('stepThicknessMM'), то есть из кода
  // материала ступеней. Запись нужна, чтобы ключ был в Record.
  treadMaterial: {},
}

// Материал-зависимые пределы (синхронизированы с каталогом MFG-0005 и
// эневлопом листов MFG-0012 на бэкенде):
// - толщина ступени: выпуск материала (сталь 3–8, дуб 20–60);
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

/**
 * Материал, которым ограничивается поле. Толщина ступени принадлежит
 * МАТЕРИАЛУ СТУПЕНЕЙ, а не каркаса: у дуба минимум 20 мм, у стали — от 2 мм.
 * Если брать пределы по одному material, то либо пришлось бы ставить стальной
 * косоур 20 мм (не делают), либо нельзя было бы выбрать деревянную ступень.
 *
 * Остальные поля (ширина/высота/толщина косоура) ограничены каркасом —
 * именно из него режут косоуры и площадки.
 */
export function materialForField(key: keyof ConfigForm, cfg: ConfigForm): MaterialCode {
  if (key === 'stepThicknessMM') {
    return (cfg.treadMaterial || cfg.material) as MaterialCode
  }
  return cfg.material
}

/** rulesFor с учётом роли поля: толщина ступени — по материалу ступеней. */
export function fieldRulesFor(key: keyof ConfigForm, cfg: ConfigForm): FieldRule {
  return rulesFor(key, materialForField(key, cfg))
}

export type FieldErrors = Partial<Record<keyof ConfigForm, string>>

export function validateForm(f: ConfigForm): FieldErrors {
  const errors: FieldErrors = {}
  for (const [key] of Object.entries(fieldRules) as Array<
    [keyof ConfigForm, FieldRule]
  >) {
    const rule = fieldRulesFor(key, f)
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
    // DOM-001: число поворотных ступеней значимо только при типе поворота
    // «поворотные ступени». При «площадке» поле пустое и не отправляется,
    // поэтому пустое значение допустимо (иначе форма была бы всегда invalid).
    if (key === 'winderCountMM') {
      if (f.flight !== 'l_shape' && f.flight !== 'u_shape') continue
      if (f.turnKind !== 'winder') continue
    }
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

/**
 * Толщина подступенка по материалу каркаса.
 *
 * Подступенок изготавливается из материала каркаса (см. маршрутизацию
 * деталей в сервисе), поэтому его толщина обязана быть допустимой для
 * каркаса, а не для материала ступеней. Если материалы совпадают,
 * наследование толщины ступени корректно и поведение прежнее.
 */
function riserThicknessFor(f: ConfigForm): number {
  const sameMaterial = !f.treadMaterial || f.treadMaterial === f.material
  if (sameMaterial) return Number(f.stepThicknessMM)
  return Math.min(
    materialThicknessMM(f.material),
    thicknessRangeMM(f.material).max,
  )
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
    // Толщина подступенка идёт по материалу КАРКАСА, а подступенок при
    // раздельных материалах стальной. Без этого поля бэкенд наследует
    // толщину ступени (40 мм у дуба) и отклоняет расчёт: стальной
    // подступенок 40 мм вне каталога MFG-0005, конфигурация блокируется.
    riser_thickness_mm: riserThicknessFor(f),
    // Материал ступеней. Пустая строка НЕ отправляется: бэкенд наследует
    // material, и лишнее поле только раздувало бы запрос.
    ...(f.treadMaterial && f.treadMaterial !== f.material
      ? { tread_material: f.treadMaterial }
      : {}),
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
  // DOM-001: тип поворота и число поворотных ступеней (только L/П-марш).
  // Для площадки winder_count не передаётся: он не имеет смысла вместе с
  // turn_kind='platform' (в БД это CHECK-ограничение).
  if (f.flight === 'l_shape' || f.flight === 'u_shape') {
    req.turn_kind = f.turnKind
    if (f.turnKind === 'winder' && f.winderCountMM.trim() !== '') {
      req.winder_count = Number(f.winderCountMM)
    }
  }
  return req
}