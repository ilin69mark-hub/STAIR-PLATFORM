import React from 'react'
import type { AdminOverview } from '@shared/types'

interface Props {
  overview: AdminOverview | null
}

export const OverviewPanel = React.memo(function OverviewPanel({ overview }: Props) {
  if (!overview) return null
  return (
    <section className="panel">
      <h2 className="panel__title">Обзор</h2>
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
    </section>
  )
})
