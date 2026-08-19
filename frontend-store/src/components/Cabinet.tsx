import { useCallback, useEffect, useState } from 'react'
import { ordersApi } from '../api/store'
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

// Личный кабинет: мои заказы и их статусы.
export function Cabinet() {
  const { user } = useAuth()
  const [orders, setOrders] = useState<OrderDTO[]>([])
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
                <td>{new Date(o.created_at).toLocaleString('ru-RU')}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}

function configDims(o: OrderDTO): string {
  const c = o.config as { width_mm?: number; height_mm?: number } | null | undefined
  if (c && typeof c.width_mm === 'number' && typeof c.height_mm === 'number') {
    return `${c.width_mm} × ${c.height_mm} мм`
  }
  return '—'
}