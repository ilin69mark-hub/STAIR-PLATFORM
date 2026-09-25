import { useCallback, useState } from 'react'
import { api } from './client'

export interface SessionUser {
  id: string
  email: string
  name: string
  role: string
}

interface AuthResponse {
  user: SessionUser
}

// session — текущий пользователь панели (null — не вошёл).
export function useSession(): {
  user: SessionUser | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => void
  refresh: () => Promise<void>
} {
  const [user, setUser] = useState<SessionUser | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    try {
      setUser(await api.get<SessionUser>('/api/v1/auth/me'))
    } catch {
      setUser(null)
    } finally {
      setLoading(false)
    }
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const res = await api.post<AuthResponse>('/api/v1/auth/login', { email, password })
    setUser(res.user)
  }, [])

  const logout = useCallback(() => {
    void api.post('/api/v1/auth/logout', {}).finally(() => setUser(null))
  }, [])

  return { user, loading, login, logout, refresh }
}
