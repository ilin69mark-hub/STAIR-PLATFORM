import { useId } from 'react'

// Контролы калькулятора: ползунок с числовым полем и сегментированные кнопки.
// Оба компонента без состояния — значение приходит сверху, наружу уходит
// только через onChange. Шкала ползунка берётся из limits (config.rulesFor),
// то есть из тех же нормативов, что проверяет бэкенд: UI не может предложить
// значение, которое сервер потом отвергнет.

export interface SliderProps {
  id: string
  label: string
  /** Текущее значение строкой (форма в конструкторе хранит строки). */
  value: string
  onChange: (value: string) => void
  min?: number
  max?: number
  step?: number
  unit?: string
  hint?: string
  invalid?: boolean
  disabled?: boolean
  /** Единица измерения в числовом поле; по умолчанию «мм». */
  tooltip?: string
}

export function Slider({
  id,
  label,
  value,
  onChange,
  min,
  max,
  step = 1,
  unit = 'мм',
  hint,
  invalid,
  disabled,
  tooltip,
}: SliderProps) {
  const hintId = useId()
  // Пустое значение нельзя скармливать <input type=range>: браузер
  // принудительно подставит середину шкалы, и пользователь потеряет
  // невведённое поле. Поэтому при пустом показываем min и помечаем
  // состояние data-empty — стилизуем поле иначе.
  const empty = value.trim() === ''
  const num = Number(value)
  const hasRange = min !== undefined && max !== undefined && max > min
  const rangeValue = empty || Number.isNaN(num) ? (min ?? 0) : num

  return (
    <div className="slider" data-empty={empty || undefined}>
      <div className="slider__head">
        <div className="field-label">
          <label className="slider__label" htmlFor={id}>
            {label}
          </label>
          {/* Подсказка «?» повторяет разметку FieldLabel: у ползунков своя
              подпись внутри, поэтому FieldLabel не рендерится, и без этого
              блока все пояснения к полям (просвет, перила, комнаты) просто
              исчезли из интерфейса. */}
          {tooltip && (
            <span className="field-help" tabIndex={0}>
              ?
              <span className="field-help-tip" role="tooltip">
                {tooltip}
              </span>
            </span>
          )}
        </div>
        <div className="slider__value">
          <input
            id={id}
            className={`slider__number${invalid ? ' field-invalid' : ''}`}
            type="text"
            // numeric, а не decimal: все значения панели — целые
            // миллиметры, десятичной запятой тут взяться неоткуда.
            inputMode="numeric"
            // Подсказка мобильным клавиатурам, не валидация: pattern сам по
            // себе буквы в поле не блокирует, только помечает форму.
            pattern="[0-9]*"
            value={value}
            disabled={disabled}
            aria-invalid={invalid || undefined}
            aria-describedby={hint ? hintId : undefined}
            // Буквы, знаки и пробелы отбрасываются на входе. Поле можно
            // очистить (пустая строка — валидное промежуточное состояние,
            // из него печатается новое число), поэтому фильтр не должен
            // запрещать пустое значение. Считаем в строке, а не Number(),
            // иначе «1200» превратилось бы в «1200» с потерей ведущих нулей
            // и не дало бы допечатать цифру.
            onChange={(e) => onChange(e.target.value.replace(/\D+/g, ''))}
          />
          <span className="slider__unit">{unit}</span>
        </div>
      </div>
      {hasRange && (
        <input
          // id у ползунка свой, производный от id числа: подпись связана с
          // ЧИСЛОВЫМ полем (его правят с клавиатуры), а ползунок —
          // альтернативный способ того же значения. Оба получают aria-label,
          // чтобы assistive tech называл их, а не «ползунок».
          id={`${id}-range`}
          aria-label={`${label}, ползунок`}
          className="slider__range"
          type="range"
          min={min}
          max={max}
          step={step}
          value={rangeValue}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value)}
        />
      )}
      {hasRange && (
        <div className="slider__scale" aria-hidden="true">
          <span>{min}</span>
          <span>{max}</span>
        </div>
      )}
      {hint && (
        <span className="slider__hint" id={hintId}>
          {hint}
        </span>
      )}
    </div>
  )
}

export interface SegmentedOption {
  value: string
  label: string
  hint?: string
  /** Кнопка заблокирована: значение показано, но выбрать его нельзя. */
  disabled?: boolean
}

export interface SegmentedProps {
  legend: string
  value: string
  options: SegmentedOption[]
  onChange: (value: string) => void
  /** grid-cols: 2 | 3 | 4 — под длину подписей. */
  columns?: 2 | 3 | 4
  /** Низкие кнопки без поля hint: подписи вроде «Перила: первый марш» в
   *  кнопке 38px занимают две строки и съедают высоту панели. */
  compact?: boolean
  hint?: string
  /** Весь переключатель заблокирован: значение показано, но не меняется.
   *  Так выглядит производное поле — например, материал подступенков,
   *  который обязан следовать за материалом ступеней. */
  locked?: boolean
  lockedHint?: string
}

export function Segmented({
  legend,
  value,
  options,
  onChange,
  columns = 3,
  hint,
  compact,
  locked,
  lockedHint,
}: SegmentedProps) {
  return (
    <fieldset
      className="segmented"
      data-cols={columns}
      data-compact={compact || undefined}
      data-locked={locked || undefined}
    >
      <legend className="segmented__legend">
        {legend}
        {locked && (
          // Замок в подписи — самый заметный признак блокировки: рамка и
          // курсор читаются не сразу, а здесь видно сразу. aria-hidden,
          // потому что сам смысл уже произнесён подписью и подсказкой ниже.
          <span className="segmented__lock" aria-hidden="true">
            <svg viewBox="0 0 12 14" width="11" height="13" focusable="false">
              <path
                d="M2 6V4a4 4 0 1 1 8 0v2"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.6"
              />
              <rect x="1" y="6" width="10" height="7" rx="1.6" fill="currentColor" />
            </svg>
            заблокировано
          </span>
        )}
      </legend>
      <div className="segmented__grid">
        {options.map((o) =>
          locked ? (
            // Заблокированный переключатель — это ПОКАЗАТЕЛЬ, а не ввод.
            // role="radio" здесь был бы враньём: получилось бы две группы
            // радиокнопок с одинаковыми именами («Металл» — у ступеней и у
            // подступенков), и скринридер читал бы заблокированное поле как
            // рабочее. aria-disabled оставляет кнопку в порядке обхода и
            // объявляет «недоступно», нативный disabled — убрал бы и то, и
            // другое: поле нельзя было бы даже прочитать с клавиатуры.
            <button
              key={o.value}
              type="button"
              aria-disabled="true"
              className={`segmented__item${value === o.value ? ' is-active' : ''}`}
              onClick={() => {}}
              title={o.hint}
            >
              <span className="segmented__label">{o.label}</span>
              {o.hint && <span className="segmented__hint">{o.hint}</span>}
            </button>
          ) : (
            <button
              key={o.value}
              type="button"
              role="radio"
              aria-checked={value === o.value}
              disabled={o.disabled}
              className={`segmented__item${value === o.value ? ' is-active' : ''}`}
              onClick={() => onChange(o.value)}
              title={o.hint}
            >
              <span className="segmented__label">{o.label}</span>
              {o.hint && <span className="segmented__hint">{o.hint}</span>}
            </button>
          ),
        )}
      </div>
      {(lockedHint || hint) && <span className="segmented__hint-line">{lockedHint ?? hint}</span>}
    </fieldset>
  )
}

// --- Переключатель изделий (Металлокаркас / Деревянные) ---------------------
//
// Два больших таба задают ТИП ИЗДЕЛИЯ, а не просто вкладку настроек: от него
// зависит набор параметров (у металлокаркаса нет толщины ступени в дереве, у
// деревянной — нет толщины косоура), материал и прайс. Поэтому переключение
// сбрасывает не только показ секций.

export interface ProductTab {
  id: string
  label: string
  disabled?: boolean
  badge?: string
}

export interface ProductTabsProps {
  products: ProductTab[]
  value: string
  onChange: (id: string) => void
}

export function ProductTabs({ products, value, onChange }: ProductTabsProps) {
  return (
    <div className="product-tabs" role="tablist" aria-label="Тип лестницы">
      {products.map((p) => (
        <button
          key={p.id}
          type="button"
          role="tab"
          aria-selected={value === p.id}
          disabled={p.disabled}
          className={`product-tab${value === p.id ? ' is-active' : ''}`}
          onClick={() => !p.disabled && onChange(p.id)}
        >
          <span className="product-tab__label">{p.label}</span>
          {p.badge && <span className="product-tab__badge">{p.badge}</span>}
        </button>
      ))}
    </div>
  )
}

// --- Сетка образцов цвета ---------------------------------------------------
//
// Отличие от Segmented: у кнопки есть кружок-образец реального цвета. На
// металле разница «чёрный / белый / шампань» не читается по словам, а по
// пятну — поэтому образец здесь не украшение, а носитель информации.

export interface SwatchOption {
  value: string
  label: string
  /** CSS-цвет образца. */
  color: string
}

export interface SwatchGroupProps {
  legend: string
  value: string
  options: SwatchOption[]
  onChange: (value: string) => void
  hint?: string
}

export function SwatchGroup({ legend, value, options, onChange, hint }: SwatchGroupProps) {
  return (
    <fieldset className="swatches">
      <legend className="swatches__legend">{legend}</legend>
      <div className="swatches__grid">
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={value === o.value}
            className={`swatch${value === o.value ? ' is-active' : ''}`}
            onClick={() => onChange(o.value)}
          >
            <span
              className="swatch__dot"
              style={{ backgroundColor: o.color }}
              aria-hidden="true"
            />
            <span className="swatch__label">{o.label}</span>
          </button>
        ))}
      </div>
      {hint && <span className="swatches__hint">{hint}</span>}
    </fieldset>
  )
}
