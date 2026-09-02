import React, { useState } from 'react'
import type { ApiKey } from '@shared/types'
import { ApiError } from '@shared/types'
import { adminApi } from '../../api/admin'

interface Props {
  keys: ApiKey[]
  onRefresh: () => void
  onError: (msg: string) => void
  onNotice: (msg: string) => void
}

export const ApiKeysPanel = React.memo(function ApiKeysPanel({ keys, onRefresh, onError, onNotice }: Props) {
  const [keyName, setKeyName] = useState('')
  const [keyScopes, setKeyScopes] = useState('users.list')
  const [newToken, setNewToken] = useState<string | null>(null)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    onError('')
    setNewToken(null)
    try {
      const scopes = keyScopes
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
      const resp = await adminApi.createApiKey({ name: keyName.trim(), scopes })
      setKeyName('')
      setKeyScopes('users.list')
      if (resp.token) setNewToken(resp.token)
      onRefresh()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : 'Не удалось создать API-ключ')
    }
  }

  const handleRevoke = async (id: string) => {
    onError('')
    try {
      await adminApi.revokeApiKey(id)
      onNotice('API-ключ отозван')
      onRefresh()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : 'Не удалось отозвать ключ')
    }
  }

  return (
    <section className="panel">
      <h2 className="panel__title">API-ключи</h2>
      <p className="muted">
        Service-токены для интеграций. Токен показывается один раз при создании.
      </p>
      {newToken && (
        <div className="alert alert--warn">
          <strong>Сохраните токен сейчас</strong> — он показывается один раз:
          <code className="token-code">{newToken}</code>
        </div>
      )}
      {keys.length === 0 ? (
        <p className="muted">API-ключей пока нет.</p>
      ) : (
        <ul className="comment-list">
          {keys.map((k) => (
            <li className="comment" key={k.id}>
              <div className="comment__meta">
                <span className="comment__author">
                  {k.name} · {k.scopes.join(', ')}
                </span>
                <span className="comment__date">
                  {k.revoked_at ? 'Отозван' : 'Активен'}
                </span>
              </div>
              <p className="comment__body">
                <span className="muted">
                  создан {new Date(k.created_at).toLocaleString('ru-RU')}
                  {k.last_used_at && ` · использован ${new Date(k.last_used_at).toLocaleString('ru-RU')}`}
                </span>
              </p>
              {!k.revoked_at && (
                <button className="btn btn--danger btn--sm" onClick={() => handleRevoke(k.id)}>
                  Отозвать
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      <form className="member-add" onSubmit={handleCreate}>
        <input
          className="field__input"
          placeholder="Имя ключа (например, CI)"
          value={keyName}
          onChange={(e) => setKeyName(e.target.value)}
          required
        />
        <input
          className="field__input"
          placeholder="Scopes через запятую (users.list)"
          value={keyScopes}
          onChange={(e) => setKeyScopes(e.target.value)}
        />
        <button className="btn btn--primary" type="submit">
          Создать ключ
        </button>
      </form>
    </section>
  )
})
