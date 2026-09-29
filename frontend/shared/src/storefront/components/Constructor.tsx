import { useEffect, useRef, useState } from 'react'
import type { ConfigForm } from '@shared/config'
import { defaultConfig, directionOptions, flightOptions, materialOptions, fitThicknessMM, fieldRulesFor, railingOptions, rulesFor, spiralDirectionOptions, toRequest, turnKindOptions, validateForm, REQUIRED_VALUE_ERROR, type FieldErrors, type FieldRule } from '@shared/config'
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
import { EVENTS, track } from '@shared/analytics'
import { useConsentGranted } from '@shared/consentReact'
import { apiErrorCode, apiErrorMessage } from '../auth/errors'
import { elementLabel } from '@shared/validationText'
import { QuoteResult as QuoteResultView, Stage3D } from './QuoteResult'
import { solverOf } from './quoteView'
import { Slider, Segmented, SwatchGroup, ProductTabs, type ProductTab } from '@shared/components/CalcControls'
import { Accordion, type AccordionSection } from '@shared/components/Accordion'
import { FINISHES } from '@shared/viewer/materials'
import { OrderForm } from './OrderForm'
import { logAction } from '@shared/api/audit'

// Поля формы сгруппированы в смысловые блоки (секции). Шаг комфорта и высота
// ступени из формы убраны: высота ступени подставляется целевую (180 мм) и
// фактически рассчитывается геометрией, шаг комфорта — дефолт 630 (EDR-0001).

// Изделия конструктора. Переключение меняет набор параметров и материал, а не
// только набор вкладок: у металлокаркаса и деревянной лестницы разные
// ограничения по толщинам (см. materialLimits в config.ts), и подставлять
// дерево в «металлокаркас» нельзя — расчёт и цена поедут.
const PRODUCTS: ProductTab[] = [
  { id: 'metal', label: 'Металлокаркас' },
  { id: 'wood', label: 'Деревянные' },
]

// Материалы каркаса по изделиям. Полный список (со всеми кодами) живёт в
// config.materialOptions — он нужен админке. Здесь только то, что осмысленно
// в конкретной конструкции: в металлокаркасе каркас металлический, в
// деревянной лестнице — деревянный.
//
// Это не косметика. Пределы по толщине разные (materialLimits в config.ts),
// и косоур 6 мм из стали в деревянной лестнице означал бы 20 мм дуба: иной
// вес, иная цена, иной раскрой.
// Калькулятор витрины предлагает ТОЛЬКО сталь: в ассортименте материалов
// MFG-0005 других металлов нет, и вторая ось выбора — порода ступеней
// (Деревянные изделия) либо металл/дерево у ступеней металлокаркаса.
//
// Список здесь, а не в config.materialOptions, потому что тот общий:
// админка и showcase должны видеть все материалы, которые мы в состоянии
// изготовить.
const METAL_MATERIALS = ['STEEL-S235'] as const
const WOOD_MATERIALS = ['WOOD-OAK', 'WOOD-WALNUT', 'WOOD-ASH', 'WOOD-SOFT'] as const

// Стартовый материал изделия. Металлокаркас — стальной каркас с дубовыми
// ступенями (самая частая комплектация), деревянная лестница — дуб целиком.
const PRODUCT_DEFAULT: Record<string, { frame: string; tread: string }> = {
  metal: { frame: 'STEEL-S235', tread: 'WOOD-OAK' },
  wood: { frame: 'WOOD-OAK', tread: 'WOOD-OAK' },
}

// Цвет покрытия берётся из FINISHES (viewer/materials.ts) — источник истины
// по видам отделки один на витрину и на админку. Менять палитру здесь нельзя:
// finishFor() не найдёт финиш и молча отдаст «без финиша».
const FINISH_SWATCH: Record<string, { color: string }> = {
  raw: { color: '#9aa3ad' },
  natural: { color: '#c8ccd2' },
  black: { color: '#24262a' },
  white: { color: '#e8e8e6' },
  oil: { color: '#c9a678' },
  matte: { color: '#c9a678' },
  toned: { color: '#9a7448' },
}
const labels: Record<keyof ConfigForm, string> = {
  widthMM: 'Ширина марша (мм)',
  heightMM: 'Высота (мм)',
  flight: 'Тип лестницы',
  material: 'Материал каркаса',
  treadMaterial: 'Материал ступеней',
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
  approachSpaceMM: 'Свободное место перед маршем',
  lowerStepCountMM: 'Нижних ступеней (шт)',
  outerRadiusMM: 'Радиус (мм)',
  railing: 'Перила',
  railingLower: 'Перила: первый марш',
  railingLanding: 'Перила: площадка',
  railingUpper: 'Перила: второй марш',
  direction: 'Направление поворота',
  spiralDirection: 'Направление спирали',
  turnKind: 'Поворот марша',
  winderCountMM: 'Поворотных ступеней (шт)',
}

// Подсказки под ползунком — только нормативы, который не видно из подписи.
// Подсказки-инструкции убраны намеренно: «Выберите тип марша» висела под
// переключателем, где «Прямой марш» уже выбран, и «Задайте параметры…»
// повторяла то, что и так видно по пустым полям и кнопке «Рассчитать».
const hints: Partial<Record<keyof ConfigForm, string>> = {
  clearanceMM: 'Рекомендуем ≥ 2000 мм',
  railingHeightMM: 'Рекомендуем 900–1100 мм',
}

const tooltips: Partial<Record<keyof ConfigForm, string>> = {
  approachSpaceMM: 'Свободная зона перед первой ступенью (норма 1000–1200 мм).',
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
  // Металлокаркас с деревянными ступенями — стартовая комплектация
  // витрины: стальной косоур, дубовая проступь. Толщина ступени обязана быть
  // в диапазоне ДЕРЕВА (20–60 мм), поэтому 6 мм из стальных пределов здесь
  // не подходит.
  treadMaterial: 'WOOD-OAK',
  stepThicknessMM: '40',
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
    cfg.turnKind,
    cfg.winderCountMM,
    cfg.outerRadiusMM,
    cfg.roomWidthMM,
    cfg.roomLengthMM,
  ])
}

function versionSummary(cfg: ConfigForm): string {
  const parts = [`Высота ${cfg.heightMM} мм`, `марш ${cfg.widthMM} мм`]
  if (cfg.flight === 'l_shape' || cfg.flight === 'u_shape') {
    if (cfg.turnKind === 'winder') {
      parts.push(`поворот ${cfg.winderCountMM || '—'} ступ.`)
    } else {
      parts.push(`площадка ${cfg.landingWidthMM}×${cfg.landingDepthMM}`)
    }
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
  // Тип изделия: металлокаркас / деревянные. Сейчас доступен только
  // металлокаркас — переключатель виден, чтобы набор изделий был понятен, но
  // деревянный помечен как «скоро» и не принимает кликов.
  const [product, setProduct] = useState('metal')
  // Цвета покрытия каркаса и ступеней. Локальное состояние витрины, НЕ часть
  // конфига: finishId не уходит в расчёт и в цену, он меняет только вид в 3D.
  // Два отдельных значения, потому что металлокаркас с чёрным каркасом и
  // дубовой ступенью — обычная комплектация, а не исключение.
  const [finishId, setFinishId] = useState('raw')
  const [treadFinishId, setTreadFinish] = useState('oil')
  // Отделка ПОДСТУПЕНКОВ. Материал подступенка производный — он следует за
  // материалом ступеней, — но отделку для дерева можно выбрать свою, поэтому
  // это отдельное состояние. Для металла оно игнорируется (см. riserFinish
  // ниже): подступенок того же цвета, что и ступени.
  const [riserFinishId, setRiserFinish] = useState('oil')
  // Ошибки не показываем до первого взаимодействия пользователя.
  const [errors, setErrors] = useState<FieldErrors>({})
  // Помечено КАЖДОЕ поле отдельно, а не форма целиком. Флаг на всю форму
  // показывал ошибки чужих полей: стоило сдвинуть «Высоту», и под пустым
  // «Ширина марша» появлялось «Укажите значение» — про поле, которого
  // пользователь не касался. Текст обязательного поля теперь вовсе не
  // показывается (REQUIRED_VALUE_ERROR), поле только краснеет, но остальные
  // ошибки («Не более 3000», «Введите число») остаются привязаны к своему
  // полю, а не ко всей форме.
  const [touched, setTouched] = useState<Partial<Record<keyof ConfigForm, true>>>({})
  const markTouched = (k: keyof ConfigForm) =>
    setTouched((t) => (t[k] ? t : { ...t, [k]: true }))
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

  // Открытие конструктора — верх воронки витрины: после лендинга и входа в
  // кабинет это главный переход.
  //
  // Зависимость от согласия обязательна: баннер висит поверх конструктора, и
  // большинство нажимает «Разрешить» уже здесь. Без неё верх воронки
  // терялся бы ровно у этих людей (событие до согласия не отправляется), и
  // отчёт показывал бы неверный вход. Событие при этом одно: ref не даёт
  // продублировать его ни при смене согласия, ни в StrictMode.
  const consentGranted = useConsentGranted()
  const openedReported = useRef(false)
  useEffect(() => {
    if (!consentGranted || openedReported.current) return
    openedReported.current = true
    track(EVENTS.constructorOpen)
  }, [consentGranted])
  // Уже отмеченные «живые» затыки. Живая валидация срабатывает на каждом
  //debounce-вводе, и без дедупликации одно и то же поле дало бы десятки
  // одинаковых событий — отчёт превратился бы в шум, а вопрос «где затыкают»
  // перестал бы читаться. Поэтому событие уходит один раз на пару
  // «поле + код» до ближайшего успеха.
  const reportedBlockers = useRef<Set<string>>(new Set())

  const applyLive = (v: LiveValidation | null) => {
    if (clearLive.current) {
      clearLive.current = false
      return
    }
    const withIssues = !!v && v.blocking && v.issues.length > 0
    setLiveBlocking(withIssues)
    setLiveIssues(withIssues ? v!.issues : [])
    setLiveFieldErrors(withIssues ? v!.fieldErrors : {})
    if (!withIssues) {
      reportedBlockers.current.clear()
      return
    }
    for (const issue of v!.issues) {
      const key = `${issue.element ?? ''}|${issue.code}`
      if (reportedBlockers.current.has(key)) continue
      reportedBlockers.current.add(key)
      track(EVENTS.blockerApi, { reason: issue.code, element: issue.element ?? '' })
    }
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
  // Вопрос «укажите размеры помещения» — самая частая точка, на которой
  // посетитель отваливается: нажал «Рассчитать» и ушёл, не ответив. По
  // отчёту без этого события он выглядит как «нажал и исчез», а на самом деле
  // вопрос ему даже не успели разобрать. Событие одно за показ вопроса.
  const roomPromptReported = useRef(false)
  useEffect(() => {
    if (!roomPrompt || roomPromptReported.current) return
    roomPromptReported.current = true
    track(EVENTS.blockerField, { reason: 'room_prompt', where: 'submit' })
  }, [roomPrompt])
  const [roomHighlight, setRoomHighlight] = useState(false)
  const [skipRoomPrompt, setSkipRoomPrompt] = useState(false)
  const roomWidthRef = useRef<HTMLInputElement | null>(null)
  const roomLengthRef = useRef<HTMLInputElement | null>(null)

  const visible = (k: keyof ConfigForm): boolean => {
    if (k === 'stringerThicknessMM') return false // скрыт: единый косоур по умолчанию
    if (k === 'stepHeightMM' || k === 'comfortStepMM') return false // скрыты: рассчитываются автоматически
    if ((k === 'landingWidthMM' || k === 'landingDepthMM' || k === 'lowerStepCountMM' ||
      k === 'turnKind' || k === 'winderCountMM') &&
      config.flight !== 'l_shape' && config.flight !== 'u_shape') {
      return false
    }
    // DOM-001: число поворотных ступеней — только при turnKind='winder'.
    if (k === 'winderCountMM' && config.turnKind !== 'winder') return false
    // При turnKind='winder' площадка не нужна: поворот выполняют ступени.
    if (config.turnKind === 'winder' &&
      (k === 'landingWidthMM' || k === 'landingDepthMM')) {
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
      case 'turnKind':
        return turnKindOptions
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
    // Смена материала подтягивает толщину ступени в допуск нового материала:
    // дерево (мин. 20 мм) иначе сразу уходит в 422 «не поддерживает толщину».
    if (k === 'material') {
      next.stepThicknessMM = fitThicknessMM(v, next.stepThicknessMM)
    }
    setConfig(next)
    markTouched(k)
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
  // Поле подсвечиваем красным в трёх случаях: пользователь уже трогал
  // форму и поле не прошло проверку, пришёл live-ответ сервера, либо
  // нажата «Внести данные площади» и комнатное поле ещё пустое.
  // Условие обязано быть общим и для ползунка (prop invalid), и для
  // обычного input (className) — иначе комнатные поля не краснеют.
  const isInvalid = (k: keyof ConfigForm) =>
    !!(touched[k] && errors[k]) ||
    liveFieldErrors[k] != null ||
    !!(roomHighlight && (k === 'roomWidthMM' || k === 'roomLengthMM') && (config[k] as string).trim() === '')

  // Под ползунком остаётся только полезная подсказка (рекомендация), а не
  // пределы: минимум и максимум ползунок показывает шкалой .slider__scale.
  // Дубли «Мин 3 / макс 8 мм» и «Макс 6000 мм» съедали по строке на каждое
  // поле и ровно они вводили в заблуждение у дубовых ступеней, где пределы
  // считаются по материалу ступеней, а не каркаса.
  const hintOf = (k: keyof ConfigForm): string | undefined => hints[k]

  const setRiser = (v: boolean) => {
    const next = { ...config, riser: v }
    setConfig(next)
    setErrors(validateForm(next))
  }

  const calculate = async (cfg: ConfigForm = config) => {
    const errs = validateForm(cfg)
    setErrors(errs)
    if (Object.keys(errs).length > 0) {
      setStatus('Исправьте поля формы перед расчётом')
      // Главный ответ на вопрос «где затык»: человек нажал «Рассчитать»,
      // но не смог. Имена полей (не значения!) уходят в blocker.field_invalid.
      for (const k of Object.keys(errs) as (keyof ConfigForm)[]) {
        track(EVENTS.blockerField, { reason: String(k), where: 'calculate' })
      }
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
      // Успех отмечаем ДО разбора валидации: расчёт был, а заблокирован он
      // или нет — это разные вопросы, и их видно по разным событиям
      // (cta.quote_clicked и blocker.* от живого ответа).
      track(EVENTS.stepDone, { step: 'quote' })
      if (res.validation.blocking) {
        for (const issue of res.validation.issues ?? []) {
          track(EVENTS.blockerApi, { reason: issue.code, element: issue.element ?? '' })
        }
      }
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
      // Отказ сервера — тоже «где затык»: код ошибки важнее текста, который
      // зависит от формулировки.
      track(EVENTS.blockerApi, { reason: apiErrorCode(e), where: 'calculate' })
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
    track(EVENTS.ctaQuote)
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

  // Этап 2 «конструктор»: правка из 3D — меняем целевую высоту ступени и
  // сразу пересчитываем (меш + цена), иначе 3D остался бы старым.
  const adjustStepHeight = (stepHeightMM: number) => {
    const next = { ...config, stepHeightMM: String(stepHeightMM) }
    setConfig(next)
    setErrors(validateForm(next))
    markTouched('stepHeightMM')
    logAction({ action: 'stair.step_height_adjusted', resource_type: 'stair', detail: String(stepHeightMM) })
    void calculate(next)
  }

  // Перетаскивание ступени в 3D меняет общую высоту марша.
  const adjustHeight = (heightMM: number) => {
    const next = { ...config, heightMM: String(heightMM) }
    setConfig(next)
    setErrors(validateForm(next))
    markTouched('heightMM')
    logAction({ action: 'stair.height_adjusted', resource_type: 'stair', detail: String(heightMM) })
    void calculate(next)
  }

  // Горизонтальный drag ступени: шаг комфорта (2h + b) — им сервер считает
  // проступь и забег. Спираль шаг комфорта считает сама, поэтому не шлём.
  const adjustComfortStep = (comfortStepMM: number) => {
    const next = { ...config, comfortStepMM: String(comfortStepMM) }
    setConfig(next)
    setErrors(validateForm(next))
    markTouched('comfortStepMM')
    logAction({
      action: 'stair.comfort_step_adjusted',
      resource_type: 'stair',
      detail: String(comfortStepMM),
    })
    void calculate(next)
  }

  // Перетаскивание площадки у L/П-маршей: ширина (вдоль поворота) или
  // глубина (вдоль марша).
  const adjustLanding = (key: 'landingWidthMM' | 'landingDepthMM', mm: number) => {
    const next = { ...config, [key]: String(mm) }
    setConfig(next)
    setErrors(validateForm(next))
    markTouched(key)
    logAction({ action: 'stair.landing_adjusted', resource_type: 'stair', detail: `${key}=${mm}` })
    void calculate(next)
  }

  const flipDirection = () => {
    const flip = <T extends string>(v: T) => (v === 'left' ? 'right' : 'left') as T
    const next = {
      ...config,
      direction: flip(config.direction),
      spiralDirection: flip(config.spiralDirection),
    }
    setConfig(next)
    setErrors(validateForm(next))
    // flipDirection меняет только направление (строка/enum), числовые поля не трогает — отмечать нечего
    logAction({ action: 'stair.direction_flipped', resource_type: 'stair', detail: next.direction })
    void calculate(next)
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

  // Сцена слева, панель справа. Меш берём из расчёта; до первого расчёта
  // (или при блокирующем ответе, когда геометрии нет) показываем пустое
  // состояние с подсказкой — сцена не должна мигать пустым канвасом.
  const hasMesh =
    !!quote && !quote.validation.blocking && !!quote.mesh?.Vertices?.length && !!quote.mesh?.Triangles
  const stageSolver = quote
    ? solverOf(
        quote,
        config.approachSpaceMM.trim() !== '' ? Number(config.approachSpaceMM) : undefined,
      )
    : null

  // ВНИМАНИЕ: блок объявлен ПОСЛЕ всех хелперов (visible, selectOptions,
  // update, hintOf, setRiser) не по вкусу, а потому что renderField и
  // calcSections вызывают их В МОМЕНТЕ СОЗДАНИЯ массива секций: JSX в
  // content: (<>…</>) вычисляется сразу, а не лениво. Объявленный выше
  // const попадает в temporal dead zone и рендер падает с ReferenceError
  // "Cannot access 'visible' before initialization". Перенос блока выше
  // ломал конструктор целиком (ErrorBoundary на витрине).

  // --- Выбор контрола по полю -------------------------------------------
  // Ползунок требует осмысленной шкалы (min < max), поэтому числовые поля без
  // верхней границы остаются текстовыми. Сегментированные кнопки ставим там,
  // где вариантов мало и они взаимно исключающие.

  // Пределы поля берём из fieldRulesFor, а НЕ из rulesFor по материалу
  // каркаса: у толщины ступени «свой» материал (treadMaterial), и валидация
  // смотрит именно на него. С правилом по каркасу слайдер показывал 3–8 мм
  // при деревянных ступенях, а любое значение ниже 20 отвергалось — то
  // есть ползунок предлагал заведомо невалидные значения.
  const fieldLimit = (k: keyof ConfigForm, bound: 'min' | 'max' | 'step'): number | undefined => {
    const rule = fieldRulesFor(k, config) as FieldRule & { step?: number }
    return bound === 'step' ? rule.step : rule[bound]
  }

  const sliderFor = (k: keyof ConfigForm): boolean => {
    if (typeof config[k] !== 'string') return false
    const rule = rulesFor(k, config.material)
    return rule.min !== undefined && rule.max !== undefined && rule.max > rule.min
  }

  // Шаг ползунка: для дискретных величин (сантиметры) — 5 мм, остальное 1 мм.
  const sliderStep = (k: keyof ConfigForm): number =>
    k === 'roomWidthMM' || k === 'roomLengthMM' || k === 'approachSpaceMM' ? 5 : 1

  const segmentedKeys: ReadonlySet<keyof ConfigForm> = new Set([
    'flight',
    'turnKind',
    'direction',
    'spiralDirection',
    'railing',
    'railingLower',
    'railingLanding',
    'railingUpper',
  ])

  const segmentedFor = (k: keyof ConfigForm): boolean =>
    segmentedKeys.has(k) && selectOptions(k) != null

  const segmentedCols = (k: keyof ConfigForm): 2 | 3 | 4 =>
    k === 'flight' ? 3 : k === 'turnKind' || k === 'direction' || k === 'spiralDirection' ? 2 : 2

  // Один контрол на поле. Вынесен в функцию, потому что секции аккордеона
  // собирают разные наборы полей, и копировать разметку в каждую секцию —
  // гарантированный способ разъехаться с валидацией.
  // Чипы ограждения делаем ниже остальных: «Перила: первый марш» в кнопке
  // 38px занимает две строки и съедает высоту панели плюс-одной.
  const isRailingField = (k: keyof ConfigForm) =>
    k === 'railing' || k === 'railingLower' || k === 'railingLanding' || k === 'railingUpper'

  const renderField = (k: keyof ConfigForm) => {
    if (!visible(k)) return null
    return (
      <div className="field" key={k}>
        {!segmentedFor(k) && !sliderFor(k) && (
          <FieldLabel label={labels[k]} tooltip={tooltips[k]} htmlFor={`cfg-${k}`} />
        )}
        {segmentedFor(k) ? (
          <Segmented
            legend={labels[k]}
            value={config[k] as string}
            options={selectOptions(k)! as Array<{ value: string; label: string }>}
            columns={segmentedCols(k)}
            compact={isRailingField(k)}
            onChange={(v) => update(k, v)}
          />
        ) : sliderFor(k) ? (
          <Slider
            id={`cfg-${k}`}
            label={labels[k]}
            tooltip={tooltips[k]}
            value={config[k] as string}
            min={fieldLimit(k, 'min')}
            max={fieldLimit(k, 'max')}
            step={fieldLimit(k, 'step') ?? sliderStep(k)}
            hint={hintOf(k)}
            invalid={isInvalid(k)}
            onChange={(v) => update(k, v)}
          />
        ) : (
          <input
            id={`cfg-${k}`}
            ref={
              k === 'roomWidthMM' ? roomWidthRef : k === 'roomLengthMM' ? roomLengthRef : undefined
            }
            type="text"
            inputMode="decimal"
            className={isInvalid(k) ? 'field-invalid' : undefined}
            value={config[k] as string}
            onChange={(e) => update(k, e.target.value)}
          />
        )}
        {hintOf(k) && sliderFor(k) ? null : hintOf(k) && <span className="sub">{hintOf(k)}</span>}
        {/* Ошибку «поле обязательно, но пустое» не показываем: поле и так
            красное, а текст только шумел (решение владельца). Остальные
            ошибки — «Не более 3000», «Введите число» — остаются: они
            называют конкретное число, которое надо поменять. */}
        {touched[k] && errors[k] && errors[k] !== REQUIRED_VALUE_ERROR && (
          <span className="error">{errors[k]}</span>
        )}
        {touched[k] && !errors[k] && liveFieldErrors[k] && (
          <span className="error">{liveFieldErrors[k]}</span>
        )}
      </div>
    )
  }

  // Степпера «Количество ступеней» в панели больше нет: по требованию
  // владельца он считался лишним, а число ступеней и так меняется
  // перетаскиванием ступени в 3D. Высота ступени — производная от высоты
  // марша, поэтому отдельного поля для неё в форме тоже нет.

  // Цвет покрытия — только витрина: finishId не уходит в расчёт, он меняет
  // материал в 3D. Прайс не зависит, поэтому подменять им материал нельзя.
  // У каркаса и у ступеней цвета СВОИ: у металлокаркаса чёрный каркас с
  // дубовой ступенью — норма, и одним полем это не выразить.
  const finishOptions = (code: string) =>
    (FINISHES[code] ?? []).map((f) => ({
      value: f.id,
      label: f.label,
      color: (FINISH_SWATCH[f.id] ?? { color: '#b0b0b0' }).color,
    }))
  const frameFinishOptions = finishOptions(config.material)
  const treadCode = config.treadMaterial || config.material
  const treadFinishOptions = finishOptions(treadCode)
  // Тумблер «Дерево / Металл» показывает ВИД материала, а не конкретный код:
  // покупатель выбирает «дерево» и дальше — породу, а не оба решения сразу.
  const isMetalTread = !treadCode.startsWith('WOOD-')
  // Подступенок идёт по материалу ступеней: переключатель заблокирован, он
  // лишь показывает, из чего будут подступенки. Отделка — своими руками только
  // для дерева; для металла подступенок того же цвета, что и ступени, и
  // riserFinishId в 3D не уходит вовсе (см. GeometryViewer).
  const riserFinish = isMetalTread ? treadFinishId : riserFinishId
  const riserFinishOptions = finishOptions(treadCode)
  // Изделие определяет набор материалов каркаса: в металлокаркасе — металлы,
  // в деревянной лестнице — породы дерева. Это не фильтр для удобства:
  // пределы толщины, вес, цена и раскрой у них разные.
  const isWoodProduct = product === 'wood'
  const frameCodes: readonly string[] = isWoodProduct ? WOOD_MATERIALS : METAL_MATERIALS

  // Переключение материала ступеней обязано подтянуть ТОЛЩИНУ под новый
  // материал: у дуба минимум 20 мм, у стали — от 2 мм. Оставить прежнюю
  // толщину — значит отправить расчёт, который сервер отвергнет.
  // Толщина детали обязана лежать в диапазоне ЕЁ материала: 6 мм у стали и
  // 6 мм у дуба — разные детали с разным весом и ценой. Помещая значение
  // вне диапазона, получаем расчёт, который сервер отвергнет.
  const clampThickness = (target: ConfigForm, key: 'stepThicknessMM' | 'stringerThicknessMM', code: string) => {
    const rule = rulesFor(key, code as never)
    const cur = Number(target[key])
    if (!Number.isFinite(cur)) return
    if (rule.min !== undefined && cur < rule.min) target[key] = String(rule.min)
    if (rule.max !== undefined && cur > rule.max) target[key] = String(rule.max)
  }

  // Смена изделия приводит конфигурацию в порядок: материалы переезжают в
  // допустимые для нового изделия, толщины подтягиваются под их пределы.
  // Без этого переключение оставляло бы, например, стальной каркас 6 мм в
  // деревянной лестнице — и первый же расчёт уходил бы с MFG-MATERIAL.
  const switchProduct = (id: string) => {
    if (id === product) return
    const d = PRODUCT_DEFAULT[id] ?? PRODUCT_DEFAULT.metal
    const allowed = id === 'wood' ? WOOD_MATERIALS : METAL_MATERIALS
    const isAllowed = (c: string) => (allowed as readonly string[]).includes(c)

    const next: ConfigForm = {
      ...config,
      material: isAllowed(config.material)
        ? config.material
        : (d.frame as ConfigForm['material']),
      treadMaterial: isAllowed(config.treadMaterial) ? config.treadMaterial : d.tread,
      flight: config.flight,
    }
    // Толщина косоура — по материалу каркаса, толщина ступени — по
    // материалу ступеней: это разные детали с разными пределами.
    clampThickness(next, 'stringerThicknessMM', next.material)
    clampThickness(next, 'stepThicknessMM', next.treadMaterial)
    setProduct(id)
    setConfig(next)
    setFinishId(FINISHES[next.material]?.[0]?.id ?? '')
    setTreadFinish(FINISHES[next.treadMaterial]?.[0]?.id ?? '')
    setErrors(validateForm(next))
    setQuote(null)
    setVariations(null)
    setActiveVariationId(null)
    setStatus(null)
  }

  const setTreadMaterial = (code: string) => {
    const next = { ...config, treadMaterial: code }
    const limit = fieldRulesFor('stepThicknessMM', next)
    const cur = Number(config.stepThicknessMM)
    if (Number.isFinite(cur) && (limit.min !== undefined && cur < limit.min)) {
      next.stepThicknessMM = String(limit.min)
    }
    if (Number.isFinite(cur) && limit.max !== undefined && cur > limit.max) {
      next.stepThicknessMM = String(limit.max)
    }
    setTreadFinish(FINISHES[code]?.[0]?.id ?? '')
    // Отделка подступенков меняет материал вместе с материалом ступеней:
    // иначе после «сталь → дуб» у подступенков осталась бы стальная палитра.
    setRiserFinish(FINISHES[code]?.[0]?.id ?? '')
    setConfig(next)
  }


  const isTurned = config.flight === 'l_shape' || config.flight === 'u_shape'

  const calcSections: AccordionSection[] = [
    {
      id: 'main',
      title: 'Основные настройки',
      content: (
        <>
          {renderField('flight')}
          {renderField('heightMM')}
          {renderField('widthMM')}
          {/* Подступень — единственный переключатель на «Да/Нет», поэтому
              подпись и сам переключатель стоят в одну строку: отдельная
              строка под подписью была пустой высотой ради одного слова. */}
          <div className="field field--inline">
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
          </div>
        </>
      ),
    },
    {
      id: 'turn',
      title: 'Настройки поворота',
      hidden: !isTurned,
      content: (
        <>
          {renderField('turnKind')}
          {config.turnKind === 'winder' ? renderField('winderCountMM') : null}
          {renderField('direction')}
          {renderField('lowerStepCountMM')}
          {config.turnKind === 'winder' ? null : renderField('landingWidthMM')}
          {config.turnKind === 'winder' ? null : renderField('landingDepthMM')}
        </>
      ),
    },
    {
      id: 'railings',
      title: 'Ограждение',
      content: (
        <>
          {config.flight === 'straight' ? renderField('railing') : null}
          {isTurned ? (
            <>
              {renderField('railingLower')}
              {renderField('railingLanding')}
              {renderField('railingUpper')}
            </>
          ) : null}
          {renderField('railingHeightMM')}
        </>
      ),
    },
    {
      id: 'color',
      title: 'Цвет и материал',
      content: (
        <>
          {/* Материал каркаса: кнопка-сегмент, если вариантов больше одного.
              При одном варианте (сейчас сталь) сегмент из одной кнопки —
              это ложный выбор: кажется, что можно переключить. */}
          {frameCodes.length > 1 ? (
          <Segmented
            legend={isWoodProduct ? 'Материал лестницы' : 'Каркас (косоуры и площадка)'}
            value={config.material}
            columns={isWoodProduct ? 4 : 3}
            options={frameCodes.map((code) => {
              const o = materialOptions.find((m) => m.value === code)
              return { value: code, label: o?.label ?? code }
            })}
            onChange={(v) => {
              // material в ConfigForm — литеральный union кодов каталога,
              // а Segmented отдаёт string: сужаем явно, иначе пришлось бы
              // размазывать ConfigForm['material'] по всем обработчикам.
              const code = v as ConfigForm['material']
              const next: ConfigForm = { ...config, material: code }
              // Толщина косоура привязана к материалу каркаса.
              clampThickness(next, 'stringerThicknessMM', code)
              setConfig(next)
              setFinishId(FINISHES[code]?.[0]?.id ?? '')
            }}
          />
          ) : (
            <div className="field">
              <span className="segmented__legend">
                {isWoodProduct ? 'Материал лестницы' : 'Каркас (косоуры и площадка)'}
              </span>
              <p className="readonly-value">
                {materialOptions.find((m) => m.value === config.material)?.label ?? config.material}
              </p>
            </div>
          )}

          {isWoodProduct ? (
            <Segmented
              legend="Порода ступеней"
              value={treadCode}
              columns={4}
              options={WOOD_MATERIALS.map((code) => {
                const o = materialOptions.find((m) => m.value === code)
                return { value: code, label: o?.label ?? code }
              })}
              onChange={(v) => setTreadMaterial(v)}
            />
          ) : (
            <Segmented
              legend="Ступени (проступи и площадка)"
              value={isMetalTread ? 'metal' : 'wood'}
              columns={2}
              options={[
                { value: 'wood', label: 'Дерево' },
                { value: 'metal', label: 'Металл' },
              ]}
              onChange={(v) => setTreadMaterial(v === 'wood' ? 'WOOD-OAK' : 'STEEL-S235')}
            />
          )}

          {/* Толщина ступени — сразу под выбором материала ступеней, потому
              что её пределы задаёт ИМЕННО он: у дуба 20–60 мм, у стали 2–6 мм.
              Раньше поле стояло в «Основных настройках», где до материала
              ступеней ещё надо было догадаться, — и человек ставил 40 мм,
              потом переключал на металл и получал отказ сервера. */}
          {renderField('stepThicknessMM')}

          {frameFinishOptions.length > 0 && (
            <SwatchGroup
              legend={isWoodProduct ? 'Цвет покрытия' : 'Цвет каркаса'}
              value={finishId}
              options={frameFinishOptions}
              onChange={setFinishId}
            />
          )}
          {treadFinishOptions.length > 0 && (
            <SwatchGroup
              legend={isMetalTread ? 'Цвет ступеней' : 'Отделка ступеней'}
              value={treadFinishId}
              options={treadFinishOptions}
              onChange={setTreadFinish}
            />
          )}
          {/* Подступенки. Блок есть только когда подступенки включены
              («Подступень: Да») — выбирать нечего, если их нет. Материал
              заблокирован: он следует за материалом ступеней, и это же
              правило режет подступенки на бэке. Отделка — своим рядом только
              для дерева; для металла подступенок того же цвета, что ступени. */}
          {config.riser && (
            <Segmented
              legend="Подступенки"
              // Подсказка под блоком объясняет правило, но «заблокировано»
              // в подписи видно сразу — об этом просил владелец.
              value={isMetalTread ? 'metal' : 'wood'}
              columns={2}
              locked
              lockedHint="Следует за материалом ступеней"
              options={[
                {
                  value: 'wood',
                  // Активная кнопка показывает НАСТОЯЩИЙ материал ступеней
                  // (для дерева — породу), а не обобщённое «Дерево».
                  label: isMetalTread
                    ? 'Дерево'
                    : (materialOptions.find((m) => m.value === treadCode)?.label ?? 'Дерево'),
                  disabled: isMetalTread,
                },
                { value: 'metal', label: 'Металл', disabled: !isMetalTread },
              ]}
              onChange={() => {}}
            />
          )}
          {config.riser && !isMetalTread && riserFinishOptions.length > 0 && (
            <SwatchGroup
              legend="Отделка подступенков"
              value={riserFinishId}
              options={riserFinishOptions}
              onChange={setRiserFinish}
            />
          )}
        </>
      ),
    },
    {
      id: 'room',
      title: 'Помещение',
      content: (
        <>
          {renderField('roomWidthMM')}
          {renderField('roomLengthMM')}
          {renderField('approachSpaceMM')}
          {renderField('clearanceMM')}
        </>
      ),
    },
  ]

  return (
    <div className="calc">
      <div className="calc__stage">
        {hasMesh && quote && stageSolver ? (
          <Stage3D
            quote={quote}
            solver={stageSolver}
            material={config.material}
            treadMaterial={treadCode}
            finishId={finishId}
            treadFinishId={treadFinishId}
            riserFinishId={riserFinish}
            onAdjustStepHeight={adjustStepHeight}
            onFlipDirection={flipDirection}
            onAdjustHeight={adjustHeight}
            onAdjustComfortStep={adjustComfortStep}
            comfortStepMM={Number(config.comfortStepMM) || undefined}
            onAdjustLandingWidth={(mm) => adjustLanding('landingWidthMM', mm)}
            onAdjustLandingDepth={(mm) => adjustLanding('landingDepthMM', mm)}
            landingWidthMM={Number(config.landingWidthMM) || undefined}
            landingDepthMM={Number(config.landingDepthMM) || undefined}
            heightMM={Number(config.heightMM) || undefined}
          />
        ) : (
          <div className="calc__stage-empty">
            <h2>3D-модель лестницы</h2>
          </div>
        )}
      </div>

      {/* Результат расчёта — отдельная колонка между 3D и конструктором.
          Раньше карточки результата лежали в том же рельсе, что и форма:
          рельс высотой в экран делил высоту на шесть панелей, панель
          конструктора схлопывалась, и её содержимое (вкладки, кнопки)
          вылезало поверх карточек, а цена уезжала на ~1000px вниз. В
          референсе (niora) результат — отдельная колонка «СТОИМОСТЬ»
          рядом с 3D; то же разделение вернуло конструктору всю высоту. */}
      {quote && (
        <aside className="calc__cost" aria-label="Результат расчёта">
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
              split
              onApplySuggestion={applySuggestion}
              onApplyVariation={applyGalleryVariation}
              variations={galleryVariations.length > 0 ? galleryVariations : undefined}
              activeVariationId={activeVariationId}
              material={config.material}
              approachSpaceMM={config.approachSpaceMM}
              heightMM={Number(config.heightMM) || undefined}
              onAdjustStepHeight={adjustStepHeight}
              onFlipDirection={flipDirection}
              onAdjustHeight={adjustHeight}
              onAdjustComfortStep={adjustComfortStep}
              comfortStepMM={Number(config.comfortStepMM) || undefined}
              onAdjustLandingWidth={(mm) => adjustLanding('landingWidthMM', mm)}
              onAdjustLandingDepth={(mm) => adjustLanding('landingDepthMM', mm)}
              landingWidthMM={Number(config.landingWidthMM) || undefined}
              landingDepthMM={Number(config.landingDepthMM) || undefined}
            />
            {!quote.validation.blocking && quote.pricing && request && (
              <OrderForm
                quote={quote}
                config={request}
                onCreated={() => setStatus('Заказ отправлен. Следите за статусом в кабинете.')}
              />
            )}
        </aside>
      )}
      <aside className="calc__rail">
      <section className="panel">
        <h2>Конструктор лестницы</h2>
        <form onSubmit={handleSubmit}>
          {/* Блок «Готовые решения» УДАЛЁН намеренно. Он был в конструкторе
              до редизайна, в референсе niora его нет, и после разделения
              материалов он стал actively harmful: пресеты задавали поле
              material (материал ВСЕЙ лестницы), поэтому «Скандинавский дуб»
              в калькуляторе металлокаркаса давал деревянный каркас, а
              «Сталь» ставила толщину ступени 6 мм
              при деревянных ступенях (минимум 20 мм) — то есть оставляла
              конфигурацию заведомо невалидной. Всё, что пресеты задавали,
              теперь выбирается явно в секции «Цвет и материал». */}
          <ProductTabs
            products={PRODUCTS}
            value={product}
            onChange={switchProduct}
          />

          <Accordion sections={calcSections} defaultOpen="main" />


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

      </aside>
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