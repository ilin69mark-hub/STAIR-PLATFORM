import { createContext, useContext } from 'react'
import type { LoginRequest, RegisterRequest } from '../api/auth'
import type { User } from '@shared/types'

export interface AuthContextValue {
  user: User | null
  loading: boolean
  login: (req: LoginRequest) => Promise<User>
  register: (req: RegisterRequest) => Promise<User>
  logout: () => Promise<void>
  clear: () => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return ctx
}
