import { useState } from 'react'
import './App.css'
import { useAuth } from './auth/context'
import { Landing } from './components/Landing'
import { Constructor } from './components/Constructor'
import { Cabinet } from './components/Cabinet'
import { CookieBanner } from './components/CookieBanner'
import { InfoPage, type InfoPageKind } from './components/InfoPage'
import { CONTACTS } from './config'

type Route = 'landing' | 'constructor' | 'cabinet'
type LegalRoute = 'offer' | 'privacy' | 'cookies'

function App() {
  const { user } = useAuth()
  const [route, setRoute] = useState<Route>('landing')
  const [legal, setLegal] = useState<LegalRoute | null>(null)

  const open = (r: Route) => {
    setLegal(null)
    setRoute(r)
  }
  const openLegal = (k: InfoPageKind) => setLegal(k)

  return (
    <div className="store">
      <header className="store-header">
        <div className="store-brand" onClick={() => open('landing')} role="button">
          STAIR PLATFORM <small>лестницы на заказ</small>
        </div>
        <div className="header-contacts">
          <a href={CONTACTS.phoneHref}>{CONTACTS.phone}</a>
          <span className="sep">·</span>
          <a href={CONTACTS.emailHref}>{CONTACTS.email}</a>
        </div>
        <nav className="store-nav">
          <a
            href="#constructor"
            onClick={(e) => {
              e.preventDefault()
              open('constructor')
            }}
          >
            Конструктор
          </a>
          <a
            href="#cabinet"
            onClick={(e) => {
              e.preventDefault()
              open('cabinet')
            }}
          >
            {user ? user.name.split(' ')[0] : 'Кабинет'}
          </a>
        </nav>
      </header>

      <main className="store-main">
        {legal !== null ? (
          <InfoPage kind={legal} onBack={() => setLegal(null)} />
        ) : (
          <>
            {route === 'landing' && <Landing onStart={() => open('constructor')} />}
            {route === 'constructor' && <Constructor />}
            {route === 'cabinet' && <Cabinet />}
          </>
        )}
      </main>

      <footer className="store-footer">
        <div className="footer-contacts">
          <a href={CONTACTS.phoneHref}>{CONTACTS.phone}</a>
          <span className="sep">·</span>
          <a href={CONTACTS.emailHref}>{CONTACTS.email}</a>
        </div>
        <div className="footer-links">
          <a href="#offer" onClick={(e) => { e.preventDefault(); openLegal('offer') }}>
            Публичная оферта
          </a>
          <a href="#privacy" onClick={(e) => { e.preventDefault(); openLegal('privacy') }}>
            Политика конфиденциальности
          </a>
          <a href="#cookies" onClick={(e) => { e.preventDefault(); openLegal('cookies') }}>
            Политика cookie
          </a>
        </div>
        <p className="footer-note">STAIR PLATFORM — инженерный расчёт и изготовление лестниц</p>
      </footer>

      <CookieBanner onOpenPolicy={() => openLegal('cookies')} />
    </div>
  )
}

export default App