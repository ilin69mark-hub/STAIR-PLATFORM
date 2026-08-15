import { useCallback, useEffect, useState } from 'react'
import { adminApi } from '../api/admin'
import { analyticsApi } from '../api/analytics'
import type {
  AdminOverview,
  AdminPolicy,
  AdminUser,
  ApiKey,
  UsageGranularity,
  UsageReport,
} from '../api/types'
import { ApiError } from '../api/types'

interface Props {
  currentUserId: string
  onBack: () => void
}

const roleLabels: Record<string, string> = { user: 'Пользователь', admin: 'Администратор' }
const statusLabels: Record<string, string> = { active: 'Активен', disabled: 'Заблокирован' }

const emptyPolicy: AdminPolicy = {
  min_password_length: 8,
  require_number: false,
  require_upper: false,
  session_ttl_seconds: 86400,
  login_rate_limit_per_min: 10,
}

export function AdminPanel({ currentUserId, onBack }: Props) {
  const [overview, setOverview] = useState<AdminOverview | null>(null)
  const [users, setUsers] = useState<AdminUser[]>([])
  const [policy, setPolicy] = useState<AdminPolicy>(emptyPolicy)
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const [keyName, setKeyName] = useState('')
  const [keyScopes, setKeyScopes] = useState('users.list')
  const [newToken, setNewToken] = useState<string | null>(null)

  const [usage, setUsage] = useState<UsageReport | null>(null)
  const [usageGranularity, setUsageGranularity] = useState<UsageGranularity>('day')
  const [usageLoading, setUsageLoading] = useState(false)

  const loadUsage = useCallback(async (granularity: UsageGranularity) => {
    setUsageLoading(true)
    try {
      const now = new Date()
      const from = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
      const rep = await analyticsApi.usage({
        from: from.toISOString().slice(0, 10),
        to: now.toISOString().slice(0, 10),
        granularity,
      })
      setUsage(rep)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить аналитику')
    } finally {
      setUsageLoading(false)
    }
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [ov, us, pl, ks] = await Promise.all([
        adminApi.overview(),
        adminApi.listUsers(),
        adminApi.getSettings(),
        adminApi.listApiKeys(),
      ])
      setOverview(ov)
      setUsers(us)
      setPolicy(pl)
      setKeys(ks)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить панель администратора')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
    void loadUsage('day')
  }, [load, loadUsage])

  const clearNotice = () => setNotice(null)

  const handleUpdateUser = async (id: string, body: { role?: 'user' | 'admin'; status?: 'active' | 'disabled' }) => {
    setError(null)
    clearNotice()
    try {
      await adminApi.updateUser(id, body)
      setNotice('Пользователь обновлён')
      void load()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось обновить пользователя')
    }
  }

  const handleSavePolicy = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    clearNotice()
    try {
      await adminApi.updateSettings(policy)
      setNotice('Политика безопасности сохранена')
      void load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сохранить политику')
    }
  }

  const setPolicyField = (key: keyof AdminPolicy, value: string | boolean | number) =>
    setPolicy((p) => ({ ...p, [key]: value as never }))

  const handleCreateKey = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    clearNotice()
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
      void load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось создать API-ключ')
    }
  }

  const handleRevokeKey = async (id: string) => {
    setError(null)
    clearNotice()
    try {
      await adminApi.revokeApiKey(id)
      setNotice('API-ключ отозван')
      void load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось отозвать ключ')
    }
  }

  const handleExport = (scope: string, format: 'json' | 'csv') => {
    window.location.href = adminApi.exportUrl(scope, format)
  }

  return (
    <div className="page">
      <header className="page__header">
        <button className="btn btn--ghost" onClick={onBack}>
          ← Проекты
        </button>
        <div>
          <h1 className="page__title">Администрирование</h1>
          <p className="page__subtitle">Enterprise Controls · EDR-0016</p>
        </div>
      </header>

      {error && <div className="alert alert--error">{error}</div>}
      {notice && <div className="alert alert--ok">{notice}</div>}
      {newToken && (
        <div className="alert alert--warn">
          <strong>Сохраните токен сейчас</strong> — он показывается один раз:
          <code className="token-code">{newToken}</code>
        </div>
      )}

      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <>
          <section className="panel">
            <h2 className="panel__title">Обзор</h2>
            {overview && (
              <dl className="kv">
                <div>
                  <dt>Пользователи</dt>
                  <dd>{overview.users}</dd>
                </div>
                <div>
                  <dt>Активных / заблокированных</dt>
                  <dd>
                    {overview.active_users} / {overview.disabled_users}
                  </dd>
                </div>
                <div>
                  <dt>Администраторы</dt>
                  <dd>{overview.admins}</dd>
                </div>
                <div>
                  <dt>Проекты</dt>
                  <dd>{overview.projects}</dd>
                </div>
                <div>
                  <dt>API-ключи (активных)</dt>
                  <dd>
                    {overview.active_api_keys} / {overview.total_api_keys}
                  </dd>
                </div>
              </dl>
            )}
          </section>

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
                    onChange={(e) => handleUpdateUser(u.id, { role: e.target.value as 'user' | 'admin' })}
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
                      onChange={(e) =>
                        handleUpdateUser(u.id, { status: e.target.value as 'active' | 'disabled' })
                      }
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

          <section className="panel">
            <h2 className="panel__title">Политика безопасности</h2>
            <form onSubmit={handleSavePolicy}>
              <div className="config-grid">
                <div className="field">
                  <label className="field__label" htmlFor="min-len">
                    Минимальная длина пароля
                  </label>
                  <input
                    id="min-len"
                    className="field__input"
                    type="number"
                    min={8}
                    max={128}
                    value={policy.min_password_length}
                    onChange={(e) => setPolicyField('min_password_length', Number(e.target.value))}
                  />
                </div>
                <div className="field">
                  <label className="field__label" htmlFor="session-ttl">
                    TTL сессии (секунды)
                  </label>
                  <input
                    id="session-ttl"
                    className="field__input"
                    type="number"
                    min={300}
                    max={86400}
                    value={policy.session_ttl_seconds}
                    onChange={(e) => setPolicyField('session_ttl_seconds', Number(e.target.value))}
                  />
                </div>
                <div className="field">
                  <label className="field__label" htmlFor="rate-limit">
                    Лимит входа (попыток/мин)
                  </label>
                  <input
                    id="rate-limit"
                    className="field__input"
                    type="number"
                    min={1}
                    max={1000}
                    value={policy.login_rate_limit_per_min}
                    onChange={(e) => setPolicyField('login_rate_limit_per_min', Number(e.target.value))}
                  />
                </div>
                <div className="field">
                  <label className="field__label">
                    <input
                      type="checkbox"
                      checked={policy.require_number}
                      onChange={(e) => setPolicyField('require_number', e.target.checked)}
                    />{' '}
                    Требовать цифру в пароле
                  </label>
                </div>
                <div className="field">
                  <label className="field__label">
                    <input
                      type="checkbox"
                      checked={policy.require_upper}
                      onChange={(e) => setPolicyField('require_upper', e.target.checked)}
                    />{' '}
                    Требовать заглавную букву
                  </label>
                </div>
              </div>
              <div className="row--actions">
                <button className="btn btn--primary" type="submit">
                  Сохранить политику
                </button>
              </div>
            </form>
          </section>

          <section className="panel">
            <h2 className="panel__title">Экспорт данных</h2>
            <p className="muted">Выгрузка данных tenant в JSON или CSV (только для администратора).</p>
            <div className="row--actions">
              {(['users', 'projects', 'audit'] as const).map((scope) => (
                <span key={scope}>
                  <button className="btn" onClick={() => handleExport(scope, 'json')}>
                    {scope} · JSON
                  </button>{' '}
                  <button className="btn" onClick={() => handleExport(scope, 'csv')}>
                    {scope} · CSV
                  </button>
                </span>
              ))}
            </div>
          </section>

          <section className="panel">
            <h2 className="panel__title">Аналитика использования</h2>
            <p className="muted">Активность tenant за выбранное окно (право analytics.read, EDR-0028).</p>
            <div className="row--actions">
              {(['day', 'week', 'month'] as UsageGranularity[]).map((g) => (
                <button
                  key={g}
                  className={g === usageGranularity ? 'btn btn--primary' : 'btn'}
                  onClick={() => {
                    setUsageGranularity(g)
                    void loadUsage(g)
                  }}
                >
                  {g === 'day' ? 'День' : g === 'week' ? 'Неделя' : 'Месяц'}
                </button>
              ))}
            </div>
            {usageLoading ? (
              <p className="muted">Загрузка…</p>
            ) : usage ? (
              <>
                <dl className="kv">
                  <div>
                    <dt>Пользователи / активные</dt>
                    <dd>
                      {usage.totals.users} / {usage.totals.active_users}
                    </dd>
                  </div>
                  <div>
                    <dt>Проекты</dt>
                    <dd>{usage.totals.projects}</dd>
                  </div>
                  <div>
                    <dt>Расчёты</dt>
                    <dd>{usage.totals.calculations}</dd>
                  </div>
                  <div>
                    <dt>Входы / экспорты / оплаты</dt>
                    <dd>
                      {usage.totals.logins} / {usage.totals.exports} / {usage.totals.payments}
                    </dd>
                  </div>
                </dl>
                <p className="muted">
                  Период: {usage.from} — {usage.to}
                </p>
                <table className="table">
                  <thead>
                    <tr>
                      <th>Дата</th>
                      <th>Входы</th>
                      <th>Активные</th>
                      <th>Проекты</th>
                      <th>Расчёты</th>
                      <th>Экспорты</th>
                      <th>Оплаты</th>
                    </tr>
                  </thead>
                  <tbody>
                    {usage.series.map((p) => (
                      <tr key={p.bucket}>
                        <td>{p.bucket}</td>
                        <td>{p.logins}</td>
                        <td>{p.active_users}</td>
                        <td>{p.projects_created}</td>
                        <td>{p.calculations}</td>
                        <td>{p.exports}</td>
                        <td>{p.payments}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            ) : null}
          </section>

          <section className="panel">
            <h2 className="panel__title">API-ключи</h2>
            <p className="muted">
              Service-токены для интеграций. Токен показывается один раз при создании.
            </p>
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
                      <button className="btn btn--danger btn--sm" onClick={() => handleRevokeKey(k.id)}>
                        Отозвать
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            )}
            <form className="member-add" onSubmit={handleCreateKey}>
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
        </>
      )}
    </div>
  )
}
