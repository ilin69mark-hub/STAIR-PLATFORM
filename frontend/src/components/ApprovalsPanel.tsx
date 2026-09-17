import { useEffect, useState } from 'react'
import { projectsApi } from '../api/projects'
import type { ConfigurationApproval } from '@shared/types'
import { ApiError } from '@shared/types'

interface Props {
  projectId: string
  configurationId?: string
}

export function ApprovalsPanel({ projectId, configurationId }: Props) {
  const [approvals, setApprovals] = useState<ConfigurationApproval[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [comment, setComment] = useState('')
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)

  const load = () => {
    setLoading(true)
    setError(null)
    projectsApi
      .listApprovals(projectId)
      .then(setApprovals)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Не удалось загрузить утверждения'))
      .finally(() => setLoading(false))
  }

  useEffect(load, [projectId])

  const handleApprove = async () => {
    if (!configurationId) return
    setBusy(true)
    setError(null)
    try {
      await projectsApi.approveConfiguration(projectId, configurationId, {
        comment: comment.trim(),
      })
      setComment('')
      setSaved(true)
      load()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось утвердить конфигурацию')
    } finally {
      setBusy(false)
    }
  }

  const alreadyApproved = configurationId ? approvals.some((a) => a.configuration_id === configurationId) : false

  return (
    <section className="panel">
      <h2 className="panel__title">Подпись ревизии в производство</h2>
      <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>Фиксирует конкретную ревизию для производства. 1 раз на ревизию, не меняет статус проекта.</p>
      {error && <div className="alert alert--error">{error}</div>}
      {saved && <div className="alert alert--ok">Ревизия утверждена.</div>}
      {alreadyApproved && <div className="alert alert--ok">Эта ревизия уже утверждена.</div>}
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <ul className="comment-list">
          {approvals.length === 0 && (
            <p className="muted">Ревизий ещё не утверждали.</p>
          )}
          {approvals.map((a) => (
            <li className="comment" key={a.id}>
              <div className="comment__meta">
                <span className="comment__author">Утвердил: {a.approved_by}</span>
                <span className="comment__date">
                  {new Date(a.created_at).toLocaleString('ru-RU')}
                </span>
              </div>
              {a.comment && <p className="comment__body">{a.comment}</p>}
            </li>
          ))}
        </ul>
      )}
      {!configurationId ? (
        <p className="muted">Сначала рассчитайте — появится «Утвердить ревизию #{approvals.length + 1}».</p>
      ) : (
        <div className="comment-add">
          <textarea
            className="field__input"
            rows={2}
            placeholder="Комментарий (необязательно)…"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
          />
          <button
            className="btn btn--primary"
            onClick={handleApprove}
            disabled={busy || alreadyApproved}
            title={alreadyApproved ? 'Эта ревизия уже утверждена' : undefined}
          >
            {busy ? 'Утверждение…' : alreadyApproved ? 'Ревизия утверждена' : 'Утвердить ревизию'}
          </button>
        </div>
      )}
    </section>
  )
}