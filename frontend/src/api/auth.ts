// Auth API (SEC-0003): регистрация, вход, выход, текущий пользователь.
// Сессионная cookie выставляется сервером (httpOnly); csrf — доступная JS.
// SSO (EDR-0017): OIDC Authorization Code flow.

import { get, post } from './client'
import { createAuthApi, type AuthApi } from '@shared/api/auth'

// re-export для совместимости с существующими импортами `../api/auth`
export type { RegisterRequest, LoginRequest } from '@shared/api/auth'

export interface SsoConfig {
  enabled: boolean
  provider: string
}

export const authApi: AuthApi & { ssoConfig: () => Promise<SsoConfig>; ssoUrl: (redirect?: string) => string } = {
  ...createAuthApi({
    // ленивый доступ к live-bindings, чтобы vi.spyOn(client, ...) работал
    get: (url) => get(url),
    post: (url, body) => post(url, body),
  }),
  ssoConfig: () => get<SsoConfig>('/api/v1/auth/sso/config'),
  ssoUrl: (redirect = '/') =>
    `/api/v1/auth/sso?redirect=${encodeURIComponent(redirect)}`,
}