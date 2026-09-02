import { useEffect, useRef, useState } from 'react'
import type { Calculation, OptimizeTarget, Project } from '@shared/types'
import { ApiError } from '@shared/types'
import { projectsApi } from '../api/projects'
import {
  defaultConfig,
  defaultRates,
  flightOptions,
  flightFields,
  materialOptions,
  directionOptions,
  spiralDirectionOptions,
  railingOptions,
  railingForSpiral,
  railingLabel,
  rulesFor,
  toRequest,
  toRatesRequest,
  validateForm,
  type ConfigForm,
  type FieldErrors,
  type RatesForm,
} from '@shared/config'
import { ResultPanel } from './ResultPanel'
import type { Variation } from '@shared/types'
import { MembersPanel } from './MembersPanel'
import { CommentsPanel } from './CommentsPanel'
import { ReviewPanel } from './ReviewPanel'
import { ApprovalsPanel } from './ApprovalsPanel'
import { VersionsPanel } from './VersionsPanel'
import { AuditPanel } from './AuditPanel'
import { AssistantPanel } from './AssistantPanel'
import { logAction } from '@shared/api/audit'
import type { ProjectStatus } from '@shared/types'

interface Props {
  projectId: string
  onBack: () => void
  onChanged: () => void
}

export function ProjectDetail({ projectId, onBack, onChanged }: Props) {
  const [project, setProject] = useState<Project | null>(null)
  const [config, setConfig] = useState<ConfigForm>(defaultConfig)
  const [rates, setRates] = useState<RatesForm>(defaultRates)
  const [calculation, setCalculation] = useState<Calculation | null>(null)
  const [savedAt, setSavedAt] = useState<Date | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [optimizeTarget, setOptimizeTarget] = useState<OptimizeTarget>('price')
  const [optimizeMsg, setOptimizeMsg] = useState<string | null>(null)
  // Превью вариации (A/B/C) без сохранения: перебор альтернатив перед
  // тем, как пользователь выберет одну и зафиксирует её расчётом.
  const [previewCalculation, setPreviewCalculation] = useState<Calculation | null>(null)
  const [activeVariantId, setActiveVariantId] = useState<string | undefined>(undefined)

  const errors = validateForm(config)
  const hasHardErrors = Object.values(errors).some(
    (e) => e === 'Введите число' || e === 'Укажите значение',
  )

  useEffect(() => {
    projectsApi
      .get(projectId)
      .then(setProject)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Не удалось загрузить проект'))
  }, [projectId])

  const configChangeTimer = useRef<number | null>(null)
  const setField = (key: keyof ConfigForm, value: string | boolean) => {
    setConfig((c) => ({ ...c, [key]: value }))
    // Аудит изменения поля (debounce 600 мс, best-effort).
    if (configChangeTimer.current) window.clearTimeout(configChangeTimer.current)
    configChangeTimer.current = window.setTimeout(() => {
      logAction({
        action: 'stair.config_changed',
        resource_type: 'stair',
        resource_id: projectId,
        detail: JSON.stringify({ field: key }),
      })
    }, 600)
  }

  const setRate = (key: keyof RatesForm, value: string) =>
    setRates((r) => ({ ...r, [key]: value }))

  const handleCalculate = async () => {
    setBusy(true)
    setError(null)
    setOptimizeMsg(null)
    try {
      const body = { ...toRequest(config) }
      const ratesReq = toRatesRequest(rates)
      if (ratesReq) body.rates = ratesReq
      const calc = await projectsApi.calculate(projectId, body)
      setCalculation(calc)
      setSavedAt(new Date())
      onChanged()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось выполнить расчёт')
    } finally {
      setBusy(false)
    }
  }

  // handleOptimize — поиск оптимальной конфигурации (EDR-0032): лучшая
  // конфигурация применяется к форме и сохраняется как расчёт проекта.
  const handleOptimize = async () => {
    setBusy(true)
    setError(null)
    setOptimizeMsg(null)
    try {
      const body: Record<string, unknown> = { ...toRequest(config), target: optimizeTarget }
      const ratesReq = toRatesRequest(rates)
      if (ratesReq) body.rates = ratesReq
      const resp = await projectsApi.optimize(projectId, body)
      if (!resp.valid || !resp.best) {
        setOptimizeMsg(
          `Оптимизация не нашла допустимую конфигурацию (проверено ${resp.evaluated} вариантов).`,
        )
        return
      }
      const best = resp.best
      setConfig((c) => ({
        ...c,
        stepHeightMM: String(best.step_height_mm),
        lowerStepCountMM:
          best.lower_step_count !== undefined ? String(best.lower_step_count) : c.lowerStepCountMM,
        comfortStepMM: best.comfort_step_mm ? String(best.comfort_step_mm) : c.comfortStepMM,
      }))
      if (resp.saved && resp.calculation_id) {
        setCalculation({
          project_id: projectId,
          calculation_id: resp.calculation_id,
          configuration_id: resp.configuration_id ?? '',
          valid: true,
          blocking: false,
          created_at: new Date().toISOString(),
          result: best.result,
        })
      }
      setSavedAt(new Date())
      const objective = new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format(
        resp.objective,
      )
      setOptimizeMsg(
        `Оптимум найден: ${best.step_count} ступ. · ${objective} ₽ · проверено ${resp.evaluated} вариантов.`,
      )
      onChanged()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось выполнить оптимизацию')
    } finally {
      setBusy(false)
    }
  }

  const handleExport = () => {
    window.location.href = projectsApi.exportUrl(projectId)
  }

  // applyVariation — пользователь выбрал вариант A/B/C: сливаем его конфиг
  // в форму и рассчитываем БЕЗ сохранения (preview), показывая результат
  // как превью. Можно перебирать варианты, пока не выберется подходящий.
  const applyVariation = async (v: Variation) => {
    const next = { ...config, ...v.config } as ConfigForm
    // Прямой марш: свободное пространство перед первой ступенью обязательно
    // (норма 1000–1200 мм); пустое/отсутствующее значение — 1000 мм по умолчанию.
    if ((next.approachSpaceMM ?? '').trim() === '') {
      next.approachSpaceMM = '1000'
    }
    setConfig(next)
    setActiveVariantId(v.id)
    logAction({
      action: 'stair.variation_applied',
      resource_type: 'stair',
      resource_id: projectId,
      detail: JSON.stringify({ id: v.id, title: v.title }),
    })
    setBusy(true)
    setError(null)
    try {
      const body = { ...toRequest(next) }
      const ratesReq = toRatesRequest(rates)
      if (ratesReq) body.rates = ratesReq
      const calc = await projectsApi.preview(projectId, body)
      setPreviewCalculation(calc)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось рассчитать вариант')
    } finally {
      setBusy(false)
    }
  }

  // applyPreview — зафиксировать выбранный вариант как сохранённый расчёт.
  const applyPreview = async () => {
    setPreviewCalculation(null)
    setActiveVariantId(undefined)
    await handleCalculate()
  }

  const closePreview = () => {
    setPreviewCalculation(null)
    setActiveVariantId(undefined)
  }

  const handleStatusChange = (status: ProjectStatus) => {
    setProject((p) => (p ? { ...p, status } : p))
  }

  return (
    <div className="page">
      <header className="page__header">
        <button className="btn btn--ghost" onClick={onBack}>
          ← Проекты
        </button>
        <h1 className="page__title">{project?.name ?? 'Проект'}</h1>
        <p className="page__subtitle">
          {project ? `${project.status} · создан ${new Date(project.created_at).toLocaleString('ru-RU')}` : 'Загрузка…'}
        </p>
      </header>

      {error && <div className="alert alert--error">{error}</div>}
      {savedAt && (
        <div className="alert alert--ok">
          Расчёт сохранён · {savedAt.toLocaleTimeString('ru-RU')}
        </div>
      )}

      <div className="projects">
        <section className="panel">
          <h2 className="panel__title">Параметры лестницы</h2>
          <ConfigForm fields={config} errors={errors} onChange={setField} />
          <RatesFormSection rates={rates} onChange={setRate} />
          <div className="row row--actions">
            <select
              className="field__input field__input--inline"
              aria-label="Цель оптимизации"
              value={optimizeTarget}
              onChange={(e) => setOptimizeTarget(e.target.value as OptimizeTarget)}
            >
              <option value="price">Оптимизировать цену</option>
              <option value="cost">Оптимизировать себестоимость</option>
              <option value="material">Оптимизировать материал</option>
            </select>
            <button className="btn btn--primary" onClick={handleCalculate} disabled={busy || hasHardErrors}>
              {busy ? 'Расчёт…' : 'Рассчитать'}
            </button>
            <button className="btn" onClick={handleOptimize} disabled={busy || hasHardErrors}>
              {busy ? 'Поиск…' : 'Оптимизировать'}
            </button>
            <button className="btn" onClick={handleExport} disabled={!calculation}>
              Экспорт JSON
            </button>
          </div>
          {hasHardErrors && (
            <p className="muted">Исправьте нечисловые или пустые поля перед расчётом.</p>
          )}
          {optimizeMsg && <p className="muted">{optimizeMsg}</p>}
          {calculation && (
            <p className="muted">
              Расчёт {calculation.valid ? 'успешен' : 'с ошибками'} ·{' '}
              {new Date(calculation.created_at).toLocaleString('ru-RU')}
            </p>
          )}
        </section>
        <MembersPanel projectId={projectId} />
        <ReviewPanel
          projectId={projectId}
          status={(project?.status ?? 'draft') as ProjectStatus}
          onStatusChange={handleStatusChange}
        />
        <ApprovalsPanel projectId={projectId} configurationId={calculation?.configuration_id} />
        <VersionsPanel projectId={projectId} />
        <AuditPanel projectId={projectId} />
        <CommentsPanel projectId={projectId} />
        <AssistantPanel config={config} rates={rates} />
      </div>

      {calculation && <ResultPanel snapshot={calculation.result} onApplyVariation={applyVariation} activeVariantId={activeVariantId} />}
      {previewCalculation && (
        <section className="panel">
          <h2 className="panel__title">Превью варианта</h2>
          <ResultPanel snapshot={previewCalculation.result} onApplyVariation={applyVariation} activeVariantId={activeVariantId} />
          <div className="row row--actions">
            <button className="btn btn--primary" onClick={applyPreview} disabled={busy}>
              Применить вариант
            </button>
            <button className="btn" onClick={closePreview} disabled={busy}>
              Отмена
            </button>
          </div>
        </section>
      )}
    </div>
  )
}

interface ConfigFormProps {
  fields: ConfigForm
  errors: FieldErrors
  onChange: (key: keyof ConfigForm, value: string | boolean) => void
}

const configLabels: Record<string, string> = {
  widthMM: 'Ширина марша, мм',
  heightMM: 'Высота подъёма H, мм',
  stepHeightMM: 'Целевая высота ступени, мм',
  stringerThicknessMM: 'Толщина косоура, мм',
  stepThicknessMM: 'Толщина ступени, мм',
  riser: 'Подступень',
  clearanceMM: 'Зазор, мм',
  railingHeightMM: 'Высота ограждения, мм',
  comfortStepMM: 'Шаг комфорта S, мм',
  landingWidthMM: 'Ширина площадки Wp, мм',
  landingDepthMM: 'Глубина площадки, мм',
  roomWidthMM: 'Ширина помещения (X) — направление марша (длина + свободное место), мм',
  roomLengthMM: 'Длина помещения (Y) — ширина марша, мм',
  approachSpaceMM: 'Свободное пространство перед маршем, мм',
  lowerStepCountMM: 'Ступеней нижнего марша (n1)',
  outerRadiusMM: 'Наружный радиус R, мм',
  railing: 'Перила',
  railingLower: 'Перила: первый марш',
  railingLanding: 'Перила: площадка',
  railingUpper: 'Перила: второй марш',
  direction: 'Направление поворота',
  spiralDirection: 'Направление спирали',
}

function labelOf(key: keyof ConfigForm): string {
  return configLabels[key] ?? key
}

// adminSelectOptions — выпадающие списки административной формы для полей
// с дискретными значениями (перила/направления).
const adminSelectOptions = (key: keyof ConfigForm) => {
  switch (key) {
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

function ConfigForm({ fields, errors, onChange }: ConfigFormProps) {
  const visible = flightFields[fields.flight]
  return (
    <div className="config-grid">
      <div className="field">
        <label className="field__label" htmlFor="cfg-flight">
          Тип марша
        </label>
        <select
          id="cfg-flight"
          className="field__input"
          value={fields.flight}
          onChange={(e) => onChange('flight', e.target.value)}
        >
          {flightOptions.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </div>
      <div className="field">
        <label className="field__label" htmlFor="cfg-material">
          Материал
        </label>
        <select
          id="cfg-material"
          className="field__input"
          value={fields.material}
          onChange={(e) => onChange('material', e.target.value)}
        >
          {materialOptions.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </div>
      {visible.map((key) => {
        const rule = rulesFor(key, fields.material)
        const error = errors[key]
        const hint = rule && (rule.hint ?? (rule.min || rule.max ? rangeText(rule) : undefined))
        const options = adminSelectOptions(key)
        return (
          <div className="field" key={key}>
            <label className="field__label" htmlFor={`cfg-${key}`}>
              {labelOf(key)}
              {hint && <span className="field__hint"> {hint}</span>}
            </label>
            {options ? (
              <select
                id={`cfg-${key}`}
                className="field__input"
                value={fields[key] as string}
                onChange={(e) => onChange(key, e.target.value)}
              >
                {options.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            ) : (
              <input
                id={`cfg-${key}`}
                className={`field__input${error ? ' field__input--invalid' : ''}`}
                type="number"
                inputMode="decimal"
                value={fields[key] as string}
                onChange={(e) => onChange(key, e.target.value)}
              />
            )}
            {error && <p className="field__error">{error}</p>}
          </div>
        )
      })}
      {fields.flight === 'spiral' && (
        <div className="field">
          <label className="field__label" htmlFor="cfg-railing-auto">
            {labelOf('railing')}
          </label>
          <input
            id="cfg-railing-auto"
            className="field__input"
            readOnly
            value={railingLabel(railingForSpiral(fields.spiralDirection))}
          />
          <p className="field__hint">Авто: по направлению спирали</p>
        </div>
      )}
      <div className="field">
        <label className="field__label" htmlFor="cfg-riser">
          {labelOf('riser')}
        </label>
        <label className="config-checkbox">
          <input
            id="cfg-riser"
            type="checkbox"
            checked={fields.riser}
            onChange={(e) => onChange('riser', e.target.checked)}
          />
          <span>{fields.riser ? 'Есть' : 'Нет'}</span>
        </label>
      </div>
    </div>
  )
}

function rangeText(rule: { min?: number; max?: number }): string {
  if (rule.min !== undefined && rule.max !== undefined) return `${rule.min}–${rule.max}`
  if (rule.min !== undefined) return `≥${rule.min}`
  if (rule.max !== undefined) return `≤${rule.max}`
  return ''
}

interface RatesFormProps {
  rates: RatesForm
  onChange: (key: keyof RatesForm, value: string) => void
}

const rateFields: Array<{ key: keyof RatesForm; label: string; placeholder: string }> = [
  { key: 'steel', label: 'Сталь STEEL-S235, ₽/кг', placeholder: 'дефолт' },
  { key: 'alum', label: 'Алюминий ALUM-5083, ₽/кг', placeholder: 'дефолт' },
  { key: 'wood', label: 'Дуб WOOD-OAK, ₽/кг', placeholder: 'дефолт' },
  { key: 'machinePerHour', label: 'Станок, ₽/час', placeholder: 'дефолт' },
  { key: 'laborPerHour', label: 'Труд, ₽/час', placeholder: 'дефолт' },
  { key: 'overheadPct', label: 'Накладные, %', placeholder: 'дефолт' },
  { key: 'marginPct', label: 'Маржа, %', placeholder: 'дефолт' },
  { key: 'discountPct', label: 'Скидка, %', placeholder: 'дефолт' },
  { key: 'taxPct', label: 'НДС, %', placeholder: 'дефолт' },
]

function RatesFormSection({ rates, onChange }: RatesFormProps) {
  return (
    <div className="rates">
      <h3 className="panel__sub">Ставки цены (опционально)</h3>
      <p className="muted">Пустые поля — значения по умолчанию.</p>
      <div className="config-grid">
        {rateFields.map((f) => (
          <div className="field" key={f.key}>
            <label className="field__label" htmlFor={`rate-${f.key}`}>
              {f.label}
            </label>
            <input
              id={`rate-${f.key}`}
              className="field__input"
              type="number"
              inputMode="decimal"
              placeholder={f.placeholder}
              value={rates[f.key]}
              onChange={(e) => onChange(f.key, e.target.value)}
            />
          </div>
        ))}
      </div>
    </div>
  )
}
