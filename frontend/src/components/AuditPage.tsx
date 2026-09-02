import { useEffect, useState } from 'react'
import { auditApi, actionLabels } from '../api/audit'
import type { AuditEvent } from '@shared/types'
import { ApiError } from '@shared/types'

// AuditPage — глобальный журнал действий tenant (admin). Показывает все
// события аудита, включая клиентские клики (изменение полей, применение
// вариантов/советов) и серверные расчёты.
export function AuditPage({ onBack }: { onBack: () => void }) {
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setLoading(true)
    setError(null)
    auditApi
      .listTenantAudit()
      .then(setEvents)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Не удалось загрузить аудит'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="page">
      <header className="page__header">
        <div>
          <h1 className="page__title">Аудит действий</h1>
          <p className="page__subtitle">Глобальный журнал событий</p>
        </div>
        <div className="page__actions">
          <button className="btn btn--ghost" onClick={onBack}>
            Назад
          </button>
        </div>
      </header>
      {error && <div className="alert alert--error">{error}</div>}
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <ul className="comment-list">
          {events.length === 0 && <p className="muted">Событий аудита пока нет.</p>}
          {events.map((e) => (
            <li className="comment" key={e.id}>
              <div className="comment__meta">
                <span className="comment__author">
                  {actionLabels[e.action] ?? e.action}
                  {e.result !== 'ok' && ` · ${e.result}`}
                </span>
                <span className="comment__date">
                  {new Date(e.created_at).toLocaleString('ru-RU')}
                </span>
              </div>
              <p className="muted">
                {e.actor_id ? `актор ${e.actor_id}` : 'актор —'}
                {e.project_id ? ` · проект ${e.project_id}` : ''}
                {e.ip ? ` · IP ${e.ip}` : ''}
              </p>
              {e.detail && <p className="comment__body">{e.detail}</p>}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
