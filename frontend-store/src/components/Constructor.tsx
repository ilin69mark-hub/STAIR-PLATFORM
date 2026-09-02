import { useRef, useState } from 'react'
import type { ConfigForm } from '@shared/config'
import { defaultConfig, directionOptions, flightOptions, materialOptions, railingForSpiral, railingLabel, railingOptions, rulesFor, spiralDirectionOptions, toRequest, validateForm, type FieldErrors, type FieldRule } from '@shared/config'
import type { QuoteResult, QuoteSuggestion, Variation } from '@shared/types'
import { quoteApi } from '../api/store'
import { apiErrorMessage } from '../auth/errors'
import { QuoteResult as QuoteResultView } from './QuoteResult'
import { OrderForm } from './OrderForm'
import { logAction } from '@shared/api/audit'

const orderFields: Array<keyof ConfigForm> = [
  'widthMM',
  'heightMM',
  'flight',
  'material',
  'stepHeightMM',
  'stepThicknessMM',
  'clearanceMM',
  'railingHeightMM',
  'railing',
  'railingLower',
  'railingLanding',
  'railingUpper',
  'direction',
  'spiralDirection',
  'landingWidthMM',
  'landingDepthMM',
  'roomWidthMM',
  'roomLengthMM',
  'approachSpaceMM',
  'lowerStepCountMM',
  'outerRadiusMM',
  'comfortStepMM',
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
  stepHeightMM: 'Комфортно: 150–190 мм',
  riser: 'Подступенок — вертикальная грань под ступенью. Его высота равна высоте ступени и рассчитывается автоматически.',
  clearanceMM: 'Рекомендуем ≥ 2000 мм',
  railingHeightMM: 'Рекомендуем 900–1100 мм',
  comfortStepMM: '600–640 мм (опционально)',
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

// Пустая форма: поля не предзаполнены. Тип марша и скрытый косоур сохраняются.
const emptyConfig: ConfigForm = {
  ...defaultConfig,
  widthMM: '',
  heightMM: '',
  stepHeightMM: '',
  stepThicknessMM: '',
  clearanceMM: '',
  railingHeightMM: '',
  comfortStepMM: '',
  landingWidthMM: '',
  lowerStepCountMM: '',
  outerRadiusMM: '',
}

// Конструктор: параметры лестницы → предварительный расчёт (анонимно).
export function Constructor() {
  const [config, setConfig] = useState<ConfigForm>(emptyConfig)
  // Ошибки считаем сразу: пустые обязательные поля подсвечиваются красным
  // при первом показе, не только после ввода/клика.
  const [errors, setErrors] = useState<FieldErrors>(() => validateForm(emptyConfig))
  const [quote, setQuote] = useState<QuoteResult | null>(null)
  const [request, setRequest] = useState<Record<string, unknown> | null>(null)
  const [status, setStatus] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const visible = (k: keyof ConfigForm): boolean => {
    if (k === 'stringerThicknessMM') return false // скрыт: единый косоур по умолчанию
    if ((k === 'landingWidthMM' || k === 'landingDepthMM' || k === 'lowerStepCountMM') &&
      config.flight !== 'l_shape' && config.flight !== 'u_shape') {
      return false
    }
    if ((k === 'roomWidthMM' || k === 'roomLengthMM') && config.flight !== 'l_shape' && config.flight !== 'straight' && config.flight !== 'u_shape' && config.flight !== 'spiral') return false
    // approachSpaceMM показывается для всех типов марша (EDR-0023).
    if (k === 'outerRadiusMM' && config.flight !== 'spiral') return false
    // Спираль считает шаг комфорта сама (S = 2h + b_ход); остальные марши используют поле.
    if (k === 'comfortStepMM' && config.flight === 'spiral') return false
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
  const update = (k: keyof ConfigForm, v: string) => {
    const next = { ...config, [k]: v }
    setConfig(next)
    setErrors(validateForm(next))
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
    } catch (e) {
      setStatus(apiErrorMessage(e, 'Не удалось выполнить расчёт'))
    } finally {
      setBusy(false)
    }
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
  const applyVariation = (v: Variation) => {
    const merged = { ...config } as unknown as Record<string, string>
    for (const [k, val] of Object.entries(v.config)) {
      if (val === '') continue
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
    void calculate(next)
    logAction({
      action: 'stair.variation_applied',
      resource_type: 'stair',
      detail: JSON.stringify({ id: v.id, title: v.title }),
    })
  }

  const reset = () => {
    setConfig(emptyConfig)
    setErrors(validateForm(emptyConfig))
    setQuote(null)
    setRequest(null)
    setStatus(null)
  }

  return (
    <div>
      <section className="panel">
        <h2>Конструктор лестницы</h2>
        <p className="sub">Задайте параметры — мы рассчитаем геометрию и предварительную цену.</p>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void calculate()
          }}
        >
          <div className="form-grid">
            {orderFields.map((k) => (
              <div className="field" key={k} hidden={!visible(k)}>
                <FieldLabel label={labels[k]} tooltip={tooltips[k]} htmlFor={`cfg-${k}`} />
                {selectOptions(k) ? (
                  <select id={`cfg-${k}`} value={config[k] as string} onChange={(e) => update(k, e.target.value)}>
                    {selectOptions(k)!.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    id={`cfg-${k}`}
                    type="text"
                    inputMode="decimal"
                    className={errors[k] ? 'field-invalid' : undefined}
                    value={config[k] as string}
                    onChange={(e) => update(k, e.target.value)}
                  />
                )}
                {hintOf(k) && <span className="sub">{hintOf(k)}</span>}
                {errors[k] && <span className="error">{errors[k]}</span>}
              </div>
            ))}
            <div className="field" hidden={config.flight !== 'spiral'}>
              <FieldLabel label={labels.railing} tooltip={tooltips.railing} htmlFor="cfg-railing-auto" />
              {/* Спираль: перила всегда с одной стороны, сторона автоматически
                  от направления закрутки (CONF-SPIRAL-RAILING). */}
              <input id="cfg-railing-auto" type="text" readOnly value={railingLabel(railingForSpiral(config.spiralDirection))} />
              <span className="sub">Авто: по направлению спирали</span>
            </div>
            <div className="field" hidden={config.flight === 'spiral'}>
              <FieldLabel label={labels.riser} htmlFor="cfg-riser" />
              <label className="checkbox">
                <input id="cfg-riser" type="checkbox" checked={config.riser} onChange={(e) => setRiser(e.target.checked)} />
                <span>{config.riser ? 'Да' : 'Нет'}</span>
              </label>
              {hints.riser && <span className="sub">{hints.riser}</span>}
            </div>
          </div>
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
          <QuoteResultView
            quote={quote}
            onApplySuggestion={applySuggestion}
            onApplyVariation={applyVariation}
            material={config.material}
            approachSpaceMM={config.approachSpaceMM}
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