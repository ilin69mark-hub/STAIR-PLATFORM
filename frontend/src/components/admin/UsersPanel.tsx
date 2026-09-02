import React from 'react'
import type { AdminUser } from '@shared/types'

const roleLabels: Record<string, string> = { user: 'Пользователь', admin: 'Администратор' }
const statusLabels: Record<string, string> = { active: 'Активен', disabled: 'Заблокирован' }

interface Props {
  users: AdminUser[]
  currentUserId: string
  onUpdate: (id: string, body: { role?: 'user' | 'admin'; status?: 'active' | 'disabled' }) => void
}

export const UsersPanel = React.memo(function UsersPanel({ users, currentUserId, onUpdate }: Props) {
  return (
    <section className="panel">
      <h2 className="panel__title">Пользователи</h2>
      <ul className="member-list">
        {users.map((u) => (
          <li className="member" key={u.id}>
            <span className="member__id">
              <strong>{u.name || u.email}</strong>
              <span className="muted">
                {' '}
                · {u.email} · {u.id}
              </span>
            </span>
            <select
              className="member__role"
              aria-label={`Роль ${u.email}`}
              value={u.role}
              disabled={u.id === currentUserId}
              onChange={(e) => onUpdate(u.id, { role: e.target.value as 'user' | 'admin' })}
            >
              {Object.entries(roleLabels).map(([v, l]) => (
                <option key={v} value={v}>
                  {l}
                </option>
              ))}
            </select>
            {u.id !== currentUserId ? (
              <select
                className="member__role"
                aria-label={`Статус ${u.email}`}
                value={u.status}
                onChange={(e) => onUpdate(u.id, { status: e.target.value as 'active' | 'disabled' })}
              >
                {Object.entries(statusLabels).map(([v, l]) => (
                  <option key={v} value={v}>
                    {l}
                  </option>
                ))}
              </select>
            ) : (
              <span className="member__role member__role--static">
                {statusLabels[u.status]} · это вы
              </span>
            )}
          </li>
        ))}
      </ul>
    </section>
  )
})
