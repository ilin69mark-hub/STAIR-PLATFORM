import { useContext, useEffect, useState } from 'react'
import { AuthContext } from '../auth/context'
import { projectsApi } from '../api/projects'
import type { Configuration, ProjectMember } from '@shared/types'
import { ApiError } from '@shared/types'

interface Props {
  projectId: string
}

const flightLabels: Record<string, string> = {
  straight: 'Прямой',
  l_shape: 'L-образный',
  u_shape: 'П-образный',
  spiral: 'Спиральный',
}

export function VersionsPanel({ projectId }: Props) {
  const auth = useContext(AuthContext)
  const user = auth?.user ?? null
  const [configs, setConfigs] = useState<Configuration[]>([])
  const [members, setMembers] = useState<ProjectMember[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)

  const canRestore = members.some((m) => {
    if (user && m.user_id !== user.id) return false
    return m.role === 'owner' || m.role === 'editor'
  })

  const load = () => {
    setLoading(true)
    setError(null)
    Promise.all([projectsApi.listConfigurations(projectId), projectsApi.listMembers(projectId)])
      .then(([cs, ms]) => {
        setConfigs(cs)
        setMembers(ms)
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Не удалось загрузить версии'))
      .finally(() => setLoading(false))
  }

  useEffect(load, [projectId])

  const handleRestore = async (configurationID: string) => {
    setBusy(true)
    setError(null)
    setSaved(false)
    try {
      await projectsApi.restoreConfiguration(projectId, configurationID)
      setSaved(true)
      load()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось восстановить версию')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel">
      <h2 className="panel__title">Версии</h2>
      {error && <div className="alert alert--error">{error}</div>}
      {saved && <div className="alert alert--ok">Версия восстановлена.</div>}
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <ul className="comment-list">
          {configs.length === 0 && <p className="muted">Версий пока нет.</p>}
          {configs.map((c) => (
            <li className="comment" key={c.id}>
              <div className="comment__meta">
                <span className="comment__author">
                  Версия {c.revision} · {flightLabels[c.flight] ?? c.flight}
                  {c.current ? ' · текущая' : ''}
                </span>
                <span className="comment__date">
                  {new Date(c.created_at).toLocaleString('ru-RU')}
                </span>
              </div>
              <p className="comment__body">
                H {c.height_mm} мм · Ш {c.width_mm} мм · шаг {c.step_height_mm} мм
              </p>
              {!c.current && canRestore && (
                <button
                  className="btn btn--sm"
                  onClick={() => handleRestore(c.id)}
                  disabled={busy}
                >
                  Восстановить
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}