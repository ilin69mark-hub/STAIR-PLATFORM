// Конфигурация лестницы (BC-002): параметры формы и дефолтные значения.
// Значения в мм; flight — тип марша.

import type { Rates } from './types'

// Типы марша ограничены тремя (решение владельца 2026-10-06): ограничение
// CHECK в БД сужено до тех же трёх значений.
export const flightOptions = [
  { value: 'straight', label: 'Прямой марш' },
  { value: 'l_shape', label: 'L-образная (с площадкой)' },
  { value: 'u_shape', label: 'П-образная (с площадкой)' },
] as const

export type Flight = (typeof flightOptions)[number]['value']

/** Все типы марша: для разбора снимков, проектов и предложений. */
export const allFlightTypes = flightOptions.map((o) => o.value) as Flight[]

// Доступные материалы конструктора (коды каталога MFG-0005). Совпадают
// с материалами DefaultMaterialRegistry и ставками RatesForm.
export const materialOptions = [
  // Сталь — только лазерный раскрой 3–8 мм; зеркало бэкендового
  // реестра материалов (DefaultMaterialRegistry).
  // Сталь — лазерный раскрой. Верхняя граница 12 мм, а не 8: норматив на
  // листовой косоур (решение владельца 2026-10-05) содержит строки 8 / 10 /
  // 12 мм, и каталог обязан пропускать все три.
  // Сталь — лазерный раскрой. Верхняя граница 12 мм, а не 8: норматив на
  // листовой косоур (решение владельца 2026-10-05) содержит строки 8 / 10 /
  // 12 мм, и каталог обязан пропускать все три.
  //
  // fixedTreadThicknessMM — толщина ступени НЕ выбирается: у металлической
  // ступени она одна, 5 мм (то же решение). Поле в калькуляторе при этом
  // скрывается, а значение приводится к 5 мм.
  {
    value: 'STEEL-C245',
    label: 'Сталь С245',
    minThicknessMM: 3,
    maxThicknessMM: 12,
    density: 7850,
    fixedTreadThicknessMM: 5,
  },
  { value: 'WOOD-OAK', label: 'Дуб', minThicknessMM: 20, maxThicknessMM: 100, density: 700, minTreadThicknessMM: 40 },
  { value: 'WOOD-WALNUT', label: 'Орех', minThicknessMM: 20, maxThicknessMM: 100, density: 640, minTreadThicknessMM: 40 },
  { value: 'WOOD-ASH', label: 'Ясень', minThicknessMM: 20, maxThicknessMM: 100, density: 690, minTreadThicknessMM: 40 },
  { value: 'WOOD-SOFT', label: 'Сосна', minThicknessMM: 20, maxThicknessMM: 100, density: 520, minTreadThicknessMM: 40 },
] as const

export type MaterialCode = (typeof materialOptions)[number]['value']

export function materialLabel(code: string): string {
  return materialOptions.find((m) => m.value === code)?.label ?? code
}

/** URL PBR-превью материала (реальная текстура вместо цветного квадратика). */
export function materialSwatch(code: string): string {
  return `/static-assets/pbr/${code}/color.jpg`
}

/** Толщина ступени по умолчанию для материала.
 *
 *  Металл — 5 мм: толщина ступени из металла НЕ выбирается (решение владельца
 *  2026-10-05), значение берётся из fixedTreadThicknessMM.
 *  Дерево — 40 мм (доска/фанера под ступень). */
export function materialThicknessMM(code: string): number {
  const fixed = fixedTreadThicknessMM(code)
  if (fixed) return fixed
  const m = materialOptions.find((o) => o.value === code)
  if (!m) return 40
  return code.startsWith('WOOD-') ? 40 : 6
}

/**
 * Толщина ступени, если она НЕ выбирается (металл — 5 мм), иначе 0.
 * Поле «Толщина ступени» при этом скрывается, а значение приводится к
 * фиксированному. Сервер делает то же самое (Material.FixedTreadThicknessMm),
 * поэтому клиент и расчёт не расходятся.
 */
export function fixedTreadThicknessMM(code: string): number {
  const m = materialOptions.find((o) => o.value === code) as
    | { fixedTreadThicknessMM?: number }
    | undefined
  return m?.fixedTreadThicknessMM ?? 0
}

/** Допустимый диапазон толщины ступени для кода материала (MFG-каталог).
 *
 * Нижняя граница — MinTreadThickness, а не общий минимум толщины детали:
 * у дуба ступень от 40 мм, тогда как косоур той же породы от 20 мм.
 * Значение зеркалит Material.MinTreadThicknessMm бэкенда. */
export function thicknessRangeMM(code: string): { min: number; max: number } {
  const m = materialOptions.find((o) => o.value === code) as
    | { minThicknessMM: number; maxThicknessMM: number; minTreadThicknessMM?: number }
    | undefined
  if (!m) return { min: 2, max: 60 }
  return { min: m.minTreadThicknessMM ?? m.minThicknessMM, max: m.maxThicknessMM }
}

/**
 * Толщина ступни, допустимая для материала. Текущее значение сохраняется,
 * если оно в допуске; иначе берётся пресет материала, вписанный в диапазон.
 * Нужна при смене материала: дерево требует ≥ 40 мм (решение владельца
 * 2026-10-05), а дефолт формы — 6 мм (стальной лист), иначе бэкенд
 * отвечает 422 MFG-MATERIAL.
 */
export function fitThicknessMM(code: string, current: string): string {
  // Толщина ступени из металла не выбирается: значение приводится к 5 мм
  // немедленно, независимо от того, что стояло в форме (решение владельца
  // 2026-10-05). Иначе в поле осталось бы 6 мм от прежних настроек, а
  // расчёт всё равно пошёл бы по 5 мм.
  const fixed = fixedTreadThicknessMM(code)
  if (fixed) return String(fixed)
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

// Назначение здания выбирает нормативный профиль правил (S-236):
// нормы на лестницы разные для жилых, общественных и производственных
// зданий. Пусто = дом (СП 55) — дефолт заказов без назначения.
//
// ОБЛАСТЬ ПРИМЕНЕНИЯ (решение владельца 2026-10-14): пока проектируются
// только лестницы в ЖИЛЫХ зданиях. В списке выбора оставлены только
// жилые, потому что для общественных и производственных нагрузки
// наполнены неполно: по п. 8.2.6 СП 20 величины 1,5 кН/м (трибуны и
// спортивные залы) и 0,3 кН/м (обслуживающие площадки) не соответствуют
// НИ ОДНОМУ нашему профилю, и лестница в спортивном сооружении молча
// получила бы 0,8 кН/м — недобор вдвое. Пока классов таких зданий в
// каталоге нет, опасного подбора не происходит; как только назначение
// снова станет доступным, профиль придётся довести.
//
// Полный нормативный список остаётся в purposeOptions: он нужен для типа
// Purpose, для purposeLabel и для показа СОХРАНЁННЫХ заказов. Убирать
// значения из него нельзя — старые заказы с purpose='public' перестали бы
// подписываться и потеряли бы нормативную привязку.
export const purposeOptions = [
  { value: 'house', label: 'Частный дом (СП 55)' },
  { value: 'flat', label: 'Многоквартирный дом (СП 54)' },
  { value: 'public', label: 'Общественное здание (СП 118)' },
  { value: 'industrial', label: 'Производственное здание (СП 56)' },
] as const

// Назначения, которые можно ВЫБРАТЬ в калькуляторе. Сужает список, но
// не выбрасывает значения: сохранённый заказ с другим назначением
// должен быть виден и назван, а не исчезнуть из формы.
export const selectablePurposes = ['house', 'flat'] as const

// scopeNote — причина сужения, показываемая рядом с переключателем.
// Без неё ограничение выглядит как недоработка интерфейса, а не как
// сознательное решение: пользователь решит, что общественное здание
// «сломано».
export const purposeScopeNote =
  'Пока считаем только лестницы в жилых зданиях: для общественных и производственных нагрузки на поручень и марш наполнены неполно.'

export type Purpose = (typeof purposeOptions)[number]['value']

export function purposeLabel(code: string): string {
  return purposeOptions.find((p) => p.value === code)?.label ?? code
}

/** Кнопка назначения в переключателе: то же поле плюс подсказка. */
export interface PurposeChoice {
  value: string
  label: string
  hint?: string
  disabled?: boolean
}

// purposeOptionsFor — что показать в переключателе типа здания при
// ТЕКУЩЕМ значении поля.
//
// Список ограничен жилыми (решение владельца 2026-10-14, см. выше), но
// одно исключение сделано намеренно: если заказ сохранён с назначением
// вне области — оно показывается, но недоступно для выбора. Иначе
// старый заказ с purpose='public' открылся бы с переключателем, где ни
// одна кнопка не нажата, и пользователь не понял бы, что с ним
// произошло: молчаливое исчезновение значения хуже его запрета.
//
// disabled, а не отсутствие в списке: кнопка остаётся в порядке обхода
// с клавиатуры и читается скринридером, тогда как native disabled убрал
// бы её и то, и другое (см. комментарий в Segmented).
export function purposeOptionsFor(current: string): PurposeChoice[] {
  const base: PurposeChoice[] = purposeOptions
    .filter((p) => (selectablePurposes as readonly string[]).includes(p.value))
    .map((p) => ({ value: p.value, label: p.label }))
  const known = purposeOptions.some((p) => p.value === current)
  if (known && !(selectablePurposes as readonly string[]).includes(current)) {
    const label = purposeLabel(current)
    base.push({
      value: current,
      label,
      disabled: true,
      hint: 'Вне текущей области: нагрузки для этого класса здания наполнены неполно',
    })
  }
  return base
}

// ---- Направление (CONF-DIRECTION) ----
// DOM-001: тип поворота площадки. «Площадка» — прямоугольная промежуточная
// площадка (поведение по умолчанию). «Поворотные ступени» — вместо площадки
// nw треугольных ступеней, разворачивающих марш на 180°.
export const turnKindOptions = [
  { value: 'platform', label: 'Площадка' },
  { value: 'winder', label: 'Поворотные ступени' },
] as const

// ---- Основания крепления (CONF-FIXING, решение владельца 2026-10-08) ----
// В бетон глухарь вворачивается через дюбель, в дерево — прямо. Набор
// закрыт: других оснований у лестничной пятки не бывает.
export const fixingBaseOptions = [
  { value: 'concrete', label: 'Бетон (через дюбель)' },
  { value: 'wood', label: 'Дерево (без дюбеля)' },
] as const

export const directionOptions = [
  { value: 'left', label: 'Влево' },
  { value: 'right', label: 'Вправо' },
] as const

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
  // Назначение здания (S-236): '' = дом (дефолт), поле не показывается
  // пустым — выбор делает пользователь явно.
  purpose: Purpose | ''
  comfortStepMM: string
  landingWidthMM: string
  landingDepthMM: string
  roomWidthMM: string
  ceilingHeightMM: string
  roomLengthMM: string
  approachSpaceMM: string
  lowerStepCountMM: string
  railing: RailingSide
  railingLower: RailingSide
  railingLanding: RailingSide
  railingUpper: RailingSide
  direction: (typeof directionOptions)[number]['value']
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
  // Основания крепления пятки пола и пятки выхода на перекрытие
  // (CONF-FIXING, решение владельца 2026-10-08). Влияют только на состав
  // крепежа в смете: в бетон глухарь идёт через дюбель, в дерево — без
  // него.
  floorFixing: (typeof fixingBaseOptions)[number]['value']
  ceilingFixing: (typeof fixingBaseOptions)[number]['value']
  // extraFasteners — дополнительный крепёж по объекту: список позиций
  // каталога в формате «КОД» или «КОД@количество». Считается в смете
  // СВЕРХ автоматического крепежа по отверстиям пяток.
  //
  // Это единственное поле формы, которое не строка и не число, а список:
  // остальные описывают размеры лестницы, а здесь — что докупить на
  // конкретный объект.
  extraFasteners: string[]
  // Герметик (CONF-SEALANT): толщина слоя в мм. Пусто = дефолт 1 мм.
  // Расход по формуле V = S × t, поэтому толщина прямо умножается на
  // площадь швов и меняет и расход, и цену.
  sealantLayerMM: string
  // Доп. картриджи герметика по объекту («если нужно больше»).
  extraSealantCartridges: string
}

export const defaultConfig: ConfigForm = {
  widthMM: '900',
  heightMM: '2700',
  flight: 'straight',
  material: 'STEEL-C245',
  stepHeightMM: '180',
  // Стартовая комплектация витрины — стальной каркас, поэтому толщина
  // косоура берётся из выпуска листа 3–8 мм, а не из деревянных 50.
  stringerThicknessMM: '8',
  // Стальной изделие по умолчанию: ступень из стали — всегда 5 мм (решение
  // владельца 2026-10-05), поэтому дефолт именно такой. При переключении
  // материала ступеней на дуб значение поднимается пресетом (40 мм) через
  // fitThicknessMM, а поле у металла скрыто.
  stepThicknessMM: '5',
  riser: true,
  clearanceMM: '80',
  railingHeightMM: '900',
  purpose: 'house',
  comfortStepMM: '',
  landingWidthMM: '1000',
  landingDepthMM: '1000',
  roomWidthMM: '',
  roomLengthMM: '',
  ceilingHeightMM: '0',
  approachSpaceMM: '1000',
  lowerStepCountMM: '6',
  railing: 'both',
  railingLower: 'both',
  railingLanding: 'both',
  railingUpper: 'both',
  direction: 'left',
  turnKind: 'platform',
  winderCountMM: '',
  // Пусто = материал ступеней наследуется от material. Дефолт нейтральный
  // СОЗНАТЕЛЬНО: он используется админкой и как база для тестов, и
  // «лестница из одного материала» не должна ломаться. Продуктовое
  // значение (металлокаркас + дуб) задаёт конструктор витрины в emptyConfig.
  treadMaterial: '',
  // Бетон по умолчанию: лестничные пятки крепят к перекрытию, и без
  // явного выбора смета должна показывать комплект крепежа, а не ноль.
  floorFixing: 'concrete',
  ceilingFixing: 'concrete',
  extraFasteners: [],
  sealantLayerMM: '1',
  extraSealantCartridges: '',
}

// ---- Поля формы по типу марша (BC-002) ----
// Конструктор показывает только поля, относящиеся к выбранному маршу.

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
    'STEEL-C245': num(f.steel),
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
  // Границы ИЗДЕЛИЯ (S-169). За ними сервер ничего не считает: возвращает
  // блокирующую ошибку и не строит геометрию. Поэтому они жёсткие, а не
  // «рекомендация». Нижняя граница высоты выведена из нормы угла наклона:
  // при H≈820 мм получается 28,5°, при H≈1020 мм — уже 30,4°. Материальные
  // пределы ниже по силе и сливаются через rulesFor.
  widthMM: { min: 600, max: 6000, hint: '600–6000 мм' },
  heightMM: { min: 1000, max: 5000, hint: '1000–5000 мм' },
  flight: {},
  material: {},
  stepHeightMM: { min: 150, max: 200 },
  // Минимум 30 мм — это норматив ПИЛОМАТЕРИАЛА (GEO-STRINGER-THICKNESS).
  // Для листового каркаса минимум задаёт materialLimits: 3 мм
  // (GEO-STRINGER-THICKNESS-SHEET, лазерный раскрой 3–8 мм). Правило
  // сливается по правилам rulesFor, и предел материала перекрывает базовый.
  stringerThicknessMM: { min: 30 },
  stepThicknessMM: {},
  riser: {},
  clearanceMM: { min: 0, max: 5000, hint: '≥2000, иначе предупреждение' },
  ceilingHeightMM: { min: 0, max: 12000, hint: 'отметка низа перекрытия; 0 = не задано' },
  railingHeightMM: { min: 900, max: 2000 },
  comfortStepMM: { min: 600, max: 640, hint: 'шаг комфорта 600–640' },
  landingWidthMM: { min: 600, max: 3000, hint: 'Wp ≥ ширины марша' },
  landingDepthMM: { min: 600, max: 5000, hint: 'глубина площадки (вдоль нижнего марша), ≥ ширины' },
  roomWidthMM: { min: 0, max: 8000, hint: 'ширина помещения (X), 0 — без проверки' },
  roomLengthMM: { min: 0, max: 8000, hint: 'длина помещения (Y), 0 — без проверки' },
  approachSpaceMM: { min: 1000, max: 1200, hint: 'свободное пространство перед первой ступенью (норма 1000–1200 мм)' },
  lowerStepCountMM: { min: 1, max: 100 },
  railing: {},
  railingLower: {},
  railingLanding: {},
  railingUpper: {},
  direction: {},
  // DOM-001: минимум 3 поворотные ступени на 180° поворота (устойчивость
  // марша, EDR-0006 §7); верхняя граница — как у lowerStepCountMM.
  turnKind: {},
  winderCountMM: { min: 3, max: 100, hint: 'поворотных ступеней (шт)' },
  // Материал ступеней ограничений не имеет сам по себе: пределы толщины
  // берутся из materialForField('stepThicknessMM'), то есть из кода
  // материала ступеней. Запись нужна, чтобы ключ был в Record.
  treadMaterial: {},
  // Основания крепления — перечисление без числовых пределов, поэтому
  // правило пустое: проверку «concrete или wood» делает бэкенд, а форма
  // предлагает только два значения.
  floorFixing: {},
  ceilingFixing: {},
  // Список позиций крепежа: числовые пределы к нему бессмысленны.
  extraFasteners: {},
  sealantLayerMM: {},
  extraSealantCartridges: {},
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
  'STEEL-C245': {
    widthMM: { max: 3000 },
    // 5000, а не 6000: граница изделия S-169 уже ниже прежнего предела
    // конструктора по длине листа.
    heightMM: { max: 5000 },
    // Толщина ступени из стали НЕ ВЫБИРАЕТСЯ: она всегда 5 мм (решение
    // владельца 2026-10-05), и поле в калькуляторе скрыто. Диапазон ниже —
    // не предложение выбора, а границы ВЫПУСКА листа, по которым сервер
    // проверяет уже приведённое к 5 мм значение.
    stepThicknessMM: { min: 3, max: 8 },
    // Косоур из стали — тоже лист лазерного раскроя. Верхняя граница 12 мм,
    // а не 8: норматив (решение владельца 2026-10-05) описывает строки
    // 8 / 10 / 12 мм, и с пределом 8 мм форма не дала бы ввести толщину,
    // которую норматив предписывает для длинных маршей.
    stringerThicknessMM: { min: 3, max: 12 },
  },
  'WOOD-OAK': {
    widthMM: { max: 3000 },
    heightMM: { max: 4550 },
    // Минимум СТУПЕНИ 40 мм (решение владельца 2026-10-05): под скругление
    // R10 нужна полка, а не фанера. Косоур той же породы остаётся от 20 мм —
    // предел ступени и предел детали разные вещи.
    stepThicknessMM: { min: 40, max: 60 },
    // Верхняя граница 100 мм: норматив на деревянный косоур (решение
    // владельца 2026-10-05) требует 80 мм для марша длиннее 4 м и 100 мм при
    // ширине более 1200 мм. С пределом 60 мм форма не дала бы ввести толщину,
    // которую норматив предписывает.
    stringerThicknessMM: { max: 100 },
  },
  'WOOD-WALNUT': {
    widthMM: { max: 3000 },
    heightMM: { max: 4550 },
    // Минимум СТУПЕНИ 40 мм (решение владельца 2026-10-05): под скругление
    // R10 нужна полка, а не фанера. Косоур той же породы остаётся от 20 мм —
    // предел ступени и предел детали разные вещи.
    stepThicknessMM: { min: 40, max: 60 },
    // Верхняя граница 100 мм: норматив на деревянный косоур (решение
    // владельца 2026-10-05) требует 80 мм для марша длиннее 4 м и 100 мм при
    // ширине более 1200 мм. С пределом 60 мм форма не дала бы ввести толщину,
    // которую норматив предписывает.
    stringerThicknessMM: { max: 100 },
  },
  'WOOD-ASH': {
    widthMM: { max: 3000 },
    heightMM: { max: 4550 },
    // Минимум СТУПЕНИ 40 мм (решение владельца 2026-10-05): под скругление
    // R10 нужна полка, а не фанера. Косоур той же породы остаётся от 20 мм —
    // предел ступени и предел детали разные вещи.
    stepThicknessMM: { min: 40, max: 60 },
    // Верхняя граница 100 мм: норматив на деревянный косоур (решение
    // владельца 2026-10-05) требует 80 мм для марша длиннее 4 м и 100 мм при
    // ширине более 1200 мм. С пределом 60 мм форма не дала бы ввести толщину,
    // которую норматив предписывает.
    stringerThicknessMM: { max: 100 },
  },
  'WOOD-SOFT': {
    widthMM: { max: 3000 },
    heightMM: { max: 4550 },
    // Минимум СТУПЕНИ 40 мм (решение владельца 2026-10-05): под скругление
    // R10 нужна полка, а не фанера. Косоур той же породы остаётся от 20 мм —
    // предел ступени и предел детали разные вещи.
    stepThicknessMM: { min: 40, max: 60 },
    // Верхняя граница 100 мм: норматив на деревянный косоур (решение
    // владельца 2026-10-05) требует 80 мм для марша длиннее 4 м и 100 мм при
    // ширине более 1200 мм. С пределом 60 мм форма не дала бы ввести толщину,
    // которую норматив предписывает.
    stringerThicknessMM: { max: 100 },
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

// ---- Норматив на косоур ----
//
// Убран 2026-10-12 (S-173). Раньше здесь стояла копия таблицы
// internal/domain/engineering/timber_stringer.go — в комментарии так и было
// написано: «Зеркало timber_stringer.go». Это нарушало 02_PROJECT_PRINCIPLES
// P-012 («запрещается дублирование инженерных правил»), а отличались витрина
// и админка в похожих ситуациях со ставками.
//
// Теперь норматив приходит готовым текстом с сервера в ответе
// POST /api/v1/public/stairs:validate (поле `hints`), и единственным
// источником остаётся timber_stringer.go. Разбор — adaptValidation в
// liveValidate.ts, показ — hintFor в Constructor.tsx.


// ---- Норматив на листовой (сварной) косоур (решение владельца 2026-10-05) ----
//
// Зеркало internal/domain/engineering/plate_stringer.go. В отличие от
// норматива на дерево, здесь ключ таблицы — ТОЛЩИНА ЛИСТА, а длина марша,
// ширина, глубина тела и высота заготовки — её пределы:
//
//   8 мм  → марш до 2.5 – 3.0 м, ширина до 0.9 – 1.0 м, тело ≥ 120–130, заготовка 250
//   10 мм → марш 3.0 – 4.0 м,      ширина до 1.1 – 1.2 м, тело ≥ 140–150, заготовка 300
//   12 мм → марш более 4.0 м,       ширина до 1.3 – 1.4 м, тело ≥ 160–180, заготовка 350
//
// Для расчёта полос берётся ВЕРХНИЙ конец диапазонов владельца, иначе полосы
// разошлись бы дырой (8 мм покрывает до 2.5 м, а 10 мм начинается с 3.0 м).
// Сами диапазоны сохранены в подписях.
export interface PlateStringerNorm {
  /** Нормативная толщина листа для текущих габаритов марша. */
  thicknessMM: number
  inTable: boolean
  runLabel: string
  widthLabel: string
  bodyLabel: string
  blankLabel: string
  notes: string
  /** Минимальная толщина листа для текущих габаритов марша. */
  requiredMM: number
  /** Нормативная ширина заготовки гребёнки для этой толщины, мм. */
  blankMinMM: number
}

const PLATE_BANDS = [
  { thicknessMM: 8, runMaxMM: 3000, runLabel: 'до 2.5 – 3.0 м', widthMaxMM: 1000, widthLabel: 'до 0.9 – 1.0 м', bodyLabel: 'не менее 120–130 мм', blankMinMM: 250, notes: 'Самый популярный вариант для стандартных домашних лестниц.' },
  { thicknessMM: 10, runMaxMM: 4000, runLabel: '3.0 – 4.0 м', widthMaxMM: 1200, widthLabel: 'до 1.1 – 1.2 м', bodyLabel: 'не менее 140–150 мм', blankMinMM: 300, notes: 'Повышенная жесткость. Подходит для длинных маршей без промежуточных опор. Идеально для открытых лестниц без подступенков.' },
  { thicknessMM: 12, runMaxMM: 0, runLabel: 'более 4.0 м', widthMaxMM: 1400, widthLabel: 'до 1.3 – 1.4 м', bodyLabel: 'не менее 160–180 мм', blankMinMM: 350, notes: 'Применяется при высоких нагрузках, широких маршах или при сборке мощного монокосоура (в виде спаренных пластин с проставками).' },
] as const

/** Пределы выпуска листа по нормативу владельца. */
export const PLATE_MIN_THICKNESS_MM = 8
export const PLATE_MAX_THICKNESS_MM = 12

/** Полоса норматива для проекции марша runMM. */
function plateBandForRun(runMM: number): (typeof PLATE_BANDS)[number] {
  for (const b of PLATE_BANDS) {
    if (b.runMaxMM <= 0 || runMM <= b.runMaxMM) return b
  }
  return PLATE_BANDS[PLATE_BANDS.length - 1]
}

/**
 * Минимальная толщина листа для марша длиной runMM по проекции и шириной
 * widthMM. Толщина берётся по длине, а ширина может только поднять её.
 */
export function minPlateStringerThicknessMM(runMM: number, widthMM: number): number {
  let need = plateBandForRun(runMM).thicknessMM
  for (const b of PLATE_BANDS) {
    if (widthMM <= b.widthMaxMM) {
      if (b.thicknessMM > need) need = b.thicknessMM
      break
    }
  }
  const last = PLATE_BANDS[PLATE_BANDS.length - 1]
  if (widthMM > last.widthMaxMM && last.thicknessMM > need) need = PLATE_MAX_THICKNESS_MM
  return need
}

/**
 * Каркас из листового металла: косоур режется из листа, и его толщину
 * выбирать нельзя — она выводится из норматива по длине и ширине марша
 * (решение владельца 2026-10-06). У дерева косоур это брус, там толщина
 * остаётся выбором пользователя.
 */
export function isPlateFrame(code: string): boolean {
  return code.startsWith('STEEL') || code.startsWith('METAL')
}

/**
 * Проекция марша по полям формы: n = H/h, b = S − 2h, проекция = n·b
 * (те же формулы, что в internal/engine/solver/solve.go, EDR-0001 §4).
 *
 * ОСТАЛОСЬ ЛОКАЛЬНО В ОТЛИЧИЕ от норматива выше, и это осознанно: значение
 * нужно ДО отправки формы — чтобы предзаполнить СКРЫТОЕ поле толщины
 * листа, иначе предпросмотр показывал бы одно число, а расчёт другое.
 * Это предзаполнение ввода, а не инженерное правило: сервер значение всё
 * равно приводит к нормативу сам (plate_stringer.go) и ответственен за него.
 *
 * Всё, что ПОКАЗЫВАЕТСЯ человеку — норматив, полосы, размеры заготовки —
 * приходит с сервера полем `hints` ответа :validate.
 */
const DEFAULT_COMFORT_STEP_MM = 630

function timberRunMM(f: ConfigForm): number {
  const H = Number(f.heightMM)
  const h = Number(f.stepHeightMM)
  if (!(H > 0) || !(h > 0)) return 0
  const S = f.comfortStepMM.trim() === '' ? DEFAULT_COMFORT_STEP_MM : Number(f.comfortStepMM)
  if (!(S > 0)) return 0
  const b = S - 2 * h
  if (!(b > 0)) return 0
  return Math.round((H / h) * b)
}

/**
 * Нормативная толщина листа косоура для текущих полей формы: 8 / 10 / 12 мм
 * по длине и ширине марша. 0 — данных не хватает (не заполнены высота или
 * шаг комфорта).
 *
 * Значение нужно форме, чтобы отправлять осмысленную толщину, а не остаток от
 * деревянного косоура: сервер всё равно приведёт её к нормативу, но расчёт и
 * предпросмотр должны показывать одно и то же число.
 */
export function plateStringerThicknessMM(f: ConfigForm): number {
  if (!isPlateFrame(f.material)) return 0
  const run = timberRunMM(f)
  if (run <= 0) return 0
  return minPlateStringerThicknessMM(run, Number(f.widthMM))
}

/**
 * Норматив на листовой косоур для текущих полей формы. null — каркас не
 * листовой (дерево) или не хватает чисел для оценки проекции.
 */

export type FieldErrors = Partial<Record<keyof ConfigForm, string>>

/**
 * Ошибка обязательного, но не заполненного поля.
 *
 * Конструктор витрины этот текст НЕ показывает: поле только краснеет
 * (field-invalid). Решение владельца — «и так понятно, что там надо
 * делать»; раньше текст вылезал даже под теми полями, которых
 * пользователь не касался, стоило сдвинуть любой соседний ползунок.
 * Само значение текста менять нельзя: по нему фильтруют шум в админке
 * (ProjectDetail) и проверяют e2e/юнит-тесты.
 */
export const REQUIRED_VALUE_ERROR = 'Укажите значение'

// validateForm проверяет ТОЛЬКО пределы выпуска материала: их отвергает и
// сервер (MFG-MATERIAL). Нормативы на косоур — деревянную балку и листовой
// сварный — сюда НЕ входят и показываются подсказкой под полем.
//
// Причина: норматив по определению рекомендателен (сервер выдаёт его как
// SeverityWarning), и как ошибка формы он отвергал бы конфигурацию по
// умолчанию — при подъёме 2700 мм проекция 4050 мм, а норматив на лист
// требует 12 мм, тогда как в форме стоит 8 мм. Форма не должна ругаться на
// собственные значения по умолчанию.
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
    // Параметры помещения значимы для всех типов марша (прямой, L, П) —
    // fit-check выполняется геометрией для любого типа и обрабатываются
    // ниже как необязательные (пустое = без проверки).
    // Свободное пространство перед первой ступенью значимо для ВСЕХ типов
    // марша (EDR-0023): прямой, L, П. Параметр обязателен — пустое
    // значение блокирует расчёт (ошибка «Укажите значение»), а не трактуется
    // как дефолт 1000 мм (дефолт подставляется только в toRequest/при
    // применении варианта, см. Constructor/ProjectDetail).
    if (key === 'approachSpaceMM') {
      // значимо для всех типов — проверяем как обычное обязательное поле
    }
    // Шаг комфорта и габариты помещения — необязательные (пустое = не задано).
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
      if (!optional) errors[key] = REQUIRED_VALUE_ERROR
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
 * Толщина подступенка = толщина ступени.
 *
 * Подступенок изготавливается из материала СТУПЕНЕЙ (engine/manufacturing:
 * PartRiser идёт в ветку cfg.TreadMaterial), поэтому его толщина обязана быть
 * допустимой для материала ступеней, а не каркаса. Отдельного параметра
 * толщины подступенка в панели больше нет — решение владельца «как у ступени».
 * Сервер при этом принимает клиентское riser_thickness_mm и отбрасывает его в
 * пользу толщины ступени, если оно недопустимо для материала ступеней, —
 * иначе накопленные конфигурации «стальной подступенок 6 мм + дубовые
 * ступени» падали бы с 422.
 */
function riserThicknessFor(f: ConfigForm): number {
  return Number(f.stepThicknessMM)
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
  // Назначение здания (S-236). Пусто не отправляем: бэкенд трактует
  // пустое как дом (СП 55) — дефолт владельца.
  if (f.purpose) {
    req.purpose = f.purpose
  }
  if (f.comfortStepMM.trim() !== '') {
    req.comfort_step_mm = Number(f.comfortStepMM)
  }
  // Параметры маршей с площадкой передаются только для l_shape/u_shape (EDR-0005/0006).
  if (f.flight === 'l_shape' || f.flight === 'u_shape') {
    req.landing_width_mm = Number(f.landingWidthMM)
    req.landing_depth_mm = Number(f.landingDepthMM)
    req.lower_step_count = Number(f.lowerStepCountMM)
  }
  if (f.flight === 'l_shape' || f.flight === 'straight' || f.flight === 'u_shape') {
    req.room_width_mm = Number(f.roomWidthMM)
    req.room_length_mm = Number(f.roomLengthMM)
    req.ceiling_height_mm = Number(f.ceilingHeightMM)

  // Попросить облегчённую сетку для показа (S-165). Цена и объём считаются
  // по производственной модели вторым проходом и от плотности не зависят —
  // расходиться не могут. Витрина картинку делает чаще и в 3D, поэтому меш
  // можно облегчить; точная сетка остаётся у админки (замеры, раскрой).
  req.preview_mesh = true
  }
  // Свободное пространство перед первой ступенью — для ВСЕХ типов марша
  // (EDR-0023): прямой, L, П. Пустое значение трактуется как
  // дефолт 1000 мм (как и в геометрии при approach == 0).
  {
    const a = (f.approachSpaceMM ?? '').trim()
    req.approach_space_mm = a === '' ? 1000 : Number(a)
  }
  // Перила и направления (CONF-RAILING/DIRECTION): прямой марш — одна
  // сторона; марши с площадкой — по сегментам + поворот площадки.
  if (f.flight === 'straight') {
    req.railing = f.railing
  }
  if (f.flight === 'l_shape' || f.flight === 'u_shape') {
    req.railing_lower = f.railingLower
    req.railing_landing = f.railingLanding
    req.railing_upper = f.railingUpper
    req.direction = f.direction
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
  // Основания крепления (CONF-FIXING). Отправляются всегда, даже когда
  // дефолтные: пустое значение на бэкенде тоже трактуется как бетон, но
  // явная передача делает сохранённую ревизию читаемой — видно, что
  // крепление выбирали, а не оставили на волю движку.
  req.floor_fixing = f.floorFixing ?? 'concrete'
  req.ceiling_fixing = f.ceilingFixing ?? 'concrete'
  // Доп. крепёж по объекту — только когда список не пуст, иначе в запрос
  // уходит пустой массив, который ничего не меняет.
  if (f.extraFasteners && f.extraFasteners.length > 0) {
    req.extra_fasteners = f.extraFasteners
  }
  // Герметик (CONF-SEALANT). Толщина и доп. картриджи отправляются только
  // когда заполнены: пустое значение на бэкенде трактуется как дефолт, и
  // незачем гонять нули в каждом запросе.
  const layer = (f.sealantLayerMM ?? '').trim()
  if (layer !== '') {
    req.sealant_layer_mm = Number(layer)
  }
  const extraCart = (f.extraSealantCartridges ?? '').trim()
  if (extraCart !== '') {
    req.extra_sealant_cartridges = Number(extraCart)
  }
  return req
}