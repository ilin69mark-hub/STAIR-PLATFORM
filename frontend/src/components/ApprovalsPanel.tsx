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

  return (
    <section className="panel">
      <h2 className="panel__title">Утверждение конфигурации</h2>
      {error && <div className="alert alert--error">{error}</div>}
      {saved && <div className="alert alert--ok">Конфигурация утверждена.</div>}
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <ul className="comment-list">
          {approvals.length === 0 && (
            <p className="muted">Конфигурация ещё не утверждалась.</p>
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
      {configurationId && (
        <div className="comment-add">
          <textarea
            className="field__input"
            rows={2}
            placeholder="Комментарий (необязательно)…"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
          />
          <button className="btn btn--primary" onClick={handleApprove} disabled={busy}>
            {busy ? 'Утверждение…' : 'Утвердить ревизию'}
          </button>
        </div>
      )}
    </section>
  )
}