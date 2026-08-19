import { useState, type FormEvent } from 'react'
import { useAuth } from '../auth/context'
import { apiErrorMessage } from '../auth/errors'

type Mode = 'login' | 'register'

interface Props {
  note?: string
  submitLabel?: string
}

// Встроенная форма входа/регистрации: используется перед созданием заказа,
// чтобы получить сессию и CSRF-токен.
export function AuthForm({ note, submitLabel }: Props) {
  const { login, register } = useAuth()
  const [mode, setMode] = useState<Mode>('login')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)

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
      setDone(true)
    } catch (err) {
      setError(apiErrorMessage(err, 'Не удалось выполнить запрос'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="panel">
      {done ? (
        <div className="alert alert--ok" role="status">
          {mode === 'register' ? 'Регистрация прошла успешно.' : 'Вы вошли.'} Теперь можно
          оформить заказ.
        </div>
      ) : (
        <>
          <h2>{mode === 'login' ? 'Вход' : 'Регистрация'}</h2>
          <div className="auth-tabs">
            <button
              type="button"
              className={mode === 'login' ? 'active' : ''}
              onClick={() => switchMode('login')}
            >
              Вход
            </button>
            <button
              type="button"
              className={mode === 'register' ? 'active' : ''}
              onClick={() => switchMode('register')}
            >
              Регистрация
            </button>
          </div>
          {note && <p className="sub">{note}</p>}
          <form className="auth-form" onSubmit={handleSubmit}>
            {mode === 'register' && (
              <div className="field">
                <label htmlFor="auth-name">Имя</label>
                <input
                  id="auth-name"
                  type="text"
                  autoComplete="name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>
            )}
            <div className="field">
              <label htmlFor="auth-email">Email</label>
              <input
                id="auth-email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor="auth-password">Пароль</label>
              <input
                id="auth-password"
                type="password"
                autoComplete={mode === 'register' ? 'new-password' : 'current-password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {error && <div className="alert alert--error">{error}</div>}
            <div className="actions">
              <button className="sp-btn sp-btn--primary" type="submit" disabled={busy}>
                {busy ? '…' : submitLabel ?? (mode === 'register' ? 'Зарегистрироваться' : 'Войти')}
              </button>
            </div>
          </form>
        </>
      )}
    </div>
  )
}