import { useCallback, useEffect, useState } from 'react'
import { ordersApi, paymentsApi, type MyPayment } from '../api/store'
import { useAuth } from '../auth/context'
import { apiErrorMessage } from '../auth/errors'
import type { OrderDTO, OrderStatus } from '@shared/types'
import { fmt } from '@shared/format'
import { AuthForm } from './AuthForm'

const statusLabels: Record<OrderStatus, string> = {
  new: 'Новый',
  priced: 'Оценён',
  confirmed: 'Подтверждён',
  in_progress: 'В работе',
  completed: 'Выполнен',
  cancelled: 'Отменён',
}

function priceOf(o: OrderDTO): number | undefined {
  const p = o.price as { final_price_rub?: number } | null | undefined
  if (p && typeof p.final_price_rub === 'number') return p.final_price_rub
  return undefined
}

function formatCreatedAt(raw: unknown): string {
  if (typeof raw !== 'string') return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ru-RU')
}

// Личный кабинет: мои заказы и их статусы.
export function Cabinet() {
  const { user } = useAuth()
  const [orders, setOrders] = useState<OrderDTO[]>([])
  // Покупки услуг (этап 4): оплаченные и ожидающие оплаты замеры/проекты.
  const [payments, setPayments] = useState<MyPayment[]>([])
  const [paymentsError, setPaymentsError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setBusy(true)
    setError(null)
    try {
      setOrders(await ordersApi.listMine())
    } catch (e) {
      setError(apiErrorMessage(e, 'Не удалось загрузить заказы'))
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => {
    if (user) void load()
  }, [user, load])

  useEffect(() => {
    if (!user) return
    paymentsApi
      .mine()
      .then(setPayments)
      .catch((e) => {
        setPayments([])
        setPaymentsError(apiErrorMessage(e, 'Не удалось загрузить покупки'))
      })
  }, [user])

  if (!user) {
    return (
      <div>
        <section className="panel">
          <h2>Личный кабинет</h2>
          <p className="sub">Войдите, чтобы видеть свои заказы и статусы их обработки.</p>
        </section>
        <AuthForm submitLabel="Войти в кабинет" />
      </div>
    )
  }

  return (
    <>
      <section className="panel">
        <h2>Мои покупки</h2>
        <p className="sub">
          {payments.length === 0
            ? 'Оплаченные услуги и счета по заказам появятся здесь.'
            : 'Услуги и их статус оплаты.'}
        </p>
        {paymentsError && <p className="error">{paymentsError}</p>}
        {payments.length > 0 && (
          <ul className="list">
            {payments.map((p) => (
              <li key={p.id} data-payment={p.tier_id ?? 'project'}>
                <div>
                  <strong>{p.title}</strong>
                  <div className="sub">
                    {formatRub(p.amount_minor)} · {PAYMENT_STATUS[p.status] ?? p.status} ·{' '}
                    {new Date(p.created_at).toLocaleDateString('ru-RU')}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
      <section className="panel">
      <h2>Мои заказы</h2>
      <p className="sub">
        {user.name} ({user.email})
      </p>
      {busy ? (
        <p className="spinner">Загрузка…</p>
      ) : error ? (
        <div className="alert alert--error" role="alert">
          {error}
        </div>
      ) : orders.length === 0 ? (
        <p className="muted">Заказов пока нет. Рассчитайте лестницу в конструкторе.</p>
      ) : (
        <table className="orders-table">
          <thead>
            <tr>
              <th>№ заказа</th>
              <th>Статус</th>
              <th>Ширина × высота</th>
              <th>Цена</th>
              <th>Создан</th>
            </tr>
          </thead>
          <tbody>
            {orders.map((o) => (
              <tr key={o.id}>
                <td>{o.id.slice(0, 8)}</td>
                <td>
                  <span className="badge">{statusLabels[o.status]}</span>
                </td>
                <td>
                  {configDims(o)}
                </td>
                <td>{fmt.rubMajor(priceOf(o))}</td>
                <td>{formatCreatedAt(o.created_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      </section>
    </>
  )
}

const PAYMENT_STATUS: Record<string, string> = {
  pending: 'ожидает оплаты',
  paid: 'оплачено',
  failed: 'ошибка оплаты',
  refunded: 'возврат',
}

// FE-18a (forensic 2026-09-27): суммы платежей показывались с
// maximumFractionDigits: 0, то есть копейки отбрасывались — клиент, оплативший
// 1 234,56 ₽, видел «1 235 ₽». Это список его собственных покупок и, рядом,
// основание для обращения в поддержку («списали больше, чем показано»).
//
// Рубля — валюта с двумя знаками после запятой (domain/pricing:
// CurrencyRUB{Decimals: 2}), поэтому копейки здесь не опция оформления, а
// часть суммы. Раньше разные экраны использовали разную точность: rubMajor в
// shared/format даёт 2 знака, а эта локальная копия резала до целых.
function formatRub(minor: number): string {
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(minor / 100)
}

function configDims(o: OrderDTO): string {
  const c = o.config as { width_mm?: number; height_mm?: number } | null | undefined
  if (c && typeof c.width_mm === 'number' && typeof c.height_mm === 'number') {
    return `${c.width_mm} × ${c.height_mm} мм`
  }
  return '—'
}