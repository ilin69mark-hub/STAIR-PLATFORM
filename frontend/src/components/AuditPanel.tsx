import { useEffect, useState } from 'react'
import { auditApi, actionLabels } from '../api/audit'
import type { AuditEvent } from '@shared/types'
import { ApiError } from '@shared/types'

interface Props {
  projectId: string
}

export function AuditPanel({ projectId }: Props) {
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = () => {
    setLoading(true)
    setError(null)
    auditApi
      .listProjectAudit(projectId)
      .then(setEvents)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Не удалось загрузить аудит'))
      .finally(() => setLoading(false))
  }

  useEffect(load, [projectId])

  return (
    <section className="panel">
      <h2 className="panel__title">Аудит</h2>
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
              {e.detail && <p className="comment__body">{e.detail}</p>}
              {e.ip && (
                <p className="muted">
                  {e.actor_id ? `актор ${e.actor_id} · ` : ''}IP {e.ip}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
