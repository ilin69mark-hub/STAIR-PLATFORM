// AssistantPanel — интерфейс AI-ассистентов (Phase D, EDR-0036): четыре
// вкладки (design/engineering/manufacturing/pricing). Запрос уходит на
// POST /api/v1/assistant/{kind} с текущей конфигурацией проекта и ставками;
// ответ — детерминированная структура Response плюс комментарий модели.
import { useState } from 'react'
import { projectsApi } from '../api/projects'
import type {
  AssistantKind,
  AssistantPriority,
  AssistantResult,
} from '@shared/types'
import { ApiError } from '@shared/types'
import { toRequest, toRatesRequest, type ConfigForm, type RatesForm } from '@shared/config'
import { fmt } from '@shared/format'

interface Props {
  config: ConfigForm
  rates: RatesForm
}

const tabs: Array<{ kind: AssistantKind; label: string; hint: string }> = [
  { kind: 'design', label: 'Проектирование', hint: 'подбор марша под требования' },
  { kind: 'engineering', label: 'Инжиниринг', hint: 'проверка норм СТАНДАРТ' },
  { kind: 'manufacturing', label: 'Производство', hint: 'анализ готовности и раскроя' },
  { kind: 'pricing', label: 'Ценообразование', hint: 'структура цены и маржа' },
]

const priorities: Array<{ value: AssistantPriority; label: string }> = [
  { value: 'price', label: 'По цене для клиента' },
  { value: 'cost', label: 'По себестоимости' },
  { value: 'material', label: 'По стоимости материала' },
  { value: 'comfort', label: 'По шагу комфорта' },
]

export function AssistantPanel({ config, rates }: Props) {
  const [kind, setKind] = useState<AssistantKind>('design')
  const [priority, setPriority] = useState<AssistantPriority>('price')
  const [result, setResult] = useState<AssistantResult | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const ask = async () => {
    setBusy(true)
    setError(null)
    setResult(null)
    try {
      const body: Record<string, unknown> = toRequest(config)
      const ratesReq = toRatesRequest(rates)
      if (ratesReq) body.rates = ratesReq
      if (kind === 'design') body.priority = priority
      const res = await projectsApi.assistant(kind, body)
      setResult(res)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось получить рекомендацию')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel">
      <h2 className="panel__title">AI-ассистент</h2>
      <p className="muted">
        Четыре эксперта анализируют текущую конфигурацию через существующие конвейеры:
        конструирование, инжиниринг, производство и цены.
      </p>

      <div className="row row--actions assistant-tabs" role="tablist" aria-label="Ассистенты">
        {tabs.map((t) => (
          <button
            key={t.kind}
            role="tab"
            aria-selected={kind === t.kind}
            className={`btn${kind === t.kind ? ' btn--primary' : ''}`}
            onClick={() => setKind(t.kind)}
          >
            {t.label}
            <span className="assistant-tab__hint"> {t.hint}</span>
          </button>
        ))}
      </div>

      <div className="row row--actions">
        {kind === 'design' && (
          <label className="field">
            <span className="field__label">Приоритет рекомендации</span>
            <select
              className="field__input field__input--inline"
              aria-label="Приоритет ассистента"
              value={priority}
              onChange={(e) => setPriority(e.target.value as AssistantPriority)}
            >
              {priorities.map((p) => (
                <option key={p.value} value={p.value}>
                  {p.label}
                </option>
              ))}
            </select>
          </label>
        )}
        <button className="btn btn--primary" onClick={ask} disabled={busy}>
          {busy ? 'Анализ…' : 'Спросить ассистента'}
        </button>
      </div>

      {error && <div className="alert alert--error">{error}</div>}
      {result && <AssistantResultView result={result} />}
    </section>
  )
}

function AssistantResultView({ result }: { result: AssistantResult }) {
  const r = result.response
  return (
    <div className="assistant-result">
      <blockquote className="assistant-result__recommendation">{r.recommendation}</blockquote>
      <p className="muted">
        Рейтинг рекомендации:{' '}
        <strong>{fmt.pct(r.rating ?? 0)}</strong> · ассистент «{result.kind}»
      </p>

      {r.findings && r.findings.length > 0 && (
        <>
          <h3 className="panel__sub">Замечания</h3>
          <ul className="comment-list">
            {r.findings.map((f, i) => (
              <li
                className={`comment comment--${f.severity}`}
                key={i}
              >
                <span className={`badge badge--${f.severity}`}>{f.severity}</span>{' '}
                {f.element && (
                  <span className="badge badge--element">{f.element}</span>
                )}{' '}
                {f.message}
              </li>
            ))}
          </ul>
        </>
      )}

      {r.suggestions && r.suggestions.length > 0 && (
        <>
          <h3 className="panel__sub">Действия</h3>
          <ol className="assistant-list">
            {r.suggestions.map((s, i) => (
              <li key={i}>
                {s.message}
                {s.rationale && <span className="muted"> — {s.rationale}</span>}
              </li>
            ))}
          </ol>
        </>
      )}

      {r.alternatives && r.alternatives.length > 0 && (
        <>
          <h3 className="panel__sub">Альтернативные варианты</h3>
          <ul className="assistant-list">
            {r.alternatives.map((a, i) => (
              <li key={i}>
                <strong>{a.title}</strong> · {fmt.pct(a.rating)}
                {a.reason && <span className="muted"> — {a.reason}</span>}
              </li>
            ))}
          </ul>
        </>
      )}

      {r.tradeoffs && r.tradeoffs.length > 0 && (
        <>
          <h3 className="panel__sub">Компромиссы</h3>
          <ul className="assistant-list">
            {r.tradeoffs.map((t, i) => (
              <li key={i}>{t}</li>
            ))}
          </ul>
        </>
      )}

      {r.notes && r.notes.length > 0 && (
        <>
          <h3 className="panel__sub">Заметки</h3>
          <ul className="assistant-list">
            {r.notes.map((n, i) => (
              <li key={i}>{n}</li>
            ))}
          </ul>
        </>
      )}

      {result.commentary && (
        <>
          <h3 className="panel__sub">Комментарий модели</h3>
          <p className="assistant-result__commentary">{result.commentary}</p>
        </>
      )}
    </div>
  )
}