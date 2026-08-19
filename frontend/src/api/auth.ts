// Auth API (SEC-0003): регистрация, вход, выход, текущий пользователь.
// Сессионная cookie выставляется сервером (httpOnly); csrf — доступная JS.
// SSO (EDR-0017): OIDC Authorization Code flow.

import { get, post } from './client'
import type { User } from '@shared/types'

export interface RegisterRequest {
  email: string
  name: string
  password: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface SsoConfig {
  enabled: boolean
  provider: string
}

export const authApi = {
  register: (body: RegisterRequest) => post<User>('/api/v1/auth/register', body),
  login: (body: LoginRequest) => post<User>('/api/v1/auth/login', body),
  logout: () => post<undefined>('/api/v1/auth/logout', {}),
  me: () => get<User>('/api/v1/auth/me'),
  ssoConfig: () => get<SsoConfig>('/api/v1/auth/sso/config'),
  ssoUrl: (redirect = '/') =>
    `/api/v1/auth/sso?redirect=${encodeURIComponent(redirect)}`,
}
