import { useContext, useEffect, useState } from 'react'
import { AuthContext } from '../auth/context'
import { projectsApi } from '../api/projects'
import type { ProjectMember, ProjectReview, ProjectStatus } from '../api/types'
import { ApiError } from '../api/types'

interface Props {
  projectId: string
  status: ProjectStatus
  onStatusChange: (status: ProjectStatus) => void
}

const statusLabels: Record<ProjectStatus, string> = {
  draft: 'Черновик',
  in_review: 'На ревью',
  approved: 'Подписано',
  changes_requested: 'Доработка',
}

const decisionLabels: Record<ProjectReview['decision'], string> = {
  requested: 'Запрошено',
  approved: 'Подписано',
  changes_requested: 'Вернули на доработку',
}

export function ReviewPanel({ projectId, status, onStatusChange }: Props) {
  const auth = useContext(AuthContext)
  const user = auth?.user ?? null
  const [reviews, setReviews] = useState<ProjectReview[]>([])
  const [members, setMembers] = useState<ProjectMember[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [comment, setComment] = useState('')
  const [busy, setBusy] = useState(false)

  const current = members.find((m) => m.user_id === user?.id)
  const isOwner = current?.role === 'owner'
  const canRequest = current?.role === 'owner' || current?.role === 'editor'

  const load = () => {
    setLoading(true)
    setError(null)
    Promise.all([projectsApi.listReviews(projectId), projectsApi.listMembers(projectId)])
      .then(([rs, ms]) => {
        setReviews(rs)
        setMembers(ms)
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Не удалось загрузить ревью'))
      .finally(() => setLoading(false))
  }

  useEffect(load, [projectId])

  const pending = reviews.find((r) => r.decision === 'requested')

  const run = async (fn: () => Promise<unknown>, nextStatus: ProjectStatus, errMsg: string) => {
    setBusy(true)
    setError(null)
    try {
      await fn()
      setComment('')
      onStatusChange(nextStatus)
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : errMsg)
    } finally {
      setBusy(false)
    }
  }

  const handleRequest = () =>
    run(
      () => projectsApi.requestReview(projectId, { comment: comment.trim() }),
      'in_review',
      'Не удалось запросить ревью',
    )

  const handleSignOff = () =>
    run(
      () => projectsApi.signOffReview(projectId, pending!.id, { comment: comment.trim() }),
      'approved',
      'Не удалось подписать ревью',
    )

  const handleChanges = () =>
    run(
      () => projectsApi.requestChanges(projectId, pending!.id, { comment: comment.trim() }),
      'changes_requested',
      'Не удалось вернуть на доработку',
    )

  const showActions =
    (status === 'draft' || status === 'changes_requested') && canRequest && !pending

  return (
    <section className="panel">
      <h2 className="panel__title">Ревью</h2>
      <p className="muted">
        Статус: <span className="review-status">{statusLabels[status]}</span>
      </p>
      {error && <div className="alert alert--error">{error}</div>}
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <ul className="comment-list">
          {reviews.length === 0 && <p className="muted">Ревью ещё не запрашивали.</p>}
          {reviews.map((r) => (
            <li className="comment" key={r.id}>
              <div className="comment__meta">
                <span className="comment__author">{decisionLabels[r.decision]}</span>
                <span className="comment__date">
                  {new Date(r.created_at).toLocaleString('ru-RU')}
                  {r.decided_at
                    ? ` · решено ${new Date(r.decided_at).toLocaleString('ru-RU')}`
                    : ''}
                </span>
              </div>
              {r.comment && <p className="comment__body">{r.comment}</p>}
            </li>
          ))}
        </ul>
      )}
      {showActions && (
        <div className="comment-add">
          <textarea
            className="field__input"
            rows={2}
            placeholder="Комментарий (необязательно)…"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
          />
          <button className="btn btn--primary" onClick={handleRequest} disabled={busy}>
            Запросить ревью
          </button>
        </div>
      )}
      {status === 'in_review' && isOwner && pending && (
        <div className="comment-add">
          <textarea
            className="field__input"
            rows={2}
            placeholder="Комментарий (необязательно)…"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
          />
          <div className="row row--actions">
            <button className="btn btn--primary" onClick={handleSignOff} disabled={busy}>
              Подписать
            </button>
            <button className="btn" onClick={handleChanges} disabled={busy}>
              Вернуть на доработку
            </button>
          </div>
        </div>
      )}
      {status === 'in_review' && !(isOwner && pending) && (
        <p className="muted">Ожидание решения владельца проекта.</p>
      )}
    </section>
  )
}