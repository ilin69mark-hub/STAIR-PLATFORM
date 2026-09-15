import { useEffect, useState } from 'react'
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

function routeFromHash(hash: string): Pick<AppState, 'route' | 'legal'> {
  const h = hash.replace(/^#/, '')
  switch (h) {
    case 'constructor':
      return { route: 'constructor', legal: null }
    case 'cabinet':
      return { route: 'cabinet', legal: null }
    case 'offer':
    case 'privacy':
    case 'cookies':
      return { route: 'landing', legal: h }
    default:
      return { route: 'landing', legal: null }
  }
}

interface AppState {
  route: Route
  legal: LegalRoute | null
}

function App() {
  const { user } = useAuth()
  const [{ route, legal }, setState] = useState<AppState>(() => routeFromHash(window.location.hash))

  useEffect(() => {
    const sync = () => setState(routeFromHash(window.location.hash))
    window.addEventListener('hashchange', sync)
    return () => window.removeEventListener('hashchange', sync)
  }, [])

  const syncHash = (r: Route, l: LegalRoute | null) => {
    const target = l ? `#${l}` : r === 'landing' ? window.location.pathname : `#${r}`
    history.replaceState(null, '', target)
  }

  const open = (r: Route) => {
    setState({ route: r, legal: null })
    syncHash(r, null)
  }
  const openLegal = (k: InfoPageKind) => {
    setState((s) => {
      syncHash(s.route, k)
      return { route: s.route, legal: k }
    })
  }
  const closeLegal = () => {
    setState((s) => {
      syncHash(s.route, null)
      return { route: s.route, legal: null }
    })
  }

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
          <InfoPage kind={legal} onBack={closeLegal} />
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