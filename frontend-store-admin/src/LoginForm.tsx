import { useState, type FormEvent } from 'react'

// LoginForm — вход владельца магазина. Панель отдельным приложением, но
// использует admin-origin (session_admin/csrf_admin), поэтому отдельная
// регистрация не нужна — учётная запись общая с админкой платформы.
export function LoginForm({
  onLogin,
  error,
}: {
  onLogin: (email: string, password: string) => Promise<void>
  error: string
}) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    try {
      await onLogin(email, password)
    } catch {
      // Сообщение показывает вызывающий: он держит error.
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="login" onSubmit={(e) => void submit(e)}>
      <h1>Вход в панель магазина</h1>
      <label>
        Email
        <input type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} />
      </label>
      <label>
        Пароль
        <input
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </label>
      {error && <p className="error">{error}</p>}
      <button type="submit" disabled={busy || !email || !password}>
        {busy ? 'Входим…' : 'Войти'}
      </button>
    </form>
  )
}
