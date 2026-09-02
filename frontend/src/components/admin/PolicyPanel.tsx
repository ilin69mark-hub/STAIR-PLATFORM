import React from 'react'
import type { AdminPolicy } from '@shared/types'

interface Props {
  policy: AdminPolicy
  onChange: (key: keyof AdminPolicy, value: string | boolean | number) => void
  onSave: (e: React.FormEvent) => void
}

export const PolicyPanel = React.memo(function PolicyPanel({ policy, onChange, onSave }: Props) {
  return (
    <section className="panel">
      <h2 className="panel__title">Политика безопасности</h2>
      <form onSubmit={onSave}>
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
              onChange={(e) => onChange('min_password_length', Number(e.target.value))}
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
              onChange={(e) => onChange('session_ttl_seconds', Number(e.target.value))}
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
              onChange={(e) => onChange('login_rate_limit_per_min', Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="field__label">
              <input
                type="checkbox"
                checked={policy.require_number}
                onChange={(e) => onChange('require_number', e.target.checked)}
              />{' '}
              Требовать цифру в пароле
            </label>
          </div>
          <div className="field">
            <label className="field__label">
              <input
                type="checkbox"
                checked={policy.require_upper}
                onChange={(e) => onChange('require_upper', e.target.checked)}
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
  )
})
