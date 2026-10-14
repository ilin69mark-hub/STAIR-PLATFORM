// Раздел «Команда» → Журнал аудита.
//
// Раньше это была отдельная вкладка с собственной шапкой и кнопкой «Назад»
// (AuditPage.tsx). Теперь аудит — раздел плитки, поэтому собственная шапка
// убрана: её рисует общий каркас кабинета, где работает и выход из системы.
import { useCallback, useEffect, useState } from 'react'

import { auditApi } from '../../api/audit'
import type { AuditEvent } from '@shared/types'
import { actionLabels } from '../../api/audit'

export function AuditSection() {
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const reload = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setEvents(await auditApi.listTenantAudit())
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось загрузить журнал аудита')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  return (
    <section>
      <div className="row--actions">
        <button type="button" className="btn" onClick={() => void reload()} disabled={loading}>
          Обновить
        </button>
        {loading && <span className="muted">Загрузка…</span>}
      </div>
      {error ? (
        <p className="alert alert--error" role="alert">
          {error}
        </p>
      ) : !loading && events.length === 0 ? (
        <p className="muted">Событий аудита пока нет.</p>
      ) : (
        <ul className="audit-list">
          {events.map((e) => (
            <li key={e.id} className="audit-list__item">
              <span className="audit-list__action">
                {actionLabels[e.action] ?? e.action}
                {e.result !== 'ok' && (
                  <span className={`badge ${e.result === 'denied' ? 'badge--warning' : 'badge--muted'}`}>
                    {e.result === 'denied' ? 'отказ' : 'ошибка'}
                  </span>
                )}
              </span>
              <time className="muted" dateTime={e.created_at}>
                {new Date(e.created_at).toLocaleString('ru-RU')}
              </time>
              {e.detail && <span className="audit-list__detail">{e.detail}</span>}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}