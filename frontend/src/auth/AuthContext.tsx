import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { authApi, type LoginRequest, type RegisterRequest } from '../api/auth'
import type { User } from '@shared/types'
import { AuthContext } from './context'
import { onUnauthorized } from '@shared/api/session'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  // При монтировании проверяем сессию (GET /auth/me). 401 → неавторизован.
  useEffect(() => {
    let cancelled = false
    authApi
      .me()
      .then((u) => {
        if (!cancelled) setUser(u)
      })
      .catch(() => {
        if (!cancelled) setUser(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback(async (req: LoginRequest) => {
    const u = await authApi.login(req)
    setUser(u)
    return u
  }, [])

  const register = useCallback(async (req: RegisterRequest) => {
    const u = await authApi.register(req)
    setUser(u)
    return u
  }, [])

  const logout = useCallback(async () => {
    try {
      await authApi.logout()
    } finally {
      setUser(null)
    }
  }, [])

  const clear = useCallback(() => setUser(null), [])

  // S3-2 (EDR-0040): подписка на шину 401 → сброс user → App рендерит логин.
  // useEffect возвращает функцию отписки (cleanup) — в dev strict-mode 
  // подписка не дублируется.
  useEffect(() => onUnauthorized(clear), [clear])

  const value = useMemo(
    () => ({ user, loading, login, register, logout, clear }),
    [user, loading, login, register, logout, clear],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
