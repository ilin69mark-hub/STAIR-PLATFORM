import { useEffect, useMemo, useRef, useState } from 'react'
import type { Calculation, OptimizeTarget, Project } from '@shared/types'
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
import { ApiError } from '@shared/types'
import { projectsApi } from '../api/projects'
import {
  defaultConfig,
  defaultRates,
  flightOptions,
  flightFields,
  materialOptions,
  directionOptions,
  fixingBaseOptions,
  railingOptions,
  rulesFor,
  toRequest,
  toRatesRequest,
  validateForm,
  type ConfigForm,
  type FieldErrors,
  type RatesForm,
} from '@shared/config'
import { ResultPanel } from './ResultPanel'
import { PipelineLiveLine } from './PipelineLiveLine'
import { usePipelineRoom } from '../lib/usePipelineRoom'
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
import { storeApi, type FastenerPrice } from '../api/store'
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
  // Прайс крепежа для списка «по объекту» (MNT-0001). Грузится один раз на
  // проект и НЕ мешает расчёту: если прайс недоступен, блок крепления
  // показывает подсказку, а расчёт уходит без extra_fasteners.
  const [fasteners, setFasteners] = useState<FastenerPrice[] | undefined>(undefined)
  const [fastenersError, setFastenersError] = useState<string | undefined>(undefined)
  // Живая валидация при вводе (S-P5): серверные блокировки (угол/проступь,
  // вписываемость), невидимые локальной проверке формы.
  const [liveBlocking, setLiveBlocking] = useState(false)
  const [liveIssues, setLiveIssues] = useState<LiveIssue[]>([])
  const [liveFieldErrors, setLiveFieldErrors] = useState<FieldErrors>({})
  const liveValidator = useRef(new LiveValidator())
  const applyLive = (v: LiveValidation | null) => {
    const withIssues = !!v && v.blocking && v.issues.length > 0
    setLiveBlocking(withIssues)
    setLiveIssues(withIssues ? v.issues : [])
    setLiveFieldErrors(withIssues ? v.fieldErrors : {})
  }
  const [optimizeTarget, setOptimizeTarget] = useState<OptimizeTarget>('price')
  // S-132b: live-уведомления конвейера (pipeline:<configuration_id>,
  // протокол S-132a). Хук сам подписывается/отписывается; без WebSocket
  // в окружении — тихо (realtimeClient no-op).
  const { notices: pipelineNotices, live: pipelineLive } = usePipelineRoom(
    calculation?.configuration_id ?? null,
  )
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
    const timerRef = configChangeTimer
    const validatorRef = liveValidator
    return () => {
      if (timerRef.current) window.clearTimeout(timerRef.current)
      validatorRef.current.cancel()
    }
  }, [])

  // Scroll-spy для липкого оглавления: подсвечиваем текущую секцию.
  const activeSection = useScrollSpy(SECTION_IDS, 'params')

  const setField = (key: keyof ConfigForm, value: string | boolean | string[]) => {
    const next = { ...config, [key]: value }
    setConfig(next)
    // Аудит изменения поля (debounce 600 мс, best-effort).
    if (configChangeTimer.current) window.clearTimeout(configChangeTimer.current)
    configChangeTimer.current = window.setTimeout(() => {
      logAction({
        action: 'stair.config_changed',
        resource_type: 'stair',
        resource_id: projectId,
        detail: JSON.stringify({ field: key }),
      }, 'admin')
    }, 600)
    // Живая валидация при вводе: дебаунс + дедуп по конфигу; при локальных
    // ошибках формы сервер не дёргаем (они уже подсвечены).
    if (Object.keys(validateForm(next)).length === 0) {
      liveValidator.current.schedule({
        key: configKey(next),
        fetch: () => projectsApi.validateStair(toRequest(next)),
        onChange: applyLive,
      })
    }
  }

  // Прайс крепежа живёт в админке магазина, а не в проекте: он одинаков для
  // всех лестниц и меняется владельцем. Один запрос на проект, ошибка не
  // блокирует расчёт — список просто останется пустым.
  useEffect(() => {
    let alive = true
    storeApi
      .fasteners()
      .then((list) => {
        if (alive) setFasteners(list)
      })
      .catch((e) => {
        if (alive) setFastenersError(e instanceof Error ? e.message : String(e))
      })
    return () => {
      alive = false
    }
  }, [projectId])

  const setRate = (key: keyof RatesForm, value: string) =>
    setRates((r) => ({ ...r, [key]: value }))

  // handleCalculate — отправляет расчёт и сохраняет результат в проекте.
  // Возвращает true при успешном сохранении.
  const handleCalculate = async (source?: ConfigForm): Promise<boolean> => {
    setBusy(true)
    setError(null)
    setOptimizeMsg(null)
    try {
      const src = source ?? config
      const body = { ...toRequest(src) }
      const ratesReq = toRatesRequest(rates)
      if (ratesReq) body.rates = ratesReq
      const calc = await projectsApi.calculate(projectId, body)
      setCalculation(calc)
      setSavedAt(new Date())
      liveValidator.current.invalidate(configKey(src))
      applyLive(null)
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
    }, 'admin')
    setBusy(true)
    setError(null)
    try {
      // DOM-005 (2026-09-26): превью уходило на бэкенд без проверки формы, и
      // пустая ширина превращалась в width_mm: 0 (Number('') === 0 в toRequest)
      // — пользователь получал «Ширина марша должна быть положительной»
      // от сервера вместо подсветки поля. Проверяем локально.
      const localErrors = validateForm(next)
      if (Object.keys(localErrors).length > 0) {
        setError('Исправьте поля формы перед применением варианта')
        return
      }
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
  // «Применить» из баннера живой валидации (S-P5): применяем предложенный
  // вариант советника/вариацию к форме и запускаем обычный расчёт проекта.
  const applyLiveSuggestion = (si: LiveSuggestion) => {
    const next = liveApplySuggestion(config, si)
    setConfig(next)
    liveValidator.current.invalidate(configKey(next))
    void handleCalculate(next)
    logAction({
      action: 'stair.live_suggestion_applied',
      resource_type: 'stair',
      resource_id: projectId,
      detail: JSON.stringify({ step_height_mm: si.stepHeightMm, step_count: si.stepCount }),
    }, 'admin')
  }
  const applyLiveVariation = (v: LiveVariation) => {
    const next = liveApplyVariation(config, v)
    setConfig(next)
    liveValidator.current.invalidate(configKey(next))
    // DOM-005 (2026-09-26): «применить» уходило в расчёт без проверки формы.
    // Пустая ширина сериализуется в width_mm: 0, и пользователь получал
    // серверное «Ширина марша должна быть положительной» вместо подсветки
    // поля. Проверяем форму до запроса.
    const localErrors = validateForm(next)
    if (Object.keys(localErrors).length > 0) {
      setError('Исправьте поля формы перед применением варианта')
      return
    }
    void handleCalculate(next)
    logAction({
      action: 'stair.live_variation_applied',
      resource_type: 'stair',
      resource_id: projectId,
      detail: JSON.stringify({ id: v.id, title: v.title }),
    }, 'admin')
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

  // Экспорт JSON-снапшота проекта (GET /api/v1/projects/{id}/export).
  // Скачивание клиентское (fetch + Blob): переход на export-URL не передаёт
  // origin-заголовок администратора, и API находит не ту session-cookie (401).
  const handleExport = async () => {
    try {
      const res = await fetch(projectsApi.exportUrl(projectId), {
        credentials: 'include',
        headers: { 'X-App-Origin': 'admin' },
      })
      if (!res.ok) {
        setError('Не удалось экспортировать проект')
        return
      }
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'project-export.json'
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    } catch {
      setError('Не удалось экспортировать проект')
    }
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
            onClick={() => void handleCalculate()}
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
          <button className="btn" onClick={handleExport} disabled={!calculation}>
            Экспорт JSON
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
              <ConfigForm
                fields={config}
                errors={errors}
                liveErrors={liveFieldErrors}
                onChange={setField}
                fasteners={fasteners}
                fastenersError={fastenersError}
              />
              {liveBlocking && liveIssues.length > 0 && (
                <div className="alert alert--warn" role="alert" style={{ marginTop: 12 }}>
                  {liveIssues.map((it, idx) => (
                    <div key={idx} style={{ marginBottom: idx < liveIssues.length - 1 ? 6 : 0 }}>
                      <strong>{it.param ? `${it.param}: ` : ''}{it.guide ?? it.message}</strong>
                      {it.fix ? ` (${it.fix})` : ''}{' '}
                      {it.suggestions && it.suggestions.length > 0 && (
                        <button
                          type="button"
                          className="btn"
                          onClick={() => applyLiveSuggestion(it.suggestions![0])}
                        >
                          Применить: {it.suggestions[0].stepCount} ступ. · h {it.suggestions[0].stepHeightMm.toFixed(1)}
                        </button>
                      )}
                      {it.variations && it.variations.length > 0 && (
                        <button
                          type="button"
                          className="btn"
                          onClick={() => {
                            const v = it.variations![0]
                            applyLiveVariation(v)
                          }}
                        >
                          Применить вариант A
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              )}
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
<>
<PipelineLiveLine live={pipelineLive} notices={pipelineNotices} />
<ResultPanel
                  snapshot={calculation.result}
                  onApplyVariation={applyVariation}
                  activeVariantId={activeVariantId}
                  heightMM={Number(config.heightMM) || undefined}
                />
</>
            )}
            {previewCalculation && (
              <section className="panel">
                <h2 className="panel__title">Превью варианта</h2>
                <ResultPanel
                  snapshot={previewCalculation.result}
                  onApplyVariation={applyVariation}
                  activeVariantId={activeVariantId}
                  heightMM={Number(config.heightMM) || undefined}
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
            <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>
              Обсуждение проекта — комментарии видны всем участникам.
            </p>
            <MeasurementSnippet config={config} calculation={calculation} />
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
  /** Ошибки живой (серверной) валидации при вводе: подсвечивают поля, у
   * которых локально всё в порядке, но бэкенд находит блокировку (S-P5). */
  liveErrors?: FieldErrors
  // value — строка, булево или (для extraFasteners) список позиций:
  // единственное поле формы, которое не строка.
  onChange: (key: keyof ConfigForm, value: string | boolean | string[]) => void
  /** Прайс крепежа для списка «по объекту». Не подгружен — блок
   * показывается с подсказкой, а не ломает форму. */
  fasteners?: FastenerPrice[]
  /** Сообщение о неудачной загрузке прайса крепежа. */
  fastenersError?: string
}

const configLabels: Record<string, string> = {
  widthMM: 'Ширина марша, мм',
  heightMM: 'Высота подъёма H, мм',
  stepHeightMM: 'Целевая высота ступени, мм',
  stringerThicknessMM: 'Лист косоура, мм',
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
  floorFixing: 'Крепление в пол',
  ceilingFixing: 'Крепление в перекрытие',
  extraFasteners: 'Дополнительный крепёж по объекту',
  sealantLayerMM: 'Слой герметика',
  extraSealantCartridges: 'Доп. картриджи герметика',
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
    case 'floorFixing':
    case 'ceilingFixing':
      return fixingBaseOptions
    default:
      return null
  }
}

function ConfigForm({
  fields,
  errors,
  liveErrors,
  onChange,
  fasteners,
  fastenersError,
}: ConfigFormProps) {
  // Неизвестный тип марша из старых данных: форма не должна падать —
  // показываем поля прямого марша и предупреждение.
  const visible = flightFields[fields.flight] ?? flightFields.straight
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
        const liveError = liveErrors?.[key]
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
                className={`field__input${error || liveError ? ' field__input--invalid' : ''}`}
                type="number"
                inputMode="decimal"
                value={fields[key] as string}
                onChange={(e) => onChange(key, e.target.value)}
              />
            )}
            {error && <p className="field__error">{error}</p>}
            {!error && liveError && <p className="field__error">{liveError}</p>}
          </div>
        )
      })}
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
      <FixingSection
        fields={fields}
        fasteners={fasteners}
        fastenersError={fastenersError}
        onFixingChange={(key, value) => onChange(key, value)}
        onExtraChange={(next) => onChange('extraFasteners', next)}
        onExtraField={(key, value) => onChange(key, value)}
      />
    </div>
  )
}

// FixingSection — блок «Крепление» (CONF-FIXING, решение владельца
// 2026-10-08): основание для пола и для перекрытия плюс раскрывающийся
// список крепежа по объекту.
//
// Отдельным блоком, а не ещё одной строкой в общей сетке, потому что здесь
// три разных вещи: два выбора основания (бетон/дерево) и список позиций с
// ценами. В общей сетке выпадающие списки и список позиций оказались бы в
// одной колонке с размерами, и список на 40 позиций растянул бы форму на
// весь экран.
//
// Что видит человек и зачем: крепёж считается по отверстиям пяток сам, эти
// два выбора говорят движку, ДОБАВЛЯТЬ ли дюбель (в бетон — да, в дерево —
// нет), а список нужен для случаев вроде «пол неровный, берём 12
// дополнительных дюбелей».
function FixingSection({
  fields,
  fasteners,
  fastenersError,
  onFixingChange,
  onExtraChange,
  onExtraField,
}: {
  fields: ConfigForm
  fasteners?: FastenerPrice[]
  fastenersError?: string
  onFixingChange: (key: 'floorFixing' | 'ceilingFixing', value: string) => void
  onExtraChange: (next: string[]) => void
  onExtraField: (key: 'sealantLayerMM' | 'extraSealantCartridges', value: string) => void
}) {
  const [open, setOpen] = useState(false)
  const selected = fields.extraFasteners ?? []
  // Поиск по названию И по коду: монтажник знает «10х60» и вводит это, а
  // «Глухарь 10х60» набирать руками дольше.
  const [query, setQuery] = useState('')
  const q = query.trim().toLowerCase()
  // Сортировка по размеру, а не по коду: по коду PLUG-10x100 идёт раньше
  // PLUG-10x50, и монтажник, ищущий «10х60», попадает в середину списка.
  const visible = (fasteners ?? [])
    .filter((f) => !q || f.name_ru.toLowerCase().includes(q) || f.code.toLowerCase().includes(q))
    .slice()
    .sort((a, b) => a.diameter_mm - b.diameter_mm || a.length_mm - b.length_mm)
  const selectedSum = (fasteners ?? [])
    .filter((f) => selected.includes(f.code))
    .reduce((sum, f) => sum + f.price_per_pc_rub, 0)

  const toggle = (code: string) => {
    onExtraChange(
      selected.includes(code) ? selected.filter((c) => c !== code) : [...selected, code],
    )
  }

  return (
    <fieldset className="fixing">
      <legend className="field__label">Крепление</legend>

      <div className="config-grid">
        <div className="field">
          <label className="field__label" htmlFor="cfg-floorFixing">
            Крепление в пол
          </label>
          <select
            id="cfg-floorFixing"
            className="field__input"
            value={fields.floorFixing ?? 'concrete'}
            onChange={(e) => onFixingChange('floorFixing', e.target.value)}
          >
            {fixingBaseOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          <p className="field__hint">
            В бетон глухарь 10х60 идёт через дюбель, в дерево — без него.
          </p>
        </div>

        <div className="field">
          <label className="field__label" htmlFor="cfg-sealantLayerMM">
            Слой герметика, мм
          </label>
          <input
            id="cfg-sealantLayerMM"
            className="field__input"
            type="number"
            inputMode="decimal"
            min={0}
            step={0.5}
            value={fields.sealantLayerMM ?? '1'}
            onChange={(e) => onExtraField('sealantLayerMM', e.target.value)}
          />
          <p className="field__hint">
            Расход = площадь швов × толщина, плюс запас 15% на брак. Площадь
            считается по геометрии обоймы выреза и торца косоура.
          </p>
        </div>

        <div className="field">
          <label className="field__label" htmlFor="cfg-extraSealantCartridges">
            Доп. картриджи герметика, шт
          </label>
          <input
            id="cfg-extraSealantCartridges"
            className="field__input"
            type="number"
            inputMode="numeric"
            min={0}
            step={1}
            value={fields.extraSealantCartridges ?? ''}
            placeholder="не нужно"
            onChange={(e) => onExtraField('extraSealantCartridges', e.target.value)}
          />
          <p className="field__hint">
            Добавляется сверх автоматического расхода, без запаса — когда швы
            шире нормы или пол неровный.
          </p>
        </div>

        <div className="field">
          <label className="field__label" htmlFor="cfg-ceilingFixing">
            Крепление в перекрытие
          </label>
          <select
            id="cfg-ceilingFixing"
            className="field__input"
            value={fields.ceilingFixing ?? 'concrete'}
            onChange={(e) => onFixingChange('ceilingFixing', e.target.value)}
          >
            {fixingBaseOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          <p className="field__hint">
            Пятка выхода на второй этаж. Если перекрытие деревянное — ставьте
            «Дерево».
          </p>
        </div>
      </div>

      <button
        type="button"
        className="fixing__toggle"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        {open ? 'Скрыть' : 'Показать'} крепёж по объекту
        {selected.length > 0 && ` (${selected.length})`}
      </button>

      {open && (
        <div className="fixing__list">
          <p className="field__hint">
            Глухари и дюбели сверх автоматического крепежа по отверстиям пяток.
            Отмеченные позиции добавятся в смету по одной штуке.
            {selected.length > 0 && ` Сейчас в смете: ${selected.length} поз., ${selectedSum} ₽.`}
          </p>

          {fastenersError && <p className="field__error">{fastenersError}</p>}

          {!fasteners && !fastenersError && <p className="field__hint">Загрузка прайса крепежа…</p>}

          {fasteners && visible.length === 0 && (
            <p className="field__hint">
              {q ? `Ничего не найдено по запросу «${query}».` : 'Прайс крепежа пуст.'}
            </p>
          )}

          {fasteners && visible.length > 0 && (
            <>
              <input
                className="field__input"
                type="search"
                aria-label="Поиск крепежа"
                placeholder="Поиск: 10х60, дюбель, глухарь…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
              <div className="fixing__rows">
                {visible.map((f) => (
                  <label key={f.code} className="fixing__row">
                    <input
                      type="checkbox"
                      checked={selected.includes(f.code)}
                      onChange={() => toggle(f.code)}
                    />
                    <span className="fixing__name">{f.name_ru}</span>
                    <code className="fixing__code">{f.code}</code>
                    <span className="fixing__price">{f.price_per_pc_rub} ₽/шт</span>
                  </label>
                ))}
              </div>
            </>
          )}
        </div>
      )}
    </fieldset>
  )
}

// MEASUREMENT_ROWS — строки блока замера: подпись поля и ключ конфигурации.
//
// Порядок и состав совпадают с порядком полей в форме: тот, кто заполнял
// замер, идёт сверху вниз по форме и видит те же строки.
const MEASUREMENT_ROWS: Array<{ key: keyof ConfigForm; label: string }> = [
  { key: 'widthMM', label: 'Ширина марша' },
  { key: 'heightMM', label: 'Высота подъёма H' },
  { key: 'stepHeightMM', label: 'Целевая высота ступени' },
  { key: 'stringerThicknessMM', label: 'Лист косоура' },
  { key: 'stepThicknessMM', label: 'Толщина ступени' },
  { key: 'clearanceMM', label: 'Просвет' },
  { key: 'railingHeightMM', label: 'Высота ограждения' },
  { key: 'roomWidthMM', label: 'Ширина помещения (X)' },
  { key: 'roomLengthMM', label: 'Длина помещения (Y)' },
  { key: 'approachSpaceMM', label: 'Свободное место перед маршем' },
]

// MeasurementSnippet — блок «Замер» перед обсуждением: все размеры текстом,
// кнопка «Скопировать» и расхождение с последним расчётом.
//
// Зачем он. После выезда на замер размеры вносятся в форму, а дальше их надо
// ещё раз переписать в смету для клиента, в письмо и в закладку монтажника.
// Раньше это означало диктовку вслух или скриншот телефона. Теперь размеры
// копируются одной строкой.
//
// Расхождение показывается рядом не для красоты: миллиметр в ширине или
// высоте меняет число ступеней, а значит и цену. Молча пересчитать и
// показать новую сумму значило бы спрятать то, что человек должен увидеть.
function MeasurementSnippet({
  config,
  calculation,
}: {
  config: ConfigForm
  calculation: Calculation | null
}) {
  const [copied, setCopied] = useState(false)

  const lines = MEASUREMENT_ROWS.map(({ key, label }) => {
    const raw = config[key]
    const value = typeof raw === 'string' ? raw.trim() : ''
    return { label, value }
  }).filter((l) => l.value !== '')

  const text = [
    `Замер ${new Date().toLocaleDateString('ru-RU')}`,
    ...lines.map((l) => `${l.label}: ${l.value} мм`),
    `Крепление в пол: ${config.floorFixing === 'wood' ? 'дерево' : 'бетон'}`,
    `Крепление в перекрытие: ${config.ceilingFixing === 'wood' ? 'дерево' : 'бетон'}`,
  ].join('\n')

  // Расхождение с последним расчётом: только по размерам, которые расчёт
  // действительно применял. Пустой расчёт — сравнивать не с чем.
  const diffs = useMemo(() => {
    if (!calculation) return []
    const out: string[] = []
    for (const { key, label } of MEASUREMENT_ROWS) {
      const now = String(config[key] ?? '').trim()
      if (now === '') continue
      const before = snapshotSize(calculation, key)
      // before — число, now — строка из формы. Сравниваем через Number,
      // иначе «900» никогда не равно 900 и блок писал бы «расхождение»
      // на каждом поле, где расхождения нет.
      const nowNum = Number(now)
      if (before === undefined || !Number.isFinite(nowNum) || before === nowNum) continue
      const delta = nowNum - before
      if (!Number.isFinite(delta) || delta === 0) continue
      out.push(`${label}: ${before} → ${now} мм (${delta > 0 ? '+' : ''}${delta})`)
    }
    return out
  }, [calculation, config])

  if (lines.length === 0) return null

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2500)
    } catch {
      // Буфер обмена недоступен (нет прав, http без localhost). Текст всё
      // равно виден на экране и выделяется вручную.
      setCopied(false)
    }
  }

  return (
    <div className="measure">
      <div className="measure__head">
        <span className="measure__title">Замер</span>
        <button type="button" className="fixing__toggle" onClick={() => void copy()}>
          {copied ? 'Скопировано' : 'Скопировать размеры'}
        </button>
      </div>
      <dl className="measure__list">
        {lines.map((l) => (
          <div key={l.label} className="measure__row">
            <dt>{l.label}</dt>
            <dd>{l.value} мм</dd>
          </div>
        ))}
      </dl>
      {diffs.length > 0 && (
        <div className="measure__diff">
          <strong>Расхождение с последним расчётом:</strong>
          {diffs.map((d) => (
            <div key={d}>{d}</div>
          ))}
          <div className="measure__diff-hint">
            Изменился размер — пересчитайте проект, чтобы цена соответствовала замеру.
          </div>
        </div>
      )}
    </div>
  )
}

// snapshotSize — размер из сохранённого расчёта по ключу формы.
//
// Снапшот хранит результат движка, а не форму, поэтому точного общего
// соответствия нет: берёмся только за те поля, которые снапшот действительно
// содержит. Раньше читались `result.flight.Width/Height/Clearance`, которых
// там нет — ширина лежит в корне снапшота, а высоты подъёма и просвета в нём
// просто нет, — и блок расхождения молча не показывался НИКОГДА.
//
// Нет такого поля — возвращаем undefined, и расхождение по нему не
// показывается: лучше не показать, чем показать ерунду.
function snapshotSize(calc: Calculation, key: keyof ConfigForm): number | undefined {
  const r = calc.result
  if (!r) return undefined
  switch (key) {
    case 'widthMM':
      return r.width
    // Толщина косоура ЗДЕСЬ СПЕЦИАЛЬНО НЕ СРАВНИВАЕТСЯ. Расчёт применяет
    // норматив листового косоура и поднимает присланные 8 мм до 12 мм
    // (строка «до 4.0 м»), тогда как в форме остаётся 8. Сравнение давало
    // «Лист косоура: 12 → 8 (−4)» на КАЖДОМ расчёте — то есть всегда ложь,
    // а блок расхождения перестал бы читаться вообще.
    //
    // Правильный источник для этого поля — колонка ревизии
    // stair_configurations.stringer_thickness_mm (там лежит ВВОД, а не
    // норматив). Когда появится ручка загрузки ревизии, сравнение переедет
    // на неё.
    case 'stepThicknessMM':
      return r.step_thickness
    case 'railingHeightMM':
      return r.railing_height
    case 'stepHeightMM':
      return r.flight?.StepHeight
    default:
      return undefined
  }
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
  { key: 'wood', label: 'Дуб WOOD-OAK, ₽/кг', placeholder: 'дефолт' },
  { key: 'walnut', label: 'Орех WOOD-WALNUT, ₽/кг', placeholder: 'дефолт' },
  { key: 'ash', label: 'Ясень WOOD-ASH, ₽/кг', placeholder: 'дефолт' },
  { key: 'soft', label: 'Сосна WOOD-SOFT, ₽/кг', placeholder: 'дефолт' },
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
