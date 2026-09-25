import { useCallback, useEffect, useState } from 'react'
import {
  ORDER_STATUSES,
  storeApi,
  type AdminOrder,
  type MaterialCatalogItem,
  type OrderStatus,
  type PaymentTier,
} from '../api/store'
import { ErrorBox, Loading } from './PricesSettings'

function useLoad<T>(load: () => Promise<T>, deps: unknown[]): {
  data: T | null
  error: string
} {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')

  const reload = useCallback(() => {
    setError('')
    load()
      .then(setData)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)

  useEffect(reload, [reload])
  return { data, error }
}

// OverviewSection — сводка магазина: заказы, каталог материалов, услуги.
export function OverviewSection() {
  const orders = useLoad<AdminOrder[]>(() => storeApi.orders(), [])
  const materials = useLoad<MaterialCatalogItem[]>(() => storeApi.materials(), [])
  const tiers = useLoad<PaymentTier[]>(() => storeApi.tiers(), [])

  const error = orders.error || materials.error || tiers.error
  if (error) return <ErrorBox text={error} />

  const newOrders = (orders.data ?? []).filter((o) => o.status === 'new').length
  return (
    <section>
      <h2>Обзор</h2>
      <div className="cards">
        <div className="card">
          <span className="card-value">{orders.data ? orders.data.length : '—'}</span>
          <span className="card-label">заказов всего</span>
        </div>
        <div className="card">
          <span className="card-value">{orders.data ? newOrders : '—'}</span>
          <span className="card-label">новых заказов</span>
        </div>
        <div className="card">
          <span className="card-value">{materials.data ? materials.data.length : '—'}</span>
          <span className="card-label">материалов в каталоге</span>
        </div>
        <div className="card">
          <span className="card-value">{tiers.data ? tiers.data.length : '—'}</span>
          <span className="card-label">платных услуг</span>
        </div>
      </div>
      {orders.data && orders.data.length === 0 && (
        <p className="hint">Заказов пока нет — они появятся здесь после заявки с витрины.</p>
      )}
    </section>
  )
}

// MaterialsSection — каталог материалов витрины (только чтение: цена меняется
// в разделе «Цены», параметры материала — в движке MFG-0005).
export function MaterialsSection() {
  const { data, error } = useLoad<MaterialCatalogItem[]>(() => storeApi.materials(), [])
  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />
  return (
    <section>
      <h2>Материалы</h2>
      <table>
        <thead>
          <tr>
            <th>Код</th>
            <th>Название</th>
            <th>Категория</th>
            <th>Плотность, кг/м³</th>
            <th>Толщина, мм</th>
            <th>Цена, ₽/кг</th>
          </tr>
        </thead>
        <tbody>
          {data.map((m) => (
            <tr key={m.code}>
              <td>
                <code>{m.code}</code>
              </td>
              <td>{m.name_ru || m.name}</td>
              <td>{m.category}</td>
              <td>{m.density_kg_m3}</td>
              <td>
                {m.min_thickness_mm}–{m.max_thickness_mm}
              </td>
              <td>{m.price_per_kg_rub}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}

// ServicesSection — платные услуги витрины (каталог tiers, этап 4).
export function ServicesSection() {
  const { data, error } = useLoad<PaymentTier[]>(() => storeApi.tiers(), [])
  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />
  return (
    <section>
      <h2>Услуги</h2>
      <table>
        <thead>
          <tr>
            <th>Код</th>
            <th>Название</th>
            <th>Описание</th>
            <th>Цена</th>
          </tr>
        </thead>
        <tbody>
          {data.map((t) => (
            <tr key={t.id}>
              <td>
                <code>{t.id}</code>
              </td>
              <td>{t.title}</td>
              <td>{t.description ?? '—'}</td>
              <td>
                {t.amount_rub} {t.currency}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}

const STATUS_LABELS: Record<OrderStatus, string> = {
  new: 'Новый',
  priced: 'Рассчитан',
  confirmed: 'Подтверждён',
  in_progress: 'В работе',
  completed: 'Выполнен',
  cancelled: 'Отменён',
}
const KIND_LABELS: Record<string, string> = {
  order: 'Заказ',
  consultation: 'Консультация',
}

// OrdersSection — заказы витрины: контакты, статус, смена статуса.
export function OrdersSection() {
  const { data, error, reload } = useLoadRethrow<AdminOrder[]>(() => storeApi.orders(), [])
  const [busy, setBusy] = useState('')
  const [message, setMessage] = useState('')

  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />

  const changeStatus = async (id: string, status: OrderStatus) => {
    setBusy(id)
    setMessage('')
    try {
      await storeApi.updateOrderStatus(id, status)
      setMessage('Статус заказа обновлён')
      reload()
    } catch (e) {
      setMessage(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy('')
    }
  }

  return (
    <section>
      <h2>Заказы</h2>
      {data.length === 0 && <p className="hint">Заказов пока нет.</p>}
      {data.map((o) => (
        <article key={o.id} className="card order">
          <div>
            <strong>{KIND_LABELS[o.kind] ?? o.kind}</strong> · {new Date(o.created_at).toLocaleString('ru-RU')}
          </div>
          <div className="hint">
            {o.contact.name} · {o.contact.email}
            {o.contact.phone ? ` · ${o.contact.phone}` : ''}
          </div>
          <label>
            Статус
            <select
              value={o.status}
              disabled={busy === o.id}
              onChange={(e) => void changeStatus(o.id, e.target.value as OrderStatus)}
            >
              {ORDER_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {STATUS_LABELS[s]}
                </option>
              ))}
            </select>
          </label>
        </article>
      ))}
      {message && <p className="ok">{message}</p>}
    </section>
  )
}

// useLoadRethrow — useLoad с ручным reload (нужен после мутаций).
function useLoadRethrow<T>(load: () => Promise<T>, deps: unknown[]) {
  const [nonce, setNonce] = useState(0)
  const reload = useCallback(() => setNonce((n) => n + 1), [])
  const state = useLoad<T>(load, [...deps, nonce])
  return { ...state, reload }
}
