import { useEffect, useState } from 'react'
import { SECTIONS, SECTIONS_META, type SectionId } from './sections'
import { MaterialsSection, OrdersSection, OverviewSection, ServicesSection } from './sections/Overview'
import { PaymentsSection } from './sections/Payments'
import { PricesSection, SettingsSection } from './sections/PricesSettings'
import { LoginForm } from './LoginForm'
import { loginErrorMessage } from './api/loginError'
import { useSession } from './api/session'

export function App() {
  const { user, loading, login, logout, refresh } = useSession()
  const [error, setError] = useState('')

  useEffect(() => {
    void refresh()
  }, [refresh])

  if (loading) return <p className="hint">Проверяем сессию…</p>
  if (!user) {
    return (
      <LoginForm
        error={error}
        onLogin={async (email, password) => {
          setError('')
          try {
            await login(email, password)
          } catch (e) {
            setError(loginErrorMessage(e))
            throw e
          }
        }}
      />
    )
  }

  return <StoreAdmin user={user} onLogout={logout} />
}

function StoreAdmin({
  user,
  onLogout,
}: {
  user: { email: string; name: string; role: string }
  onLogout: () => void
}) {
  const [active, setActive] = useState<SectionId>('overview')

  return (
    <div className="layout">
      <nav aria-label="Разделы магазина">
        <h1>Магазин</h1>
        <ul>
          {SECTIONS.map((s) => (
            <li key={s.id}>
              <button
                type="button"
                className={s.id === active ? 'nav active' : 'nav'}
                aria-current={s.id === active ? 'page' : undefined}
                onClick={() => setActive(s.id)}
              >
                {s.title}
              </button>
            </li>
          ))}
        </ul>
        <div className="account">
          <span className="hint">{user.name || user.email}</span>
          <button type="button" className="secondary" onClick={onLogout}>
            Выйти
          </button>
        </div>
      </nav>
      <main>
        {active === 'overview' && <OverviewSection />}
        {active === 'prices' && <PricesSection />}
        {active === 'settings' && <SettingsSection />}
        {active === 'materials' && <MaterialsSection />}
        {active === 'services' && <ServicesSection />}
        {active === 'orders' && <OrdersSection />}
        {active === 'payments' && <PaymentsSection />}
        {!SECTIONS_META[active].ready && <Placeholder id={active} />}
      </main>
    </div>
  )
}

function Placeholder({ id }: { id: SectionId }) {
  const meta = SECTIONS_META[id]
  return (
    <section>
      <h2>{SECTIONS.find((s) => s.id === id)?.title}</h2>
      <p className="hint">{meta.note ?? 'Раздел в разработке.'}</p>
    </section>
  )
}
