import { useState } from 'react'
import type { Project } from '@shared/types'
import { projectsApi } from '../api/projects'
import { ApiError } from '@shared/types'
import { statusMeta } from '../lib/status'

interface Props {
  projects: Project[]
  loading: boolean
  currentUserId: string
  onSelect: (id: string) => void
  onCreated: (p: Project) => void
}

export function ProjectList({
  projects,
  loading,
  currentUserId,
  onSelect,
  onCreated,
}: Props) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const canCreate = name.trim() !== '' && !creating

  const owned = projects.filter((p) => p.owner_id === currentUserId)
  const shared = projects.filter((p) => p.owner_id !== currentUserId)

  const renderProjects = (list: Project[], empty: string) => (
    list.length === 0 ? (
      <p className="muted">{empty}</p>
    ) : (
      <ul className="project-list">
        {list.map((p) => {
          const meta = statusMeta(p.status)
          return (
            <li key={p.id}>
              <button
                className="project-card"
                style={{ ['--status-color' as string]: meta.color }}
                onClick={() => onSelect(p.id)}
              >
                <span className="project-card__row">
                  <span className="project-card__name">{p.name}</span>
                  <span className={`badge ${meta.badge}`}>{meta.label}</span>
                </span>
                {p.description && <span className="project-card__meta">{p.description}</span>}
                <span className="project-card__meta">
                  обновлён {new Date(p.updated_at).toLocaleString('ru-RU')}
                </span>
              </button>
            </li>
          )
        })}
      </ul>
    )
  )

  const handleCreate = async () => {
    if (!canCreate) return
    setCreating(true)
    setError(null)
    try {
      const p = await projectsApi.create({ name: name.trim(), description: description.trim() })
      setName('')
      setDescription('')
      onCreated(p)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось создать проект')
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="projects">
      <section className="panel">
        <h2 className="panel__title">Создать проект</h2>
        {error && <div className="alert alert--error">{error}</div>}
        <div className="field">
          <label className="field__label" htmlFor="project-name">
            Название *
          </label>
          <input
            id="project-name"
            className="field__input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Лестница на второй этаж"
          />
        </div>
        <div className="field">
          <label className="field__label" htmlFor="project-description">
            Описание
          </label>
          <textarea
            id="project-description"
            className="field__input"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Заказ, адрес, комментарий"
            rows={2}
          />
        </div>
        <button className="btn btn--accent" onClick={handleCreate} disabled={!canCreate}>
          {creating ? 'Создание…' : 'Создать проект'}
        </button>
      </section>

      <section className="panel">
        <h2 className="panel__title">Мои проекты</h2>
        {loading ? (
          <p className="muted">Загрузка…</p>
        ) : (
          renderProjects(owned, 'Проектов пока нет. Создайте первый.')
        )}
      </section>

      {!loading && shared.length > 0 && (
        <section className="panel">
          <h2 className="panel__title">Доступные мне</h2>
          {renderProjects(shared, '')}
        </section>
      )}
    </div>
  )
}