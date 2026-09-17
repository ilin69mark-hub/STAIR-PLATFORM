import React from 'react'
import type { AdminOverview } from '@shared/types'

interface Props {
  overview: AdminOverview | null
}

export const OverviewPanel = React.memo(function OverviewPanel({ overview }: Props) {
  if (!overview) return null

  const stats: Array<{ label: string; value: number }> = [
    { label: 'Пользователи', value: overview.users },
    { label: 'Активных', value: overview.active_users },
    { label: 'Заблокированных', value: overview.disabled_users },
    { label: 'Администраторы', value: overview.admins },
    { label: 'Проекты', value: overview.projects },
    { label: 'Активных API-ключей', value: overview.active_api_keys },
  ]

  return (
    <section className="panel">
      <h2 className="panel__title">Обзор</h2>
      <div className="stat-grid">
        {stats.map((st) => (
          <div className="stat-card" key={st.label}>
            <div className="stat-card__label">{st.label}</div>
            <div className="stat-card__value">{st.value}</div>
          </div>
        ))}
      </div>
    </section>
  )
})
