// Auth API (SEC-0003): общий клиент регистрации/входа/выхода/профиля для
// обоих приложений (admin и store). Раньше файл целиком дублировался в
// frontend/src/api/auth.ts и frontend-store/src/api/auth.ts; SSO-методы
// (EDR-0017) остаются только в admin-обёртке.

import type { User } from '../types'

export interface RegisterRequest {
  email: string
  name: string
  password: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface AuthResponse {
  user: User
  token: string
}

// Client — минимальный срез API-клиента, нужный auth (get/post).
// Каждое приложение передаёт свой origin-aware клиент. Методы вызываются
// лениво (в момент вызова), чтобы vi.spyOn на модуле client работал.
export interface AuthApiClient {
  get: (url: string) => Promise<unknown>
  post: (url: string, body: unknown) => Promise<unknown>
}

// createAuthApi собирает authApi поверх заданного клиента.
export function createAuthApi(client: AuthApiClient) {
  return {
    register: (body: RegisterRequest) =>
      client.post('/api/v1/auth/register', body).then((r) => (r as AuthResponse).user),
    login: (body: LoginRequest) =>
      client.post('/api/v1/auth/login', body).then((r) => (r as AuthResponse).user),
    logout: () => client.post('/api/v1/auth/logout', {}),
    me: () => client.get('/api/v1/auth/me').then((r) => r as User),
  }
}

export type AuthApi = ReturnType<typeof createAuthApi>