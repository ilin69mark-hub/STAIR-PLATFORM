import { useEffect, useRef, useState } from 'react'
import type { ConfigForm } from '@shared/config'
import { defaultConfig, directionOptions, flightOptions, materialOptions, railingForSpiral, railingLabel, railingOptions, rulesFor, spiralDirectionOptions, toRequest, validateForm, type FieldErrors, type FieldRule } from '@shared/config'
import type { QuoteResult, QuoteSuggestion, Variation } from '@shared/types'
import {
  LiveValidator,
  applySuggestion as liveApplySuggestion,
  applyVariation as liveApplyVariation,
  configKey,
  type LiveIssue,
  type LiveSuggestion,
  type LiveValidation,
  type LiveVariation,
} from '@shared/liveValidate'
import { quoteApi } from '../api/store'
import { apiErrorMessage } from '../auth/errors'
import { elementLabel } from '@shared/validationText'
import { QuoteResult as QuoteResultView } from './QuoteResult'
import { OrderForm } from './OrderForm'
import { logAction } from '@shared/api/audit'

// Поля формы сгруппированы в смысловые блоки (секции). Шаг комфорта и высота
// ступени из формы убраны: высота ступени подставляется целевую (180 мм) и
// фактически рассчитывается геометрией, шаг комфорта — дефолт 630 (EDR-0001).
const fieldSections: Array<{ title: string; fields: Array<keyof ConfigForm> }> = [
  { title: 'Основные размеры', fields: ['widthMM', 'heightMM', 'flight', 'material'] },
  { title: 'Ступени', fields: ['stepThicknessMM', 'clearanceMM'] },
  { title: 'Перила', fields: ['railingHeightMM', 'railing', 'railingLower', 'railingLanding', 'railingUpper'] },
  { title: 'Поворот и площадка', fields: ['direction', 'landingWidthMM', 'landingDepthMM', 'lowerStepCountMM'] },
  { title: 'Помещение', fields: ['roomWidthMM', 'roomLengthMM', 'approachSpaceMM'] },
  { title: 'Спираль', fields: ['outerRadiusMM', 'spiralDirection'] },
]

const labels: Record<keyof ConfigForm, string> = {
  widthMM: 'Ширина марша (мм)',
  heightMM: 'Высота (мм)',
  flight: 'Тип лестницы',
  material: 'Материал',
  stepHeightMM: 'Высота ступени (мм)',
  stringerThicknessMM: 'Толщина косоура (мм)',
  stepThicknessMM: 'Толщина ступени (мм)',
  riser: 'Подступень',
  clearanceMM: 'Просвет (мм)',
  railingHeightMM: 'Высота перил (мм)',
  comfortStepMM: 'Шаг комфорта (мм)',
  landingWidthMM: 'Ширина площадки (мм)',
  landingDepthMM: 'Глубина площадки (мм)',
  roomWidthMM: 'Ширина помещения (мм)',
  roomLengthMM: 'Длина помещения (мм)',
  approachSpaceMM: 'Свободное пространство перед маршем (мм)',
  lowerStepCountMM: 'Нижних ступеней (шт)',
  outerRadiusMM: 'Радиус (мм)',
  railing: 'Перила',
  railingLower: 'Перила: первый марш',
  railingLanding: 'Перила: площадка',
  railingUpper: 'Перила: второй марш',
  direction: 'Направление поворота',
  spiralDirection: 'Направление спирали',
}

const hints: Partial<Record<keyof ConfigForm, string>> = {
  flight: 'Выберите тип марша',
  riser: 'Подступенок — вертикальная грань под ступенью. Его высота равна высоте ступени и рассчитывается автоматически.',
  clearanceMM: 'Рекомендуем ≥ 2000 мм',
  railingHeightMM: 'Рекомендуем 900–1100 мм',
  outerRadiusMM: 'Только для спирали',
  landingDepthMM: 'Глубина площадки вдоль нижнего марша (X в плане). Должна быть ≥ ширины марша.',
  roomWidthMM: 'Ширина помещения (X) — направление марша: длина забега + свободное место (1000–1200 мм). 0 — без проверки вписываемости.',
  roomLengthMM: 'Длина помещения (Y) — ширина марша. 0 — без проверки вписываемости.',
  approachSpaceMM: 'Свободная зона перед первой ступенью (норма 1000–1200 мм).',
}

// rangeHint — текст подсказки диапазона поля: «Мин X / макс Y мм», «Мин X мм»
// или «Макс X мм». Для материал-зависимых полей пересчитывается rulesFor.
function rangeHint(r: FieldRule): string {
  if (r.min !== undefined && r.max !== undefined) return `Мин ${r.min} / макс ${r.max} мм`
  if (r.min !== undefined) return `Мин ${r.min} мм`
  if (r.max !== undefined) return `Макс ${r.max} мм`
  return ''
}

const tooltips: Partial<Record<keyof ConfigForm, string>> = {
  clearanceMM:
    'Просвет — вертикальное расстояние от ступени до перекрытия. Рекомендуемый проход — от 2000 мм.',
  railing:
    'Сторона перил: встаньте у первой ступени и посмотрите вперёд по ходу подъёма. Слева от вас — левые перила, справа — правые.',
  railingLower:
    'Сторона перил на первом марше: встаньте у первой ступени и посмотрите вперёд по ходу подъёма. Слева — левые, справа — правые.',
  railingLanding:
    'Сторона перил на площадке: встаньте у первой ступени площадки и посмотрите вперёд по ходу подъёма. Слева — левые, справа — правые.',
  railingUpper:
    'Сторона перил на втором марше: встаньте у первой ступени марша и посмотрите вперёд по ходу подъёма. Слева — левые, справа — правые.',
}

// Пустая форма: поля не предзаполнены. Тип марша, скрытый косоур и целевая
// высота ступени (дефолтный таргет для геометрии) сохраняются.
const emptyConfig: ConfigForm = {
  ...defaultConfig,
  widthMM: '',
  heightMM: '',
  stepThicknessMM: '',
  clearanceMM: '',
  railingHeightMM: '',
  comfortStepMM: '',
  landingWidthMM: '',
  lowerStepCountMM: '',
  outerRadiusMM: '',
}

// Снапшот конфигурации, который пользователь реально видел: исходный марш и
// каждый применённый вариант остаются в галерее, чтобы можно было вернуться.
// id в пространстве `cfg-…` — такие карточки восстанавливаются целиком,
// тогда как бэкенд-варианты (A/B/C) сливаются в текущий конфиг.
const VERSION_ID_PREFIX = 'cfg-'

interface StairVersion {
  id: string
  config: ConfigForm
  title: string
  summary: string
}

const flightTitle: Record<ConfigForm['flight'], string> = {
  straight: 'Прямой марш',
  l_shape: 'L-образный марш',
  u_shape: 'П-образный марш',
  spiral: 'Спираль',
}

// Ключ содержимого конфига для дедупликации: сравниваем параметры марша,
// по которым варианты реально отличаются (тип, габариты, площадка/радиус).
function versionContentKey(cfg: Record<string, unknown>): string {
  return JSON.stringify([
    cfg.flight,
    cfg.material,
    cfg.heightMM,
    cfg.widthMM,
    cfg.stepThicknessMM,
    cfg.clearanceMM,
    cfg.railingHeightMM,
    cfg.stepHeightMM,
    cfg.landingWidthMM,
    cfg.landingDepthMM,
    cfg.lowerStepCountMM,
    cfg.outerRadiusMM,
    cfg.roomWidthMM,
    cfg.roomLengthMM,
  ])
}

function versionSummary(cfg: ConfigForm): string {
  const parts = [`Высота ${cfg.heightMM} мм`, `марш ${cfg.widthMM} мм`]
  if (cfg.flight === 'l_shape' || cfg.flight === 'u_shape') {
    parts.push(`площадка ${cfg.landingWidthMM}×${cfg.landingDepthMM}`)
  }
  if (cfg.flight === 'spiral') parts.push(`радиус ${cfg.outerRadiusMM}`)
  return parts.join(' · ')
}

// toVariation — снапшот как карточка галереи (полное восстановление).
function toVariation(v: StairVersion): Variation {
  return {
    id: v.id,
    title: v.title,
    description: '',
    summary: v.summary,
    fits: true,
    config: v.config as unknown as Record<string, string>,
  }
}

// Конструктор: параметры лестницы → предварительный расчёт (анонимно).
export function Constructor() {
  const [config, setConfig] = useState<ConfigForm>(emptyConfig)
  // Ошибки не показываем до первого взаимодействия пользователя.
  const [errors, setErrors] = useState<FieldErrors>({})
  const [touched, setTouched] = useState(false)
  const [quote, setQuote] = useState<QuoteResult | null>(null)
  const [request, setRequest] = useState<Record<string, unknown> | null>(null)
  const [status, setStatus] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  // Живая валидация при вводе (S-P5): серверные блокировки, которые локально
  // не видны (R>W спирали, угол/проступь по норме, вписываемость в помещение).
  // Баннер + подсветка полей; blocking-issue несут готовые варианты «Применить».
  const [liveBlocking, setLiveBlocking] = useState(false)
  const [liveIssues, setLiveIssues] = useState<LiveIssue[]>([])
  const [liveFieldErrors, setLiveFieldErrors] = useState<FieldErrors>({})
  const liveValidator = useRef(new LiveValidator())
  const clearLive = useRef(false)
  const applyLive = (v: LiveValidation | null) => {
    if (clearLive.current) {
      clearLive.current = false
      return
    }
    const withIssues = !!v && v.blocking && v.issues.length > 0
    setLiveBlocking(withIssues)
    setLiveIssues(withIssues ? v!.issues : [])
    setLiveFieldErrors(withIssues ? v!.fieldErrors : {})
  }

  // Вариации (A/B/C) от последнего блокирующего ответа + снапшоты «моих
  // вариантов» (исходный марш и каждый применённый вариант). Галерея склеивает
  // их: свои конфиги никогда не исчезают, к ним можно вернуться.
  const [versions, setVersions] = useState<StairVersion[]>([])
  const [variations, setVariations] = useState<Variation[] | null>(null)
  const [activeVariationId, setActiveVariationId] = useState<string | null>(null)
  const versionSeq = useRef(0)
  // Зеркало версий в ref: дедупликация не зависит от устаревшего замыкания
  // (pushVersion может вызываться после `await` в calculate).
  const versionsRef = useRef<StairVersion[]>([])

  // pushVersion добавляет снапшот конфига (без дублей по содержимому)
  // и возвращает его id — активный/выбранный вариант галереи.
  const pushVersion = (cfg: ConfigForm): string => {
    const key = versionContentKey(cfg as unknown as Record<string, unknown>)
    const existing = versionsRef.current.find(
      (p) => versionContentKey(p.config as unknown as Record<string, unknown>) === key,
    )
    if (existing) return existing.id
    versionSeq.current += 1
    const id = `${VERSION_ID_PREFIX}${versionSeq.current}`
    const ver: StairVersion = {
      id,
      config: cfg,
      title: flightTitle[cfg.flight],
      summary: versionSummary(cfg),
    }
    versionsRef.current = [...versionsRef.current, ver]
    setVersions(versionsRef.current)
    return id
  }

  // Предупреждение при расчёте без габаритов помещения + подсветка комнатных
  // полей, если пользователь выбрал «внести данные площади».
  const [roomPrompt, setRoomPrompt] = useState(false)
  const [roomHighlight, setRoomHighlight] = useState(false)
  const [skipRoomPrompt, setSkipRoomPrompt] = useState(false)
  const roomWidthRef = useRef<HTMLInputElement | null>(null)
  const roomLengthRef = useRef<HTMLInputElement | null>(null)

  const visible = (k: keyof ConfigForm): boolean => {
    if (k === 'stringerThicknessMM') return false // скрыт: единый косоур по умолчанию
    if (k === 'stepHeightMM' || k === 'comfortStepMM') return false // скрыты: рассчитываются автоматически
    if ((k === 'landingWidthMM' || k === 'landingDepthMM' || k === 'lowerStepCountMM') &&
      config.flight !== 'l_shape' && config.flight !== 'u_shape') {
      return false
    }
    // approachSpaceMM показывается для всех типов марша (EDR-0023).
    if (k === 'outerRadiusMM' && config.flight !== 'spiral') return false
    // Перила: прямой марш — один выбор, марши с площадкой — по сегментам,
    // спираль — авто (сторона от направления закрутки), свой блок ниже.
    if (k === 'railing' && config.flight !== 'straight') return false
    if ((k === 'railingLower' || k === 'railingLanding' || k === 'railingUpper' || k === 'direction') &&
      config.flight !== 'l_shape' && config.flight !== 'u_shape') {
      return false
    }
    if (k === 'spiralDirection' && config.flight !== 'spiral') return false
    return true
  }

  // selectOptions — варианты выпадающих списков формы по ключу поля.
  const selectOptions = (k: keyof ConfigForm) => {
    switch (k) {
      case 'flight':
        return flightOptions
      case 'material':
        return materialOptions
      case 'railing':
      case 'railingLower':
      case 'railingLanding':
      case 'railingUpper':
        return railingOptions
      case 'direction':
        return directionOptions
      case 'spiralDirection':
        return spiralDirectionOptions
      default:
        return null
    }
  }

  const configChangeTimer = useRef<number | null>(null)

  // Очистка debounce таймеров (аудит и живая валидация) при unmount.
  useEffect(() => {
    return () => {
      if (configChangeTimer.current) window.clearTimeout(configChangeTimer.current)
      liveValidator.current.cancel()
    }
  }, [])

  const update = (k: keyof ConfigForm, v: string) => {
    const next = { ...config, [k]: v }
    setConfig(next)
    setTouched(true)
    const errs = validateForm(next)
    setErrors(errs)
    if (next.roomWidthMM.trim() !== '' && next.roomLengthMM.trim() !== '') {
      setRoomHighlight(false)
    }
    // Живая валидация при вводе (S-P5): дебаунс + дедуп по конфигу.
    // Локальные ошибки формы уже подсвечены — сервер не дёргаем.
    if (Object.keys(errs).length === 0) {
      clearLive.current = false
      liveValidator.current.schedule({
        key: configKey(next),
        fetch: () => quoteApi.validate(toRequest(next)),
        onChange: applyLive,
      })
    }
    // Аудит изменения поля (debounce 600 мс, best-effort).
    if (configChangeTimer.current) window.clearTimeout(configChangeTimer.current)
    configChangeTimer.current = window.setTimeout(() => {
      logAction({
        action: 'stair.config_changed',
        resource_type: 'stair',
        detail: JSON.stringify({ field: k }),
      })
    }, 600)
  }

  // Подсказка поля: ширина/высота и толщины считаются по материалу
  // (rulesFor → materialLimits), остальные поля — статический текст.
  // Для ширины/высоты показываем материал-зависимый максимум («Макс … мм»),
  // для толщины ступени — полный диапазон материала.
  const hintOf = (k: keyof ConfigForm): string | undefined => {
    if (k === 'widthMM' || k === 'heightMM') {
      return rangeHint({ max: rulesFor(k, config.material).max })
    }
    if (k === 'stepThicknessMM') {
      return rangeHint(rulesFor(k, config.material))
    }
    return hints[k]
  }

  const setRiser = (v: boolean) => {
    const next = { ...config, riser: v }
    setConfig(next)
    setErrors(validateForm(next))
  }

  const roomError = (k: keyof ConfigForm): string | undefined => {
    if (!roomHighlight) return undefined
    if (k === 'roomWidthMM' && config.roomWidthMM.trim() === '') return 'Укажите ширину помещения'
    if (k === 'roomLengthMM' && config.roomLengthMM.trim() === '') return 'Укажите длину помещения'
    return undefined
  }

  const calculate = async (cfg: ConfigForm = config) => {
    const errs = validateForm(cfg)
    setErrors(errs)
    if (Object.keys(errs).length > 0) {
      setStatus('Исправьте поля формы перед расчётом')
      return
    }
    setBusy(true)
    setStatus(null)
    setQuote(null)
    setRequest(null)
    try {
      const body = toRequest(cfg)
      const res = await quoteApi.calculate(body)
      setQuote(res)
      setRequest(body)
      // Полный расчёт выполнен: его validation и есть актуальная картина —
      // живой баннер и подсветка больше не нужны.
      liveValidator.current.invalidate(configKey(cfg))
      applyLive({ valid: true, blocking: false, issues: [], fieldErrors: {} })
      // Новый блокирующий ответ с вариациями заменяет список альтернатив.
      // Текущий (заблокированный) конфиг якорим как снапшот — исходный марш
      // остаётся в галерее и к нему можно вернуться.
      const firstVar = res.validation.issues?.find(
        (i) => i.variations && i.variations.length > 0,
      )
      if (firstVar && firstVar.variations) {
        setVariations(firstVar.variations)
        setActiveVariationId(pushVersion(cfg))
      }
    } catch (e) {
      setStatus(apiErrorMessage(e, 'Не удалось выполнить расчёт'))
    } finally {
      setBusy(false)
    }
  }

  // Ручной запуск расчёта: сначала проверяем габариты помещения и, если
  // пользователь не ввёл ширину/длину, предлагаем заполнить либо продолжить
  // без проверки вписываемости.
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (Object.keys(validateForm(config)).length > 0) {
      void calculate()
      return
    }
    const hasRoom = config.roomWidthMM.trim() !== '' && config.roomLengthMM.trim() !== ''
    if (!hasRoom && !skipRoomPrompt) {
      setRoomPrompt(true)
      return
    }
    void calculate()
  }

  const continueWithoutArea = () => {
    setRoomPrompt(false)
    setSkipRoomPrompt(true)
    void calculate()
  }

  const enterAreaData = () => {
    setRoomPrompt(false)
    setRoomHighlight(true)
    // Курсор сразу на первое пустое «красное» поле (ширина/длина помещения).
    const target =
      config.roomWidthMM.trim() === '' ? roomWidthRef.current : roomLengthRef.current
    requestAnimationFrame(() => {
      target?.focus()
      if (target && typeof target.scrollIntoView === 'function') {
        target.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }
    })
  }

  // Применение готового варианта советника: подставляем значения в форму
  // и сразу пересчитываем — блокировка снимается.
  const applySuggestion = (s: QuoteSuggestion) => {
    const next: ConfigForm = { ...config, stepHeightMM: String(s.step_height_mm) }
    if ((config.flight === 'l_shape' || config.flight === 'u_shape') && s.lower_step_count) {
      next.lowerStepCountMM = String(s.lower_step_count)
    }
    if (config.flight === 'spiral' && s.outer_radius_mm && s.width_mm) {
      next.widthMM = String(s.width_mm)
      next.outerRadiusMM = String(s.outer_radius_mm)
    }
    setConfig(next)
    void calculate(next)
    logAction({
      action: 'stair.suggestion_applied',
      resource_type: 'stair',
      detail: JSON.stringify({ step_height_mm: s.step_height_mm, step_count: s.step_count }),
    })
  }

  // Применение вариации (A/B/C, напр. невписываемость в помещение): сливаем
  // её конфиг в форму и пересчитываем — блокировка снимается.
  // Вариации от бэкенда содержат все поля ConfigForm, в т.ч. пустые
  // (railing/direction/сегменты перил и т.п. не заданы для данного варианта).
  // Пустые значения НЕ перезаписывают выбор пользователя, иначе форма
  // оказывается невалидной и пересчёт падает (clobbering).
  // Шаг комфорта из варианта НЕ применяем: он всегда дефолтный (630 мм) —
  // прокрутку пользователь не видит. Целевая высота ступени подставляется
  // скрытым полем (её не видно, но вариант воспроизводит обещанную геометрию).
  const applyVariation = (v: Variation) => {
    const merged = { ...config } as unknown as Record<string, string>
    for (const [k, val] of Object.entries(v.config)) {
      if (val === '') continue
      if (k === 'comfortStepMM') continue
      merged[k] = val
    }
    const next = merged as unknown as ConfigForm
    // Свободное пространство перед первой ступенью обязательно для всех
    // типов марша (норма 1000–1200 мм, EDR-0023); пустое/отсутствующее
    // значение — 1000 мм по умолчанию.
    if ((next.approachSpaceMM ?? '').trim() === '') {
      next.approachSpaceMM = '1000'
    }
    setConfig(next)
    // Применённый вариант тоже становится снапшотом — галерея хранит и его.
    setActiveVariationId(pushVersion(next))
    void calculate(next)
    logAction({
      action: 'stair.variation_applied',
      resource_type: 'stair',
      detail: JSON.stringify({ id: v.id, title: v.title }),
    })
  }

  // «Применить» из баннера живой валидации: применяем предложенный вариант
  // советника/вариацию к форме и сразу пересчитываем (блокировка снимается).
  const applyLiveSuggestion = (si: LiveSuggestion) => {
    const next = liveApplySuggestion(config, si)
    setConfig(next)
    setErrors(validateForm(next))
    liveValidator.current.invalidate(configKey(next))
    void calculate(next)
    logAction({
      action: 'stair.live_suggestion_applied',
      resource_type: 'stair',
      detail: JSON.stringify({ step_height_mm: si.stepHeightMm, step_count: si.stepCount }),
    })
  }
  const applyLiveVariation = (v: LiveVariation) => {
    const next = liveApplyVariation(config, v)
    setConfig(next)
    setErrors(validateForm(next))
    liveValidator.current.invalidate(configKey(next))
    void calculate(next)
    logAction({
      action: 'stair.live_variation_applied',
      resource_type: 'stair',
      detail: JSON.stringify({ id: v.id, title: v.title }),
    })
  }

  // Клик по карточке галереи: свои снапшоты (cfg-…) восстанавливаем целиком,
  // бэкенд-варианты сливаем в текущий конфиг (applyVariation).
  const applyGalleryVariation = (v: Variation) => {
    if (v.id.startsWith(VERSION_ID_PREFIX)) {
      const ver = versionsRef.current.find((x) => x.id === v.id)
      if (!ver) return
      setConfig(ver.config)
      setActiveVariationId(ver.id)
      void calculate(ver.config)
      logAction({
        action: 'stair.variation_applied',
        resource_type: 'stair',
        detail: JSON.stringify({ id: ver.id, title: ver.title, method: 'restore' }),
      })
      return
    }
    applyVariation(v)
  }

  const reset = () => {
    setConfig(emptyConfig)
    setErrors(validateForm(emptyConfig))
    setQuote(null)
    setRequest(null)
    setStatus(null)
    setVersions([])
    versionsRef.current = []
    versionSeq.current = 0
    setVariations(null)
    setActiveVariationId(null)
    setRoomPrompt(false)
    setRoomHighlight(false)
    setSkipRoomPrompt(false)
    liveValidator.current.cancel()
    applyLive(null)
  }

  const renderRoomFieldError = (k: keyof ConfigForm): React.ReactNode =>
    roomError(k) ? <span className="error">{roomError(k)}</span> : null

  // Галерея: снапшоты пользователя + свежие альтернативы бэкенда без дублей
  // по содержимому (совпавшая с уже выбранным конфигом альтернатива скрыта).
  // Спасение расчёта: первый вариант БЭКЕНДА, отличный от текущего
  // (заблокированного) конфига. galleryVariations[0] не годится — там первым
  // стоит снапшот самого заблокированного конфига (pushVersion).
  const rescueVariation = (variations ?? []).find(
    (alt) =>
      !versions.some(
        (ver) =>
          versionContentKey(alt.config) ===
          versionContentKey(ver.config as unknown as Record<string, unknown>),
      ),
  )

  const galleryVariations: Variation[] = [
    ...versions.map(toVariation),
    ...(variations ?? []).filter(
      (alt) =>
        !versions.some(
          (ver) =>
            versionContentKey(alt.config) ===
            versionContentKey(ver.config as unknown as Record<string, unknown>),
        ),
    ),
  ]

  return (
    <div>
      <section className="panel">
        <h2>Конструктор лестницы</h2>
        <p className="sub">Задайте параметры — мы рассчитаем геометрию и предварительную цену.</p>
        <form onSubmit={handleSubmit}>
          <div className="form-sections">
            {fieldSections.map((section) => (
              <section className="form-section" key={section.title}>
                <h3 className="form-section__title">{section.title}</h3>
                <div className="form-grid">
                  {section.fields.map((k) =>
                    k === 'stepHeightMM' || k === 'comfortStepMM' ? null : (
                      <div className="field" key={k} hidden={!visible(k)}>
                        <FieldLabel label={labels[k]} tooltip={tooltips[k]} htmlFor={`cfg-${k}`} />
                        {selectOptions(k) ? (
                          <select
                            id={`cfg-${k}`}
                            value={config[k] as string}
                            onChange={(e) => update(k, e.target.value)}
                          >
                            {selectOptions(k)!.map((o) => (
                              <option key={o.value} value={o.value}>
                                {o.label}
                              </option>
                            ))}
                          </select>
                        ) : (
                          <input
                            id={`cfg-${k}`}
                            ref={
                              k === 'roomWidthMM'
                                ? roomWidthRef
                                : k === 'roomLengthMM'
                                  ? roomLengthRef
                                  : undefined
                            }
                            type="text"
                            inputMode="decimal"
                            className={
                              (touched && errors[k]) ||
                              liveFieldErrors[k] != null ||
                              (roomHighlight &&
                                (k === 'roomWidthMM' || k === 'roomLengthMM') &&
                                (config[k] as string).trim() === '')
                                ? 'field-invalid'
                                : undefined
                            }
                            value={config[k] as string}
                            onChange={(e) => update(k, e.target.value)}
                          />
                        )}
                        {hintOf(k) && <span className="sub">{hintOf(k)}</span>}
                        {touched && errors[k] && <span className="error">{errors[k]}</span>}
                        {touched && !errors[k] && liveFieldErrors[k] && (
                          <span className="error">{liveFieldErrors[k]}</span>
                        )}
                        {renderRoomFieldError(k)}
                      </div>
                    ),
                  )}
                  {section.title === 'Ступени' && (
                    <div className="field" hidden={config.flight === 'spiral'}>
                      <FieldLabel label={labels.riser} htmlFor="cfg-riser" />
                      <label className="checkbox">
                        <input
                          id="cfg-riser"
                          type="checkbox"
                          checked={config.riser}
                          onChange={(e) => setRiser(e.target.checked)}
                        />
                        <span>{config.riser ? 'Да' : 'Нет'}</span>
                      </label>
                      {hints.riser && <span className="sub">{hints.riser}</span>}
                    </div>
                  )}
                  {section.title === 'Перила' && config.flight === 'spiral' && (
                    <div className="field">
                      <FieldLabel label={labels.railing} tooltip={tooltips.railing} htmlFor="cfg-railing-auto" />
                      {/* Спираль: перила всегда с одной стороны, сторона автоматически
                          от направления закрутки (CONF-SPIRAL-RAILING). */}
                      <input
                        id="cfg-railing-auto"
                        type="text"
                        readOnly
                        value={railingLabel(railingForSpiral(config.spiralDirection))}
                      />
                      <span className="sub">Авто: по направлению спирали</span>
                    </div>
                  )}
                </div>
              </section>
            ))}
          </div>

          {roomPrompt && (
            <div className="alert alert--warn room-prompt" role="alert">
              <div className="room-prompt__text">
                Корректный расчёт под ваше помещение возможен только с шириной и длиной
                помещения — укажите их, чтобы мы проверили, поместится ли лестница.
              </div>
              <div className="room-prompt__actions">
                <button className="sp-btn sp-btn--primary" type="button" onClick={enterAreaData}>
                  Внести данные площади
                </button>
                <button className="sp-btn" type="button" onClick={continueWithoutArea}>
                  Продолжить без площади
                </button>
              </div>
            </div>
          )}
          {liveBlocking && liveIssues.length > 0 && (
            <div className="alert alert--warn" role="alert">
              <div className="room-prompt__text">
                <div className="live-issues">
                  {liveIssues.map((it, idx) => (
                    <div className="live-issue" key={idx}>
                      <span>
                        <strong>{it.param ? `${elementLabel(it.param)}: ` : ''}{it.guide ?? it.message}</strong>
                        {it.fix ? ` (${it.fix})` : ''}
                      </span>
                      {it.suggestions && it.suggestions.length > 0 && (
                        <button
                          type="button"
                          className="sp-btn sp-btn--primary live-issue__apply"
                          onClick={() => applyLiveSuggestion(it.suggestions![0])}
                        >
                          Применить: {it.suggestions[0].stepCount} ступ. · h {it.suggestions[0].stepHeightMm.toFixed(1)}
                        </button>
                      )}
                      {it.variations && it.variations.length > 0 && (
                        <button
                          type="button"
                          className="sp-btn live-issue__apply"
                          onClick={() => { const v = it.variations![0]; applyLiveVariation(v) }}
                        >
                          Применить вариант A
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}
          {status && <div className="alert alert--error" role="alert">{status}</div>}
          <div className="actions">
            <button className="sp-btn sp-btn--primary" type="submit" disabled={busy}>
              {busy ? 'Расчёт…' : 'Рассчитать'}
            </button>
            <button className="sp-btn" type="button" onClick={reset}>
              Сбросить
            </button>
          </div>
        </form>
      </section>

      {quote && (
        <>
          {quote.validation.blocking && rescueVariation && (
            <div className="alert alert--warn rescue" role="alert">
              <div className="rescue__text">
                <strong>Такой расчёт невозможен.</strong> Ближайший рабочий вариант
                подходит под ваши габариты и открывает 3D с ценой.
              </div>
              <button
                type="button"
                className="sp-btn sp-btn--primary"
                onClick={() => applyGalleryVariation(rescueVariation)}
              >
                Спасти расчёт
              </button>
            </div>
          )}
          <QuoteResultView
            quote={quote}
            onApplySuggestion={applySuggestion}
            onApplyVariation={applyGalleryVariation}
            variations={galleryVariations.length > 0 ? galleryVariations : undefined}
            activeVariationId={activeVariationId}
            material={config.material}
            approachSpaceMM={config.approachSpaceMM}
            heightMM={Number(config.heightMM) || undefined}
          />
          {!quote.validation.blocking && quote.pricing && request && (
            <OrderForm
              quote={quote}
              config={request}
              onCreated={() => setStatus('Заказ отправлен. Следите за статусом в кабинете.')}
            />
          )}
        </>
      )}
    </div>
  )
}

// FieldLabel — подпись поля с опциональным знаком справки «?» и тултипом по наведению.
function FieldLabel({ label, tooltip, htmlFor }: { label: string; tooltip?: string; htmlFor: string }) {
  return (
    <div className="field-label">
      <label htmlFor={htmlFor}>{label}</label>
      {tooltip && (
        <span className="field-help" tabIndex={0}>
          ?
          <span className="field-help-tip" role="tooltip">
            {tooltip}
          </span>
        </span>
      )}
    </div>
  )
}