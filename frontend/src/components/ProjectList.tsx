import { useState } from 'react'
import type { Project } from '../api/types'
import { projectsApi } from '../api/projects'
import { ApiError } from '../api/types'

interface Props {
  projects: Project[]
  loading: boolean
  onSelect: (id: string) => void
  onCreated: (p: Project) => void
}

export function ProjectList({ projects, loading, onSelect, onCreated }: Props) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const canCreate = name.trim() !== '' && !creating

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
        <button className="btn btn--primary" onClick={handleCreate} disabled={!canCreate}>
          {creating ? 'Создание…' : 'Создать'}
        </button>
      </section>

      <section className="panel">
        <h2 className="panel__title">Проекты</h2>
        {loading ? (
          <p className="muted">Загрузка…</p>
        ) : projects.length === 0 ? (
          <p className="muted">Проектов пока нет. Создайте первый.</p>
        ) : (
          <ul className="project-list">
            {projects.map((p) => (
              <li key={p.id}>
                <button className="project-card" onClick={() => onSelect(p.id)}>
                  <span className="project-card__name">{p.name}</span>
                  <span className="project-card__meta">
                    {p.status} · обновлён {new Date(p.updated_at).toLocaleString('ru-RU')}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}