import { useEffect, useState } from 'react'
import type { Calculation, OptimizeTarget, Project } from '../api/types'
import { ApiError } from '../api/types'
import { projectsApi } from '../api/projects'
import {
  defaultConfig,
  defaultRates,
  flightOptions,
  toRequest,
  toRatesRequest,
  validateForm,
  fieldRules,
  type ConfigForm,
  type FieldErrors,
  type RatesForm,
} from '../lib/config'
import { ResultPanel } from './ResultPanel'
import { MembersPanel } from './MembersPanel'
import { CommentsPanel } from './CommentsPanel'
import { ReviewPanel } from './ReviewPanel'
import { ApprovalsPanel } from './ApprovalsPanel'
import { VersionsPanel } from './VersionsPanel'
import { AuditPanel } from './AuditPanel'
import { AssistantPanel } from './AssistantPanel'
import type { ProjectStatus } from '../api/types'

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

  const setField = (key: keyof ConfigForm, value: string) =>
    setConfig((c) => ({ ...c, [key]: value }))

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

      {calculation && <ResultPanel snapshot={calculation.result} />}
    </div>
  )
}

interface ConfigFormProps {
  fields: ConfigForm
  errors: FieldErrors
  onChange: (key: keyof ConfigForm, value: string) => void
}

const configFields: Array<{ key: keyof ConfigForm; label: string }> = [
  { key: 'widthMM', label: 'Ширина марша, мм' },
  { key: 'heightMM', label: 'Высота подъёма H, мм' },
  { key: 'stepHeightMM', label: 'Целевая высота ступени, мм' },
  { key: 'stringerThicknessMM', label: 'Толщина косоура, мм' },
  { key: 'stepThicknessMM', label: 'Толщина ступени, мм' },
  { key: 'clearanceMM', label: 'Зазор, мм' },
  { key: 'railingHeightMM', label: 'Высота ограждения, мм' },
  { key: 'comfortStepMM', label: 'Шаг комфорта S, мм' },
]

// Поля, специфичные для маршей с площадкой (EDR-0005, EDR-0006).
const landingFields: Array<{ key: keyof ConfigForm; label: string }> = [
  { key: 'landingWidthMM', label: 'Ширина площадки Wp, мм' },
  { key: 'lowerStepCountMM', label: 'Ступеней нижнего марша (n1)' },
]

// Поля, специфичные для спирального марша (EDR-0007).
const spiralFields: Array<{ key: keyof ConfigForm; label: string }> = [
  { key: 'outerRadiusMM', label: 'Наружный радиус R, мм' },
]

function ConfigForm({ fields, errors, onChange }: ConfigFormProps) {
  const withLanding = fields.flight === 'l_shape' || fields.flight === 'u_shape'
  const withSpiral = fields.flight === 'spiral'
  const visible = withLanding
    ? [...configFields, ...landingFields]
    : withSpiral
      ? [...configFields, ...spiralFields]
      : configFields.filter((f) => !landingFields.some((lf) => lf.key === f.key))
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
      {visible.map((f) => {
        const rule = fieldRules[f.key]
        const error = errors[f.key]
        const hint = rule && (rule.hint ?? (rule.min || rule.max ? rangeText(rule) : undefined))
        return (
          <div className="field" key={f.key}>
            <label className="field__label" htmlFor={`cfg-${f.key}`}>
              {f.label}
              {hint && <span className="field__hint"> {hint}</span>}
            </label>
            <input
              id={`cfg-${f.key}`}
              className={`field__input${error ? ' field__input--invalid' : ''}`}
              type="number"
              inputMode="decimal"
              value={fields[f.key]}
              onChange={(e) => onChange(f.key, e.target.value)}
            />
            {error && <p className="field__error">{error}</p>}
          </div>
        )
      })}
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
