import { useState, type FormEvent } from 'react'
import { ordersApi } from '../api/store'
import { useAuth } from '../auth/context'
import { apiErrorMessage } from '../auth/errors'
import type { OrderDTO, QuoteResult } from '@shared/types'
import { AuthForm } from './AuthForm'

interface Props {
  quote: QuoteResult
  config: Record<string, unknown>
  onCreated?: (order: OrderDTO) => void
}

// Форма оформления заказа: контакты (Имя/Email/Телефон) отправляются вместе
// со снимками конфигурации и цены. Требуется авторизация + CSRF.
export function OrderForm({ quote, config, onCreated }: Props) {
  const { user } = useAuth()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  if (!user) {
    return (
      <AuthForm note="Для отправки заказа нужна учётная запись. Заказ будет доступен в личном кабинете." />
    )
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !email.trim()) {
      setError('Укажите имя и email')
      return
    }
    setBusy(true)
    setError(null)
    try {
      const created = await ordersApi.create({
        contact: { name: name.trim(), email: email.trim(), phone: phone.trim() },
        config,
        price: quote.pricing ?? null,
      })
      onCreated?.(created)
    } catch (err) {
      setError(apiErrorMessage(err, 'Не удалось отправить заказ'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="panel">
      <h2>Оформление заказа</h2>
      <p className="sub">
        Менеджер свяжется с вами для согласования проекта. Также можно удалить расчёт и
        вернуться к ним в кабинете.
      </p>
      <form className="auth-form" onSubmit={handleSubmit}>
        <div className="field">
          <label htmlFor="order-name">Имя</label>
          <input
            id="order-name"
            type="text"
            autoComplete="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor="order-email">Email</label>
          <input
            id="order-email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor="order-phone">Телефон</label>
          <input
            id="order-phone"
            type="tel"
            autoComplete="tel"
            placeholder="+7 (___) ___-__-__"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
        </div>
        {error && <div className="alert alert--error">{error}</div>}
        <div className="actions">
          <button className="sp-btn sp-btn--primary" type="submit" disabled={busy}>
            {busy ? 'Отправка…' : 'Отправить заказ'}
          </button>
        </div>
      </form>
    </div>
  )
}