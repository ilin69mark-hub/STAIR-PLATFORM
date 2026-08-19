import { useContext, useEffect, useState } from 'react'
import { AuthContext } from '../auth/context'
import { projectsApi } from '../api/projects'
import type { ProjectComment } from '@shared/types'
import { ApiError } from '@shared/types'

interface Props {
  projectId: string
}

export function CommentsPanel({ projectId }: Props) {
  const auth = useContext(AuthContext)
  const user = auth?.user ?? null
  const [comments, setComments] = useState<ProjectComment[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [body, setBody] = useState('')
  const [posting, setPosting] = useState(false)

  const load = () => {
    setLoading(true)
    setError(null)
    projectsApi
      .listComments(projectId)
      .then(setComments)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Не удалось загрузить комментарии'))
      .finally(() => setLoading(false))
  }

  useEffect(load, [projectId])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const text = body.trim()
    if (!text || posting) return
    setPosting(true)
    setError(null)
    try {
      await projectsApi.addComment(projectId, { body: text })
      setBody('')
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось отправить комментарий')
    } finally {
      setPosting(false)
    }
  }

  const canDelete = (c: ProjectComment) => c.author_id === user?.id

  const handleDelete = async (commentID: string) => {
    setError(null)
    try {
      await projectsApi.deleteComment(projectId, commentID)
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось удалить комментарий')
    }
  }

  return (
    <section className="panel">
      <h2 className="panel__title">Обсуждение</h2>
      {error && <div className="alert alert--error">{error}</div>}
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <ul className="comment-list">
          {comments.length === 0 && <p className="muted">Комментариев пока нет.</p>}
          {comments.map((c) => (
            <li className="comment" key={c.id}>
              <div className="comment__meta">
                <span className="comment__author">{c.author_id}</span>
                <span className="comment__date">
                  {new Date(c.created_at).toLocaleString('ru-RU')}
                </span>
              </div>
              <p className="comment__body">{c.body}</p>
              {canDelete(c) && (
                <button
                  className="btn btn--danger btn--sm"
                  onClick={() => handleDelete(c.id)}
                >
                  Удалить
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      <form className="comment-add" onSubmit={handleSubmit}>
        <textarea
          className="field__input"
          rows={2}
          placeholder="Комментарий…"
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
        <button className="btn btn--primary" type="submit" disabled={!body.trim() || posting}>
          {posting ? 'Отправка…' : 'Отправить'}
        </button>
      </form>
    </section>
  )
}