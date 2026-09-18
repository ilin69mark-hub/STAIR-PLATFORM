// Живая валидация при вводе (S-P5): дебаунс + дедуп по конфигу + единый
// маппинг ответа бэкенда (:validate) на форму. Используется стора-конструктором
// (quoteApi.validate, анонимный публичный эндпоинт) и админ-конструктором
// (stairsApi.validate). Ответ валидации на бэкенде повторяет блок validation
// расчёта: blocking-issue приходят с готовыми Suggestions и Variations (A/B/C).
//
// Локальная валидация формы (validateForm) остаётся синхронной и первой
// линией; живая валидация ловит то, что локально не видно (советы геометрии:
// R>W спирали, проступь/угол по норме, вписываемость в помещение и т.п.).

import type { ConfigForm, FieldErrors } from './config'

// Задержка перед отправкой запроса: гасим пачки правок при скоростном вводе.
export const VALIDATE_DEBOUNCE_MS = 700

// ---- Нормированная модель (не зависит от DTO: store snake_case / админ PascalCase) ----

export interface LiveSuggestion {
  stepCount: number
  lowerStepCount?: number
  stepHeightMm: number
  treadDepthMm: number
  angleDeg: number
  outerRadiusMm?: number
  widthMm?: number
}

export interface LiveVariation {
  id: string
  title: string
  description: string
  config: Record<string, string>
  fits: boolean
  summary: string
}

export interface LiveIssue {
  code: string
  severity: string
  element: string
  message: string
  guide?: string
  param?: string
  fix?: string
  suggestions?: LiveSuggestion[]
  variations?: LiveVariation[]
}

export interface LiveValidation {
  valid: boolean
  blocking: boolean
  issues: LiveIssue[]
  /** Ошибки, привязанные к полям формы (уже нормированные в FieldErrors). */
  fieldErrors: FieldErrors
}

// ---- Raw-формы ответа :validate (обе сущности, чтобы не дублировать типы) ----

interface RawIssue {
  code?: string
  Code?: string
  severity?: string
  Severity?: string
  element?: string
  Element?: string
  message?: string
  Message?: string
  guide?: string
  Guide?: string
  param?: string
  Param?: string
  fix?: string
  Fix?: string
  suggestions?: unknown[]
  Suggestions?: unknown[]
  variations?: unknown[]
  Variations?: unknown[]
}

interface RawValidation {
  valid?: boolean
  Valid?: boolean
  blocking?: boolean
  Blocking?: boolean
  issues?: RawIssue[]
  Issues?: RawIssue[]
}

type MaybeNumber = number | undefined

const num = (a: unknown, b: unknown): MaybeNumber => {
  const v = (a ?? b) as MaybeNumber
  return v === undefined || Number.isNaN(Number(v)) ? undefined : Number(v)
}
const strv = (a: unknown, b: unknown): string => (a as string) ?? (b as string) ?? ''
const has = (a: unknown): boolean => a !== undefined && a !== null && a !== ''

// ---- Маппинг element/param бэкенда на поля формы ----

// Element-validation engine (bindings в validation/validate.go). Проступь
// (tread_depth) и угол (angle) — производные: правится шаг ступени.
const ELEMENT_TO_KEY: Record<string, keyof ConfigForm> = {
  step_height: 'stepHeightMM',
  tread_depth: 'stepHeightMM',
  angle: 'stepHeightMM',
  clearance: 'clearanceMM',
  stringer_thickness: 'stringerThicknessMM',
  railing_height: 'railingHeightMM',
  stringer: 'stringerThicknessMM',
}

// InputError'ы приходят с Element=«configuration» и человекочитаемым
// Param (русское имя поля, см. solver/input.go и errors.go).
const PARAM_TO_KEY: Record<string, keyof ConfigForm> = {
  'Радиус спирали': 'outerRadiusMM',
  'Ширина марша': 'widthMM',
  'Ширина площадки': 'landingWidthMM',
  'Высота': 'heightMM',
  'Высота ступени': 'stepHeightMM',
  'Толщина косоура': 'stringerThicknessMM',
  'Толщина ступени': 'stepThicknessMM',
  'Просвет': 'clearanceMM',
  'Высота перил': 'railingHeightMM',
  'Материал': 'material',
  'Нижних ступеней': 'lowerStepCountMM',
  'Тип лестницы': 'flight',
  'Перила': 'railing',
  'Направление поворота': 'direction',
  'Направление спирали': 'spiralDirection',
}

// Вписываемость в помещение (geometry room_fit) подсвечивает оба габарита.
const ROOM_KEYS: Array<keyof ConfigForm> = ['roomWidthMM', 'roomLengthMM']

export function fieldKeysFor(format: LiveIssue): Array<keyof ConfigForm> {
  const keyword = strv(format.element, '')
  if (keyword === 'room') return ROOM_KEYS
  const direct = ELEMENT_TO_KEY[keyword]
  if (direct) return [direct]
  const paramKey = PARAM_TO_KEY[strv(format.param, '').trim()]
  return paramKey ? [paramKey] : []
}

// ---- Адаптация raw-ответа в нормированную модель ----

function adaptSuggestion(raw?: Record<string, unknown>): LiveSuggestion {
  if (!raw) return { stepCount: 0, stepHeightMm: 0, treadDepthMm: 0, angleDeg: 0 }
  return {
    stepCount: num(raw.step_count, raw.StepCount) ?? 0,
    lowerStepCount: num(raw.lower_step_count, raw.LowerStepCount),
    stepHeightMm: num(raw.step_height_mm, raw.StepHeightMm) ?? 0,
    treadDepthMm: num(raw.tread_depth_mm, raw.TreadDepthMm) ?? 0,
    angleDeg: num(raw.angle_deg, raw.AngleDeg) ?? 0,
    outerRadiusMm: num(raw.outer_radius_mm, raw.OuterRadiusMm),
    widthMm: num(raw.width_mm, raw.WidthMm),
  }
}

function adaptVariation(raw?: Record<string, unknown>): LiveVariation {
  if (!raw) {
    return { id: '', title: '', description: '', config: {}, fits: false, summary: '' }
  }
  return {
    id: strv(raw.id, raw.ID),
    title: strv(raw.title, raw.Title),
    description: strv(raw.description, raw.Description),
    config: (raw.config as Record<string, string>) ?? (raw.Config as Record<string, string>) ?? {},
    fits: (raw.fits as boolean) ?? (raw.Fits as boolean) ?? false,
    summary: strv(raw.summary, raw.Summary),
  }
}

export function adaptValidation(raw: RawValidation): LiveValidation {
  const issuesRaw = raw?.issues ?? raw?.Issues ?? []
  const issues: LiveIssue[] = issuesRaw.map((r) => {
    const element = strv(r.element, r.Element)
    const message = strv(r.message, r.Message)
    const code = strv(r.code, r.Code)
    const severity = strv(r.severity, r.Severity)
    const param = has(r.param) || has(r.Param) ? strv(r.param, r.Param) : undefined
    const guide: string | undefined = has(r.guide) || has(r.Guide) ? strv(r.guide, r.Guide) : undefined
    const fix: string | undefined = has(r.fix) || has(r.Fix) ? strv(r.fix, r.Fix) : undefined
    const suggestions = (r.suggestions ?? r.Suggestions)?.map((x) => adaptSuggestion(x as Record<string, unknown>))
    const variations = (r.variations ?? r.Variations)?.map((x) => adaptVariation(x as Record<string, unknown>))
    return {
      code,
      severity,
      element,
      message,
      guide,
      param,
      fix,
      suggestions: suggestions?.length ? suggestions : undefined,
      variations: variations?.length ? variations : undefined,
    }
  })
  const blocking = Boolean(raw?.blocking ?? raw?.Blocking)
  const valid = Boolean(raw?.valid ?? raw?.Valid)
  const fieldErrors: FieldErrors = {}
  for (const it of issues) {
    for (const key of fieldKeysFor(it)) {
      if (!fieldErrors[key]) fieldErrors[key] = it.guide ?? it.message
    }
  }
  return { valid, blocking, issues, fieldErrors }
}

// ---- Ключ идентичности конфига (дедуп запросов) ----

export function configKey(config: ConfigForm): string {
  return JSON.stringify(config)
}

// ---- Применение готовых вариантов (A/B/C) и советников ----

// applySuggestion подставляет значения советника в форму (как в сторе).
export function applySuggestion(prev: ConfigForm, s: LiveSuggestion): ConfigForm {
  const next: ConfigForm = { ...prev, stepHeightMM: String(s.stepHeightMm) }
  if ((prev.flight === 'l_shape' || prev.flight === 'u_shape') && s.lowerStepCount) {
    next.lowerStepCountMM = String(s.lowerStepCount)
  }
  if (prev.flight === 'spiral' && s.outerRadiusMm && s.widthMm) {
    next.widthMM = String(s.widthMm)
    next.outerRadiusMM = String(s.outerRadiusMm)
  }
  return next
}

// applyVariation сливает конфиг варианта в форму. Пустые значения НЕ
// перезаписывают выбор пользователя (иначе форма становится невалидной,
// clobbering); шаг комфорта из варианта не применяем — он дефолтный и
// прокрутку пользователь не редактирует (см. Constructor). Подстраховка:
// свободное пространство перед первой ступенью обязательно (норма 1000–1200).
export function applyVariation(prev: ConfigForm, v: LiveVariation): ConfigForm {
  const merged = { ...prev } as unknown as Record<string, string>
  for (const [k, val] of Object.entries(v.config)) {
    if (typeof val !== 'string' || val.trim() === '') continue
    if (k === 'comfortStepMM') continue
    merged[k] = val
  }
  if (merged.approachSpaceMM?.trim() === '') merged.approachSpaceMM = '1000'
  return merged as unknown as ConfigForm
}

// ---- Дебаунсер с дедупом по конфигу ----

export interface LiveValidateTarget {
  key: string
  fetch: () => Promise<RawValidation>
  onChange: (value: LiveValidation | null) => void
}

/**
 * LiveValidator выполняет отложенный вызов :validate с дедупом:
 * — серия быстрых правок схлопывается в один запрос (debounce);
 * — возврат конфигурации к уже проверенному состоянию не повторяет запрос,
 *   переиспользуется последний результат того же ключа (dedup).
 * InstanceState не отменяет уже отправленные запросы (ответы приходят чужими
 * — перезаписывать пользователю может быть опасно), но различает их и
 * применяет только результат для актуального ключа.
 */
export class LiveValidator {
  private timer: number | null = null
  private resolved = new Map<string, LiveValidation>()

  schedule(target: LiveValidateTarget): void {
    if (this.timer !== null) {
      window.clearTimeout(this.timer)
      this.timer = null
    }
    const cached = this.resolved.get(target.key)
    if (cached) {
      // Конфиг уже проверен и не менялся — показываем сохранённый результат.
      target.onChange(cached)
      return
    }
    this.timer = window.setTimeout(() => {
      this.timer = null
      void this.perform(target)
    }, VALIDATE_DEBOUNCE_MS)
  }

  private async perform(target: LiveValidateTarget): Promise<void> {
    try {
      const raw = await target.fetch()
      const adapted = adaptValidation(raw)
      this.resolved.set(target.key, adapted)
      target.onChange(adapted)
    } catch {
      // Ошибка сети/лимита: живая валидация второстепенна — не трогаем UI,
      // следующий ввод повторит запрос. Кэш сбрасываем, чтобы не показать
      // устаревший результат для новой конфигурации.
      this.resolved.delete(target.key)
      target.onChange(null)
    }
  }

  /** Сброс кэша (после успешного расчёта), чтобы вернуться к прежним
   * значениям поле пере-валидировалось. */
  invalidate(key?: string): void {
    if (key !== undefined) {
      this.resolved.delete(key)
    } else {
      this.resolved.clear()
    }
  }

  cancel(): void {
    if (this.timer !== null) {
      window.clearTimeout(this.timer)
      this.timer = null
    }
    this.resolved.clear()
  }
}