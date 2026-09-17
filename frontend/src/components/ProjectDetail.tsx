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
import { generateProposalPdf } from '../lib/proposal'
import { MembersPanel } from './MembersPanel'
import { CommentsPanel } from './CommentsPanel'
import { ReviewPanel } from './ReviewPanel'
import { ApprovalsPanel } from './ApprovalsPanel'
import { VersionsPanel } from './VersionsPanel'
import { AuditPanel } from './AuditPanel'
import { AssistantPanel } from './AssistantPanel'
import { logAction } from '@shared/api/audit'
import type { ProjectStatus } from '@shared/types'
import { statusMeta } from '../lib/status'
import { useScrollSpy } from '../lib/useScrollSpy'

interface Props {
  projectId: string
  onBack: () => void
  onChanged: () => void
}

// SECTIONS — оглавление длинной страницы проекта (sticky TOC в сайдбаре).
const SECTIONS = [
  { id: 'params', label: 'Параметры' },
  { id: 'result', label: 'Результат' },
  { id: 'team', label: 'Участники' },
  { id: 'review', label: 'Ревью' },
  { id: 'approvals', label: 'Утверждение' },
  { id: 'versions', label: 'Версии' },
  { id: 'audit', label: 'Аудит' },
  { id: 'comments', label: 'Обсуждение' },
  { id: 'assistant', label: 'AI-ассистент' },
]

const SECTION_IDS = SECTIONS.map((s) => s.id)

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

  // Очистка debounce таймера при unmount.
  useEffect(() => {
    return () => {
      if (configChangeTimer.current) window.clearTimeout(configChangeTimer.current)
    }
  }, [])

  // Scroll-spy для липкого оглавления: подсвечиваем текущую секцию.
  const activeSection = useScrollSpy(SECTION_IDS, 'params')

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

  // handleCalculate — отправляет расчёт и сохраняет результат в проекте.
  // Возвращает true при успешном сохранении.
  const handleCalculate = async (): Promise<boolean> => {
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
      return true
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось выполнить расчёт')
      return false
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

  const [proposalBusy, setProposalBusy] = useState(false)
  const handleProposal = async () => {
    if (!calculation || !project) return
    setProposalBusy(true)
    try {
      await generateProposalPdf(project, calculation.result)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось сформировать КП')
    } finally {
      setProposalBusy(false)
    }
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
  // Превью очищаем только ПОСЛЕ успешного расчёта: при ошибке пользователь
  // должен сохранить текущий превью-результат.
  const applyPreview = async () => {
    const ok = await handleCalculate()
    if (!ok) return
    setPreviewCalculation(null)
    setActiveVariantId(undefined)
  }

  const closePreview = () => {
    setPreviewCalculation(null)
    setActiveVariantId(undefined)
  }

  const handleStatusChange = (status: ProjectStatus) => {
    setProject((p) => (p ? { ...p, status } : p))
  }

  const meta = statusMeta(project?.status ?? 'draft')

  return (
    <div className="page">
      <header className="page__header page__header--stacked">
        <div className="page__header-title">
          <button className="btn btn--ghost" onClick={onBack}>
            ← Проекты
          </button>
          <h1 className="page__title" aria-label={project?.name}>
            {project?.name ?? 'Проект'}
            {project && <span className={`badge ${meta.badge}`}>{meta.label}</span>}
          </h1>
          <p className="page__subtitle">
            {project
              ? `создан ${new Date(project.created_at).toLocaleDateString('ru-RU')} · обновлён ${new Date(project.updated_at).toLocaleDateString('ru-RU')}`
              : 'Загрузка…'}
          </p>
        </div>
        <div className="page__header-actions">
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
          <button
            className="btn btn--accent"
            onClick={handleCalculate}
            disabled={busy || hasHardErrors || project?.status === 'in_review'}
            title={project?.status === 'in_review' ? 'Расчёт заморожен до решения владельца' : undefined}
          >
            {busy ? 'Расчёт…' : 'Рассчитать'}
          </button>
          <button
            className="btn"
            onClick={handleOptimize}
            disabled={busy || hasHardErrors || project?.status === 'in_review'}
            title={project?.status === 'in_review' ? 'Оптимизация заморожена до решения владельца' : undefined}
          >
            {busy ? 'Поиск…' : 'Оптимизировать'}
          </button>
          <button className="btn btn--accent" onClick={handleProposal} disabled={!calculation || proposalBusy}>
            {proposalBusy ? 'Формируем…' : 'Коммерческое предложение (PDF)'}
          </button>
        </div>
      </header>

      {error && <div className="alert alert--error">{error}</div>}
      {savedAt && (
        <div className="alert alert--ok">
          Расчёт сохранён · {savedAt.toLocaleTimeString('ru-RU')}
        </div>
      )}

      <div className="page__content">
        <div className="page__main">
          {project?.status === 'in_review' && (
            <div className="alert alert--info">Расчёт заморожен — проект на ревью. Дождитесь решения владельца.</div>
          )}
          <section id="params" className="section">
            <div className="panel">
              <h2 className="panel__title">Параметры лестницы</h2>
              <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>Задайте геометрию, материалы и ставки. Каждая калькуляция создаёт новую версию.</p>
              <ConfigForm fields={config} errors={errors} onChange={setField} />
              <RatesFormSection rates={rates} onChange={setRate} />
              {hasHardErrors && (
                <p className="muted">
                  Исправьте нечисловые или пустые поля перед расчётом.
                </p>
              )}
              {optimizeMsg && <p className="muted">{optimizeMsg}</p>}
            </div>
          </section>

          <section id="result" className="section">
            <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>Результат последнего расчёта. Вариации A/B/C — превью без сохранения, «Применить» создаёт версию.</p>
            {calculation && (
              <ResultPanel
                snapshot={calculation.result}
                onApplyVariation={applyVariation}
                activeVariantId={activeVariantId}
              />
            )}
            {previewCalculation && (
              <section className="panel">
                <h2 className="panel__title">Превью варианта</h2>
                <ResultPanel
                  snapshot={previewCalculation.result}
                  onApplyVariation={applyVariation}
                  activeVariantId={activeVariantId}
                />
                <div className="row row--actions">
                  <button className="btn btn--accent" onClick={applyPreview} disabled={busy}>
                    Применить вариант
                  </button>
                  <button className="btn" onClick={closePreview} disabled={busy}>
                    Отмена
                  </button>
                </div>
              </section>
            )}
            {!calculation && !previewCalculation && (
              <div className="panel">
                <h2 className="panel__title">Результат расчёта</h2>
                <p className="muted">
                  {hasHardErrors
                    ? 'Исправьте ошибки в параметрах перед расчётом.'
                    : 'Расчёт ещё не выполнен. Заполните параметры и нажмите «Рассчитать».'}
                </p>
              </div>
            )}
          </section>

          <section id="team" className="section">
            <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>Участники: владелец — решает ревью и утверждает, редактор — считает, наблюдатель — смотрит.</p>
            <MembersPanel projectId={projectId} />
          </section>

          <section id="review" className="section">
            <ReviewPanel
              projectId={projectId}
              status={(project?.status ?? 'draft') as ProjectStatus}
              onStatusChange={handleStatusChange}
            />
          </section>

          <section id="approvals" className="section">
            <ApprovalsPanel
              projectId={projectId}
              configurationId={calculation?.configuration_id}
            />
          </section>

          <section id="versions" className="section">
            <VersionsPanel projectId={projectId} />
          </section>

          <section id="audit" className="section">
            <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>Лента событий: кто, когда и что менял — для контроля.</p>
            <AuditPanel projectId={projectId} />
          </section>

          <section id="comments" className="section">
            <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>Обсуждение проекта — комментарии видны всем участникам.</p>
            <CommentsPanel projectId={projectId} />
          </section>

          <section id="assistant" className="section">
            <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>AI-ассистент: подсказки по геометрии, производству и цене.</p>
            <AssistantPanel config={config} rates={rates} />
          </section>
        </div>

        <aside className="page__sidebar">
          <div className="status-card">
            <div className="status-card__header">
              <span className="status-card__label">Статус</span>
              <span className={`badge ${meta.badge}`}>{meta.label}</span>
            </div>
            <div className="status-card__next">{meta.next}</div>
          </div>

          <nav className="toc" aria-label="Разделы проекта">
            <div className="toc__title">Навигация</div>
            <ul className="toc__list">
              {SECTIONS.map((s) => (
                <li key={s.id}>
                  <a
                    className={`toc__link${activeSection === s.id ? ' toc__link--active' : ''}`}
                    href={`#${s.id}`}
                  >
                    {s.label}
                  </a>
                </li>
              ))}
            </ul>
          </nav>
        </aside>
      </div>
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
