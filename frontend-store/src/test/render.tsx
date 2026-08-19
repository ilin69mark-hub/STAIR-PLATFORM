import { render } from '@testing-library/react'
import { vi } from 'vitest'
import { AuthProvider } from '../auth/AuthContext'
import { authApi, type LoginRequest, type RegisterRequest } from '../api/auth'
import type { User } from '@shared/types'

export const testUser: User = {
  id: 'u-1',
  email: 'buyer@example.com',
  name: 'Иван Покупатель',
  role: 'user',
  tenant_id: 't-1',
}

export function mockMe(user: User | null = testUser) {
  return vi.spyOn(authApi, 'me').mockResolvedValue(user as User)
}

export function mockLogin(user: User = testUser) {
  return vi.spyOn(authApi, 'login').mockResolvedValue(user)
}

export function mockRegister(user: User = testUser) {
  return vi.spyOn(authApi, 'register').mockResolvedValue(user)
}

// Рендер с реальным AuthProvider: сессия поднимается через мок /me.
export async function renderWithAuth(ui: React.ReactElement, user: User | null = testUser) {
  mockMe(user)
  const utils = render(<AuthProvider>{ui}</AuthProvider>)
  return utils
}

export type { LoginRequest, RegisterRequest }