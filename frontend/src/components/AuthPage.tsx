import { useEffect, useState, type FormEvent } from 'react'
import { useAuth } from '../auth/context'
import { authApi, type SsoConfig } from '../api/auth'
import { apiErrorMessage } from '../auth/errors'

type Mode = 'login' | 'register'

export function AuthPage() {
  const { login, register } = useAuth()
  const [mode, setMode] = useState<Mode>('login')
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [sso, setSso] = useState<SsoConfig | null>(null)

  // SSO (EDR-0017): показываем кнопку только когда провайдер включён.
  useEffect(() => {
    let cancelled = false
    authApi
      .ssoConfig()
      .then((c) => {
        if (!cancelled) setSso(c)
      })
      .catch(() => {
        if (!cancelled) setSso(null)
      })
    // Обработка отказа IdP (редирект колбэка с ?error=...).
    const params = new URLSearchParams(window.location.search)
    if (params.get('error') === 'sso_error') {
      const msg = params.get('message') ?? 'Не удалось войти через SSO'
      setError(msg)
      const url = new URL(window.location.href)
      url.search = ''
      window.history.replaceState({}, '', url.toString())
    }
    return () => {
      cancelled = true
    }
  }, [])

  const switchMode = (m: Mode) => {
    setMode(m)
    setError(null)
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!email || !password || (mode === 'register' && !name)) {
      setError('Заполните все поля')
      return
    }
    setBusy(true)
    setError(null)
    try {
      if (mode === 'register') {
        await register({ email, name, password })
      } else {
        await login({ email, password })
      }
    } catch (err) {
      setError(apiErrorMessage(err, 'Не удалось выполнить запрос'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page auth--page">
      <header className="page__header page__header--stacked">
        <div className="page__header-title">
          <h1 className="page__title">STAIR PLATFORM</h1>
          <p className="page__subtitle">Вход в систему</p>
        </div>
      </header>

      <div className="auth">
        <section className="panel">
          <div className="auth__tabs">
            <button
              className={`auth__tab${mode === 'login' ? ' auth__tab--active' : ''}`}
              onClick={() => switchMode('login')}
            >
              Вход
            </button>
            <button
              className={`auth__tab${mode === 'register' ? ' auth__tab--active' : ''}`}
              onClick={() => switchMode('register')}
            >
              Регистрация
            </button>
          </div>

          <form className="auth__form" onSubmit={handleSubmit}>
            {mode === 'register' && (
              <div className="field">
                <label className="field__label" htmlFor="auth-name">
                  Имя
                </label>
                <input
                  id="auth-name"
                  className="field__input"
                  type="text"
                  autoComplete="name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>
            )}
            <div className="field">
              <label className="field__label" htmlFor="auth-email">
                Email
              </label>
              <input
                id="auth-email"
                className="field__input"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="field">
              <label className="field__label" htmlFor="auth-password">
                Пароль
              </label>
              <input
                id="auth-password"
                className="field__input"
                type="password"
                autoComplete={mode === 'register' ? 'new-password' : 'current-password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {error && <div className="alert alert--error">{error}</div>}
            <div className="row row--actions">
              <button className="btn btn--accent auth__submit" type="submit" disabled={busy}>
                {busy ? '…' : mode === 'register' ? 'Зарегистрироваться' : 'Войти'}
              </button>
            </div>
          </form>

          {sso?.enabled && (
            <div className="auth__sso">
              <div className="auth__divider">
                <span>или</span>
              </div>
              <a className="btn btn--ghost auth__sso-btn" href={authApi.ssoUrl('/')}>
                Войти через {sso.provider}
              </a>
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
