import React from 'react'
import type {
  CostReport,
  ManufacturingReport,
  ProjectReport,
  UsageGranularity,
  UsageReport,
} from '@shared/types'

interface UsageProps {
  usage: UsageReport | null
  loading: boolean
  granularity: UsageGranularity
  onChangeGranularity: (g: UsageGranularity) => void
}

export const UsageAnalyticsPanel = React.memo(function UsageAnalyticsPanel({
  usage,
  loading,
  granularity,
  onChangeGranularity,
}: UsageProps) {
  return (
    <section className="panel">
      <h2 className="panel__title">Аналитика использования</h2>
      <p className="muted">Активность tenant за выбранное окно (право analytics.read, EDR-0028).</p>
      <div className="row--actions">
        {(['day', 'week', 'month'] as UsageGranularity[]).map((g) => (
          <button
            key={g}
            className={g === granularity ? 'btn btn--primary' : 'btn'}
            onClick={() => onChangeGranularity(g)}
          >
            {g === 'day' ? 'День' : g === 'week' ? 'Неделя' : 'Месяц'}
          </button>
        ))}
      </div>
      {loading ? (
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
  )
})

interface ProjectsProps {
  projects: ProjectReport | null
  loading: boolean
}

export const ProjectsAnalyticsPanel = React.memo(function ProjectsAnalyticsPanel({
  projects,
  loading,
}: ProjectsProps) {
  return (
    <section className="panel">
      <h2 className="panel__title">Проекты</h2>
      <p className="muted">Сводка по проектам tenant (EDR-0029).</p>
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : projects ? (
        <>
          <dl className="kv">
            <div>
              <dt>Проекты (в окне)</dt>
              <dd>
                {projects.totals.projects} ({projects.totals.projects_created})
              </dd>
            </div>
            <div>
              <dt>Статусы</dt>
              <dd>
                {projects.totals.by_status.draft ?? 0} черновиков ·{' '}
                {projects.totals.by_status.in_review ?? 0} на ревью ·{' '}
                {projects.totals.by_status.approved ?? 0} утверждено
              </dd>
            </div>
            <div>
              <dt>С расчётом / валидных</dt>
              <dd>
                {projects.totals.projects_with_calculation} / {projects.totals.valid_projects}
              </dd>
            </div>
            <div>
              <dt>Конфигурации / расчёты / комментарии</dt>
              <dd>
                {projects.totals.configurations} / {projects.totals.calculations} /{' '}
                {projects.totals.comments}
              </dd>
            </div>
          </dl>
          <table className="table">
            <thead>
              <tr>
                <th>Проект</th>
                <th>Статус</th>
                <th>Конфигурации</th>
                <th>Расчёты</th>
                <th>Последний расчёт</th>
                <th>Комментарии</th>
                <th>Участники</th>
              </tr>
            </thead>
            <tbody>
              {projects.projects.map((p) => (
                <tr key={p.id}>
                  <td>
                    <strong>{p.name}</strong>
                    <span className="muted"> · {p.status}</span>
                  </td>
                  <td>{p.status}</td>
                  <td>{p.configurations}</td>
                  <td>{p.calculations}</td>
                  <td>
                    {p.latest_calculation_valid === null
                      ? '—'
                      : p.latest_calculation_valid
                        ? 'валиден'
                        : 'ошибки'}
                  </td>
                  <td>{p.comments}</td>
                  <td>{p.members}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      ) : null}
    </section>
  )
})

interface MfgProps {
  mfg: ManufacturingReport | null
  loading: boolean
  granularity: UsageGranularity
  onChangeGranularity: (g: UsageGranularity) => void
}

export const ManufacturingPanel = React.memo(function ManufacturingPanel({
  mfg,
  loading,
  granularity,
  onChangeGranularity,
}: MfgProps) {
  return (
    <section className="panel">
      <h2 className="panel__title">Производство</h2>
      <p className="muted">Агрегация производственных данных из снапшотов расчётов (EDR-0030).</p>
      <div className="row--actions">
        {(['day', 'week', 'month'] as UsageGranularity[]).map((g) => (
          <button
            key={g}
            className={g === granularity ? 'btn btn--primary' : 'btn'}
            onClick={() => onChangeGranularity(g)}
          >
            {g === 'day' ? 'День' : g === 'week' ? 'Неделя' : 'Месяц'}
          </button>
        ))}
      </div>
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : mfg ? (
        <>
          <dl className="kv">
            <div>
              <dt>Расчёты / детали</dt>
              <dd>
                {mfg.totals.calculations} / {mfg.totals.parts}
              </dd>
            </div>
            <div>
              <dt>BOM / карта раскроя / листы</dt>
              <dd>
                {mfg.totals.bom_lines} / {mfg.totals.cut_items} / {mfg.totals.sheets}
              </dd>
            </div>
            <div>
              <dt>Утилизация</dt>
              <dd>{(mfg.totals.utilization * 100).toFixed(1)}%</dd>
            </div>
            <div>
              <dt>Площади деталей / листов / отходы (м²)</dt>
              <dd>
                {(mfg.totals.part_area / 1e6).toFixed(2)} /{' '}
                {(mfg.totals.sheet_area / 1e6).toFixed(2)} /{' '}
                {(mfg.totals.waste_area / 1e6).toFixed(2)}
              </dd>
            </div>
            {Object.keys(mfg.totals.materials).length > 0 && (
              <div>
                <dt>Материалы</dt>
                <dd>
                  {Object.entries(mfg.totals.materials)
                    .map(([m, n]) => `${m}: ${n}`)
                    .join(', ')}
                </dd>
              </div>
            )}
          </dl>
          <table className="table">
            <thead>
              <tr>
                <th>Дата</th>
                <th>Расчёты</th>
                <th>Детали</th>
                <th>Листы</th>
                <th>Утилизация</th>
              </tr>
            </thead>
            <tbody>
              {mfg.series.map((p) => (
                <tr key={p.bucket}>
                  <td>{p.bucket}</td>
                  <td>{p.calculations}</td>
                  <td>{p.parts}</td>
                  <td>{p.sheets}</td>
                  <td>{(p.utilization * 100).toFixed(1)}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      ) : null}
    </section>
  )
})

interface CostProps {
  cost: CostReport | null
  loading: boolean
  granularity: UsageGranularity
  onChangeGranularity: (g: UsageGranularity) => void
}

export const CostAnalyticsPanel = React.memo(function CostAnalyticsPanel({
  cost,
  loading,
  granularity,
  onChangeGranularity,
}: CostProps) {
  return (
    <section className="panel">
      <h2 className="panel__title">Стоимость</h2>
      <p className="muted">Финансовые метрики из ценовых брейкдаунов расчётов (EDR-0031).</p>
      <div className="row--actions">
        {(['day', 'week', 'month'] as UsageGranularity[]).map((g) => (
          <button
            key={g}
            className={g === granularity ? 'btn btn--primary' : 'btn'}
            onClick={() => onChangeGranularity(g)}
          >
            {g === 'day' ? 'День' : g === 'week' ? 'Неделя' : 'Месяц'}
          </button>
        ))}
      </div>
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : cost ? (
        <>
          <dl className="kv">
            <div>
              <dt>Расчёты</dt>
              <dd>{cost.totals.calculations}</dd>
            </div>
            <div>
              <dt>Себестоимость (материал/машина/труд/накладные)</dt>
              <dd>
                {cost.totals.material} / {cost.totals.machine} / {cost.totals.labor} /{' '}
                {cost.totals.overhead}
              </dd>
            </div>
            <div>
              <dt>Итоговая цена / средняя</dt>
              <dd>
                {cost.totals.final_price} / {cost.totals.avg_final_price.toFixed(2)} {cost.totals.currency}
              </dd>
            </div>
            <div>
              <dt>Прибыль / налог</dt>
              <dd>
                {cost.totals.margin} / {cost.totals.tax}
              </dd>
            </div>
          </dl>
          <table className="table">
            <thead>
              <tr>
                <th>Дата</th>
                <th>Расчёты</th>
                <th>Итоговая цена</th>
                <th>Средняя</th>
              </tr>
            </thead>
            <tbody>
              {cost.series.map((p) => (
                <tr key={p.bucket}>
                  <td>{p.bucket}</td>
                  <td>{p.calculations}</td>
                  <td>{p.final_price}</td>
                  <td>{p.avg_final_price.toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      ) : null}
    </section>
  )
})
