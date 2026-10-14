import { useContext, useEffect, useState } from 'react'
import { AuthContext } from '../auth/context'
import { projectsApi } from '../api/projects'
import type { ProjectMember, ProjectRole } from '@shared/types'
import { ApiError } from '@shared/types'

interface Props {
  projectId: string
}

const roleLabels: Record<ProjectRole, string> = {
  owner: 'Владелец',
  editor: 'Редактор',
  viewer: 'Наблюдатель',
}

/** Как показать участника: имя и почта, иначе — идентификатор. */
function memberLabel(m: ProjectMember): string {
  if (m.name && m.email) return `${m.name} — ${m.email}`
  if (m.email) return m.email
  if (m.name) return m.name
  return m.user_id
}

export function MembersPanel({ projectId }: Props) {
  const auth = useContext(AuthContext)
  const user = auth?.user ?? null
  const [members, setMembers] = useState<ProjectMember[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [addEmail, setAddEmail] = useState('')
  const [addRole, setAddRole] = useState<ProjectRole>('viewer')
  // Передача владения: кому и что подтверждаем.
  const [transferTo, setTransferTo] = useState('')
  const [transferring, setTransferring] = useState(false)

  const current = members.find((m) => m.user_id === user?.id)
  const canManage = current?.role === 'owner'
  // Передать можно только участнику проекта: новый владелец появляется здесь
  // же, в списке участников, а не «откуда-то ещё».
  const others = members.filter((m) => m.role !== 'owner')

  const load = () => {
    setLoading(true)
    setError(null)
    projectsApi
      .listMembers(projectId)
      .then(setMembers)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Не удалось загрузить участников'))
      .finally(() => setLoading(false))
  }

  useEffect(load, [projectId])

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await projectsApi.addMember(projectId, { email: addEmail.trim(), role: addRole })
      setAddEmail('')
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось пригласить участника')
    }
  }

  const handleRole = async (m: ProjectMember, role: ProjectRole) => {
    setError(null)
    try {
      await projectsApi.updateMemberRole(projectId, m.user_id, role)
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось изменить роль')
    }
  }

  const handleTransfer = async (toUserId: string) => {
    setError(null)
    setTransferring(true)
    try {
      await projectsApi.transferOwnership(projectId, toUserId)
      setTransferTo('')
      load()
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : 'Не удалось передать владение проектом',
      )
    } finally {
      setTransferring(false)
    }
  }

  const handleRemove = async (userId: string) => {
    setError(null)
    try {
      await projectsApi.removeMember(projectId, userId)
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось удалить участника')
    }
  }

  return (
    <section className="panel">
      <h2 className="panel__title">Участники проекта</h2>
      {error && <div className="alert alert--error">{error}</div>}
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <ul className="member-list">
          {members.map((m) => (
            <li className="member" key={m.user_id}>
              <span className="member__id">{memberLabel(m)}</span>
              {canManage && m.role !== 'owner' ? (
                <select
                  className="member__role"
                  value={m.role}
                  onChange={(e) => handleRole(m, e.target.value as ProjectRole)}
                >
                  {(['editor', 'viewer'] as ProjectRole[]).map((r) => (
                    <option key={r} value={r}>
                      {roleLabels[r]}
                    </option>
                  ))}
                </select>
              ) : (
                <span className={`role-badge role-badge--${m.role}`}>{roleLabels[m.role]}</span>
              )}
              {canManage && m.role !== 'owner' && (
                <button
                  className="btn btn--danger btn--sm"
                  onClick={() => handleRemove(m.user_id)}
                >
                  Удалить
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      {canManage && others.length > 0 && (
        <div className="member-transfer" role="group" aria-label="Передать владение проектом">
          <p className="muted">
            Передать владение: прежний владелец станет редактором и сможет уйти из
            проекта сам.
          </p>
          <div className="member-add">
            <select
              className="field__input"
              aria-label="Новый владелец проекта"
              value={transferTo}
              onChange={(e) => setTransferTo(e.target.value)}
            >
              <option value="">— выберите участника —</option>
              {others.map((m) => (
                <option key={m.user_id} value={m.user_id}>
                  {memberLabel(m)} ({roleLabels[m.role].toLowerCase()})
                </option>
              ))}
            </select>
            <button
              className="btn btn--primary"
              disabled={!transferTo || transferring}
              onClick={() => void handleTransfer(transferTo)}
            >
              {transferring ? 'Передаём…' : 'Передать'}
            </button>
          </div>
        </div>
      )}
      {canManage && (
        <form className="member-add" onSubmit={handleAdd}>
          <input
            className="field__input"
            placeholder="Email участника"
            value={addEmail}
            onChange={(e) => setAddEmail(e.target.value)}
            required
          />
          <select
            className="field__input member-add__role"
            value={addRole}
            onChange={(e) => setAddRole(e.target.value as ProjectRole)}
          >
            <option value="viewer">Наблюдатель</option>
            <option value="editor">Редактор</option>
          </select>
          <button className="btn btn--primary" type="submit">
            Добавить
          </button>
        </form>
      )}
    </section>
  )
}