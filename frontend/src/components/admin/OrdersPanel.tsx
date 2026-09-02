import React from 'react'
import type { OrderDTO } from '@shared/types'
import { fmt } from '@shared/format'

const orderStatusOptions = [
  { value: 'new', label: 'Новый' },
  { value: 'priced', label: 'Оценён' },
  { value: 'confirmed', label: 'Подтверждён' },
  { value: 'in_progress', label: 'В работе' },
  { value: 'completed', label: 'Выполнен' },
  { value: 'cancelled', label: 'Отменён' },
] as const

function configSize(o: OrderDTO): string {
  const c = o.config as { width_mm?: number; height_mm?: number } | null | undefined
  if (c && typeof c.width_mm === 'number' && typeof c.height_mm === 'number') {
    return `${c.width_mm} × ${c.height_mm} мм`
  }
  return '—'
}

function priceOf(o: OrderDTO): string {
  const p = o.price as { final_price_rub?: number } | null | undefined
  if (p && typeof p.final_price_rub === 'number') return fmt.rubMajor(p.final_price_rub)
  return '—'
}

function orderDetails(o: OrderDTO): string {
  if (o.kind === 'consultation') {
    const c = o.config as { question?: string } | null | undefined
    return c && typeof c.question === 'string' ? c.question : '—'
  }
  const size = configSize(o)
  const price = priceOf(o)
  if (size === '—' && price === '—') return '—'
  if (size === '—') return price
  if (price === '—') return size
  return `${size} · ${price}`
}

interface Props {
  orders: OrderDTO[]
  onStatusChange: (id: string, status: string) => void
}

export const OrdersPanel = React.memo(function OrdersPanel({ orders, onStatusChange }: Props) {
  return (
    <section className="panel">
      <h2 className="panel__title">Заказы (store)</h2>
      <p className="muted">Лиды клиентского сайта: заказы из конструктора и консультации.</p>
      {orders.length === 0 ? (
        <p className="muted">Заказов пока нет.</p>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>№</th>
              <th>Тип</th>
              <th>Клиент</th>
              <th>Email / телефон</th>
              <th>Детали</th>
              <th>Статус</th>
              <th>Создан</th>
            </tr>
          </thead>
          <tbody>
            {orders.map((o) => (
              <tr key={o.id}>
                <td>{o.id.slice(0, 8)}</td>
                <td>
                  <span className={`badge ${o.kind === 'consultation' ? 'badge--warning' : ''}`}>
                    {o.kind === 'consultation' ? 'Консультация' : 'Заказ'}
                  </span>
                </td>
                <td>{o.contact.name}</td>
                <td>
                  {o.contact.email}
                  {o.contact.phone ? <span className="muted"> · {o.contact.phone}</span> : null}
                </td>
                <td>{orderDetails(o)}</td>
                <td>
                  <select
                    className="member__role"
                    aria-label={`Статус заказа ${o.id}`}
                    value={o.status}
                    onChange={(e) => onStatusChange(o.id, e.target.value)}
                  >
                    {orderStatusOptions.map((s) => (
                      <option key={s.value} value={s.value}>
                        {s.label}
                      </option>
                    ))}
                  </select>
                </td>
                <td>{new Date(o.created_at).toLocaleString('ru-RU')}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
})
