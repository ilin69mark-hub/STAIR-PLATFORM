import React from 'react'
import type {
  CostReport,
  FunnelReport,
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

// ---- Воронка витрины --------------------------------------------------------
//
// Три вопроса, ради которых панель и делается: где идёт трафик (шаги),
// где спотыкаются (blockers) и где уходят (abandons). Числа подписаны словами
// по-человечески: «какое поле оставили пустым» читается сразу, а
// «blocker.field_invalid × 25» пришлось бы расшифровывать каждый раз.

const STEP_LABELS: Record<string, string> = {
  'session.start': 'Открыл сайт',
  'page.view': 'Смотрел страницы',
  'funnel.constructor_open': 'Дошёл до конструктора',
  'funnel.step_done': 'Получил расчёт',
  'cta.quote_clicked': 'Нажал «Рассчитать»',
  'cta.order_clicked': 'Отправил заявку',
}

const EVENT_LABELS: Record<string, string> = {
  'session.start': 'открытие сайта',
  'page.view': 'просмотр страницы',
  'funnel.constructor_open': 'открытие конструктора',
  'funnel.step_done': 'расчёт получен',
  'cta.quote_clicked': 'нажатие «Рассчитать»',
  'blocker.field_invalid': 'поле не прошло проверку',
  'blocker.api_error': 'отказ сервера',
  'cta.order_clicked': 'отправка заявки',
}

const REASON_LABELS: Record<string, string> = {
  widthMM: 'ширина марша',
  heightMM: 'высота',
  stepThicknessMM: 'толщина ступени',
  stringerThicknessMM: 'толщина косоура',
  approachSpaceMM: 'просвет',
  clearanceMM: 'просвет',
  roomWidthMM: 'ширина помещения',
  roomLengthMM: 'длина помещения',
  railingHeightMM: 'высота перил',
  landingWidthMM: 'ширина площадки',
  landingDepthMM: 'глубина площадки',
  winderCountMM: 'поворотные ступени',
  network: 'сеть недоступна',
}

function stepLabel(name: string): string {
  return STEP_LABELS[name] ?? name
}

function eventLabel(name: string): string {
  return EVENT_LABELS[name] ?? name
}

function reasonLabel(reason: string): string {
  if (reason === '') return 'без кода'
  if (REASON_LABELS[reason]) return REASON_LABELS[reason]
  // Идентификаторы полей приходят из формы верблюжьим регистром; серверные
  // коды — заглавными буквами. Разворачиваем оба случая в подпись.
  return reason.replace(/MM$/, '').replace(/([a-z])([A-Z])/g, '$1 $2').toLowerCase()
}

function pct(v: number): string {
  return `${(v * 100).toFixed(1)}%`
}

function humanSeconds(v: number): string {
  if (!Number.isFinite(v) || v <= 0) return '—'
  if (v < 90) return `${Math.round(v)} с`
  return `${Math.round(v / 60)} мин`
}

interface FunnelProps {
  funnel: FunnelReport | null
  loading: boolean
}

export const FunnelAnalyticsPanel = React.memo(function FunnelAnalyticsPanel({
  funnel,
  loading,
}: FunnelProps) {
  if (loading) {
    return (
      <section className="panel">
        <h2 className="panel__title">Воронка витрины</h2>
        <p className="muted">Загрузка…</p>
      </section>
    )
  }
  if (!funnel) {
    return (
      <section className="panel">
        <h2 className="panel__title">Воронка витрины</h2>
        <p className="muted">Нет данных. События появляются после того, как посетитель согласится на сбор статистики.</p>
      </section>
    )
  }

  const hasData = funnel.sessions > 0
  const people = funnel.visitors > 0 ? funnel.visitors : funnel.sessions
  // Люди — это разные числа: визитов может быть в разы больше, чем
  // посетителей. Показываем оба, иначе «100 визитов» читается как
  // «100 человек».
  const peopleWord = funnel.visitors > 0 ? 'Уникальных посетителей' : 'Визитов (без идентификации)'

  return (
    <section className="panel">
      <h2 className="panel__title">Воронка витрины</h2>
      <p className="muted">
        Поведение посетителей сайта: {funnel.from} — {funnel.to}. Данные собираются
        только у тех, кто дал согласие, поэтому счётчики меньше реального трафика.
      </p>

      {!hasData ? (
        <p className="muted">За выбранное окно согласий не было — собрать нечего.</p>
      ) : (
        <>
          {!funnel.visitor_identity_enabled && (
            <p className="muted">
              Уникальные посетители не считаются: на сервере не задана соль{' '}
              <code>STAIR_ANALYTICS_SALT</code>. Пока её нет, визиты и посетители
              совпадают. Это осознанный режим приватности: без соли пересечь
              визиты невозможно.
            </p>
          )}
          <dl className="kv">
            <div>
              <dt>{peopleWord}</dt>
              <dd>{people}</dd>
            </div>
            <div>
              <dt>Визитов</dt>
              <dd>{funnel.sessions}</dd>
            </div>
            <div>
              <dt>Событий</dt>
              <dd>{funnel.events}</dd>
            </div>
            <div>
              <dt>Средняя длительность визита</dt>
              <dd>{humanSeconds(funnel.avg_seconds)}</dd>
            </div>
          </dl>

          <p className="panel__sub">Где идёт трафик</p>
          <table className="table">
            <thead>
              <tr>
                <th>Шаг</th>
                <th className="num">Людей</th>
                <th className="num">Визитов</th>
                <th className="num">Доля</th>
                <th className="num">От предыдущего</th>
              </tr>
            </thead>
            <tbody>
              {funnel.steps.map((s) => (
                <tr key={s.name}>
                  <td>{stepLabel(s.name)}</td>
                  <td className="num">{s.visitors > 0 ? s.visitors : s.sessions}</td>
                  <td className="num">{s.sessions}</td>
                  <td className="num">{pct(s.share)}</td>
                  <td className="num row--total">
                    {s.step_share > 0 ? pct(s.step_share) : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <p className="panel__sub">Где спотыкаются</p>
          {funnel.blockers.length === 0 ? (
            <p className="muted">Затыков не зафиксировано.</p>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Причина</th>
                  <th>Что не так</th>
                  <th className="num">Раз</th>
                </tr>
              </thead>
              <tbody>
                {funnel.blockers.map((b) => (
                  <tr key={`${b.event}:${b.reason}`}>
                    <td>{eventLabel(b.event)}</td>
                    <td>{reasonLabel(b.reason)}</td>
                    <td className="num">{b.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          <p className="panel__sub">Где бросают</p>
          <p className="muted">
            Последнее действие перед уходом со страницы. Это и есть ответ на
            вопрос, что чинить в первую очередь. Доли считаются от всех
            уходов, поэтому в столбце сумма всегда 100%.
          </p>
          {funnel.abandons.length === 0 ? (
            <p className="muted">Данных об уходах нет (нужно время на визит).</p>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>На чём бросили</th>
                  <th className="num">Визитов</th>
                  <th className="num">Доля от уходов</th>
                  <th className="num">Сколько пробыли</th>
                </tr>
              </thead>
              <tbody>
                {funnel.abandons.map((a) => (
                  <tr key={a.last_event}>
                    <td>{eventLabel(a.last_event)}</td>
                    <td className="num">{a.sessions}</td>
                    <td className="num">{pct(a.share)}</td>
                    <td className="num">{humanSeconds(a.avg_seconds)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </>
      )}
    </section>
  )
})
