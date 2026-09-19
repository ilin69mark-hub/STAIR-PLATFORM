// Auth API (SEC-0003): регистрация, вход, выход, текущий пользователь.
// Сессионная cookie выставляется сервером (httpOnly); csrf — доступная JS.

import { get, post } from './client'
import { createAuthApi } from '@shared/api/auth'

// re-export для совместимости с существующими импортами `../api/auth`
export type { RegisterRequest, LoginRequest } from '@shared/api/auth'

export const authApi = createAuthApi({
  // ленивый доступ к live-bindings, чтобы vi.spyOn(client, ...) работал
  get: (url) => get(url),
  post: (url, body) => post(url, body),
})