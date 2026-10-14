import React, { useState } from 'react'
import type { AdminUser } from '@shared/types'
import type { UpdateUserRequest, UserDeletionPreview } from '../../api/admin'

// Четыре роли: manager/sales/owner/admin. Подписи на русском, значения —
// те же, что ждёт бэкенд (auth.ParseRole, EDR-0041 §3.2).
const roleLabels: Record<NonNullable<UpdateUserRequest['role']>, string> = {
  user: 'Менеджер по продажам',
  sales: 'Руководитель продаж',
  owner: 'Владелец',
  admin: 'Администратор (полный доступ)',
}
const statusLabels: Record<string, string> = { active: 'Активен', disabled: 'Заблокирован' }

// Кому можно передать проекты: любой активный сотрудник, кроме того, кого
// удаляют. Владелец стоит первым — чаще всего проекты забирает он.
function recipients(users: AdminUser[], excludeId: string): AdminUser[] {
  return users
    .filter((u) => u.id !== excludeId && u.status === 'active')
    .sort((a, b) => {
      const owner = (u: AdminUser) => (u.role === 'owner' ? 0 : 1)
      return owner(a) - owner(b) || a.email.localeCompare(b.email)
    })
}

// Подпись получателя: имя, почта и роль. Почта нужна обязательно — по одному
// имени двух людей с одинаковым «Иван» не различить, а это выбор, к которому
// уедут чужие проекты.
function recipientLabel(u: AdminUser): string {
  const role = roleLabels[u.role as keyof typeof roleLabels]
  const base = u.name ? `${u.name} — ${u.email}` : u.email
  return u.role === 'user' ? base : `${base} · ${role}`
}

interface Props {
  users: AdminUser[]
  currentUserId: string
  onUpdate: (id: string, body: UpdateUserRequest) => void
  /** Право staff.delete: без него кнопки удаления нет вовсе. */
  canDelete: boolean
  /** Цифры по сотруднику до удаления (сервер их считает). */
  onPreviewDelete: (id: string) => Promise<UserDeletionPreview>
  onDelete: (id: string, transferTo: string) => Promise<void>
}

export const UsersPanel = React.memo(function UsersPanel({
  users,
  currentUserId,
  onUpdate,
  canDelete,
  onPreviewDelete,
  onDelete,
}: Props) {
  // Кто сейчас открыт на удаление и какие цифры по нему показаны.
  const [deleting, setDeleting] = useState<string | null>(null)
  const [preview, setPreview] = useState<UserDeletionPreview | null>(null)
  const [transferTo, setTransferTo] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const close = () => {
    setDeleting(null)
    setPreview(null)
    setTransferTo('')
    setError('')
  }

  const openDelete = async (id: string) => {
    setError('')
    setDeleting(id)
    setPreview(null)
    setTransferTo('')
    try {
      setPreview(await onPreviewDelete(id))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось загрузить данные об удалении')
    }
  }

  const confirmDelete = async (id: string) => {
    if (!transferTo) {
      setError('Выберите, кому передать проекты сотрудника')
      return
    }
    setBusy(true)
    setError('')
    try {
      await onDelete(id, transferTo)
      close()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось удалить сотрудника')
    } finally {
      setBusy(false)
    }
  }

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
              onChange={(e) => onUpdate(u.id, { role: e.target.value as UpdateUserRequest['role'] })}
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
            {/* Себя удалить нельзя (правило сервера), поэтому кнопки нет. */}
            {canDelete && u.id !== currentUserId && (
              <button
                type="button"
                className="btn btn--danger"
                onClick={() => void openDelete(u.id)}
              >
                Удалить
              </button>
            )}
            {canDelete && deleting === u.id && (
              <div className="member-delete" role="group" aria-label={`Удаление ${u.email}`}>
                {!preview ? (
                  <p className="muted">Считаем, что останется…</p>
                ) : (
                  <>
                    <p className="member-delete__numbers">
                      {preview.projects > 0 ? (
                        <>
                          Перейдут другому человеку: <strong>{preview.projects}</strong>{' '}
                          проект(ов).{' '}
                        </>
                      ) : (
                        <>У сотрудника нет своих проектов.{' '}</>
                      )}
                      Останутся в истории: расчётов — {preview.calculations}, комментариев —{' '}
                      {preview.comments}, заказов — {preview.orders}.
                    </p>
                    <label className="field">
                      <span className="field__label">Кому передать проекты</span>
                      <select
                        className="field__input"
                        aria-label={`Передать проекты ${u.email}`}
                        value={transferTo}
                        onChange={(e) => setTransferTo(e.target.value)}
                      >
                        <option value="">— выберите —</option>
                        {recipients(users, u.id).map((r) => (
                          <option key={r.id} value={r.id}>
                            {recipientLabel(r)}
                          </option>
                        ))}
                      </select>
                    </label>
                    {error && (
                      <p className="alert alert--error" role="alert">
                        {error}
                      </p>
                    )}
                    <div className="row--actions">
                      <button
                        type="button"
                        className="btn btn--danger"
                        disabled={busy || !transferTo}
                        onClick={() => void confirmDelete(u.id)}
                      >
                        {busy ? 'Удаляем…' : 'Передать и удалить'}
                      </button>
                      <button type="button" className="btn btn--ghost" onClick={close} disabled={busy}>
                        Отмена
                      </button>
                    </div>
                    <p className="muted">
                      Человек пропадёт из списка и больше не сможет войти. История его
                      работы останется.
                    </p>
                  </>
                )}
              </div>
            )}
          </li>
        ))}
      </ul>
    </section>
  )
})
