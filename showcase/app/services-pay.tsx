'use client'

// Оплата услуги с витрины (этап 4). Клиент выбирает услугу из серверного
// каталога, при необходимости входит, затем мы уводим его на страницу оплаты
// PSP. Сумма приходит с сервера — подменить её в браузере нельзя (S-150).
import { useEffect, useState } from 'react'
import { paymentsApi, type PaymentTier } from '@shared/storefront/api/store'
import { AuthForm } from '@shared/storefront/components/AuthForm'
import { StorefrontProviders } from '@shared/storefront/StorefrontProviders'
import { useAuth } from '@shared/storefront/auth/context'

function formatRub(v: number): string {
  return new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 }).format(v)
}

function ServicesPayInner() {
  const { user, loading } = useAuth()
  const [tiers, setTiers] = useState<PaymentTier[] | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState<PaymentTier | null>(null)

  useEffect(() => {
    let alive = true
    paymentsApi
      .tiers()
      .then((list) => {
        if (alive) setTiers(list)
      })
      .catch(() => {
        if (alive) setTiers([])
      })
    return () => {
      alive = false
    }
  }, [])

  const pay = async (tier: PaymentTier) => {
    setBusyId(tier.id)
    setError(null)
    try {
      const checkout = await paymentsApi.checkout(tier.id)
      if (!checkout.checkout_url) {
        setError('Платёжная страница недоступна. Попробуйте позже.')
        return
      }
      window.location.assign(checkout.checkout_url)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Не удалось перейти к оплате')
      setBusyId(null)
    }
  }

  if (loading) return <div className="scene__loading">Проверяем вход…</div>


  return (
    <div data-testid="services-pay">
      {tiers === null && <div className="scene__loading">Загружаем услуги…</div>}
      {pending && !user && (
        <div className="card" style={{ marginTop: 16 }} data-testid="services-auth">
          <span className="card__title">Оплата услуги «{pending.title}»</span>
          <p className="card__meta">
            {formatRub(pending.amount_rub)}. Войдите — и мы сразу перейдём на страницу оплаты.
          </p>
          <AuthForm note="После входа вернёмся к оплате выбранной услуги." />
        </div>
      )}
      {tiers?.length === 0 && (
        <div className="notice">Каталог услуг временно недоступен — обновите страницу.</div>
      )}
      {error && <div className="alert alert--error">{error}</div>}
      <div className="card-grid">
        {tiers?.map((t) => (
          <div className="card" key={t.id} data-tier={t.id}>
            <span className="card__title">{t.title}</span>
            <span className="price">{formatRub(t.amount_rub)}</span>
            <p className="card__meta">{t.description}</p>
            <button
              type="button"
              className="btn"
              disabled={busyId === t.id}
              data-action={user ? 'pay' : 'login'}
              onClick={() => (user ? pay(t) : setPending(t))}
            >
              {busyId === t.id ? 'Переходим к оплате…' : user ? 'Оплатить' : 'Войти и оплатить'}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}

export function ServicesPay() {
  return (
    <StorefrontProviders>
      <ServicesPayInner />
    </StorefrontProviders>
  )
}
