import { useEffect, useRef, useState } from 'react'
import { storeApi, type AdminPayment, type PaymentStatus } from '../api/store'

type Feedback = {
  kind: 'success' | 'error'
  text: string
}

const PAYMENT_STATUS_LABELS: Record<PaymentStatus, string> = {
  pending: 'Ожидает оплаты',
  paid: 'Оплачен',
  failed: 'Ошибка оплаты',
  refunded: 'Возвращён',
}

export function PaymentsSection() {
  const [payments, setPayments] = useState<AdminPayment[] | null>(null)
  const [loadError, setLoadError] = useState('')
  const [refundingId, setRefundingId] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<Feedback | null>(null)
  const refundInFlight = useRef(false)

  useEffect(() => {
    let active = true
    storeApi.payments().then(
      (loaded) => {
        if (active) setPayments(loaded)
      },
      (error: unknown) => {
        if (active) {
          setLoadError(error instanceof Error ? error.message : 'Не удалось загрузить платежи')
        }
      },
    )
    return () => {
      active = false
    }
  }, [])

  const refund = async (id: string) => {
    if (refundInFlight.current) return

    refundInFlight.current = true
    setRefundingId(id)
    setFeedback(null)
    try {
      const updated = await storeApi.refundPayment(id)
      setPayments((current) => current?.map((payment) => (payment.id === id ? updated : payment)) ?? current)
      setFeedback({ kind: 'success', text: `Возврат платежа ${id} оформлен` })
    } catch (error: unknown) {
      setFeedback({
        kind: 'error',
        text: error instanceof Error ? error.message : 'Не удалось оформить возврат',
      })
    } finally {
      refundInFlight.current = false
      setRefundingId(null)
    }
  }

  return (
    <section>
      <h2>Платежи</h2>
      {loadError ? (
        <p className="error" role="alert">
          {loadError}
        </p>
      ) : payments === null ? (
        <p className="hint" role="status">
          Загрузка платежей…
        </p>
      ) : payments.length === 0 ? (
        <p className="hint">Платежей пока нет.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Сумма</th>
              <th>Статус</th>
              <th>Провайдер</th>
              <th>Дата</th>
              <th>Услуга</th>
              <th>Действия</th>
            </tr>
          </thead>
          <tbody>
            {payments.map((payment) => (
              <tr key={payment.id}>
                <td>
                  <code>{payment.id}</code>
                </td>
                <td>{formatAmount(payment)}</td>
                <td>{PAYMENT_STATUS_LABELS[payment.status] ?? payment.status}</td>
                <td>{payment.provider}</td>
                <td>
                  <time dateTime={payment.created_at}>
                    {new Date(payment.created_at).toLocaleString('ru-RU')}
                  </time>
                </td>
                <td>{serviceLabel(payment)}</td>
                <td>
                  {payment.status === 'paid' && (
                    <button
                      type="button"
                      className="secondary"
                      disabled={refundingId !== null}
                      onClick={() => void refund(payment.id)}
                    >
                      {refundingId === payment.id ? 'Возврат…' : 'Оформировать возврат'}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {feedback && (
        <p className={feedback.kind === 'error' ? 'error' : 'ok'} role={feedback.kind === 'error' ? 'alert' : 'status'}>
          {feedback.text}
        </p>
      )}
    </section>
  )
}

function formatAmount(payment: AdminPayment): string {
  const formatter = new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency: payment.currency,
  })
  const fractionDigits = formatter.resolvedOptions().maximumFractionDigits ?? 2
  return formatter.format(payment.amount_minor / 10 ** fractionDigits)
}

function serviceLabel(payment: AdminPayment): string {
  if (payment.tier_id) return payment.tier_id
  if (payment.project_id) return `Проект ${payment.project_id}`
  return '—'
}
