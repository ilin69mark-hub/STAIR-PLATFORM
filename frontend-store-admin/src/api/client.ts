// API-клиент панели магазина: тонкие fetch-обёртки к admin-ручкам бэкенда.
// Сессионная cookie передаётся автоматически (same-origin через vite proxy),
// на мутирующие запросы добавляется X-CSRF-Token (double-submit, SEC-0003).
//
// Панель магазина работает с origin «admin» (session_admin/csrf_admin) —
// тем же, что внутренняя админка платформы: отдельный origin заведём, когда
// роль владельца магазина отделится от роли оператора платформы; права на
// store-разделы всё равно проверяет бэкенд (store.* permissions).

import { csrfHeaders, type AppOrigin } from '@shared/api/csrf'

const APP_ORIGIN: AppOrigin = 'admin'

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

interface ApiErrorBody {
  error?: { code?: string; message?: string }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const method = (init?.method ?? 'GET').toUpperCase()
  const isMutating = !['GET', 'HEAD', 'OPTIONS'].includes(method)
  const res = await fetch(url, {
    credentials: 'include',
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-App-Origin': APP_ORIGIN,
      ...(isMutating ? csrfHeaders(APP_ORIGIN) : {}),
      ...(init?.headers as Record<string, string> | undefined),
    },
  })

  if (res.status === 204) return undefined as T

  const text = await res.text()
  let payload: unknown = null
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = null
    }
  }

  if (!res.ok) {
    const body = payload as ApiErrorBody | null
    throw new ApiError(
      res.status,
      body?.error?.code ?? 'unknown',
      body?.error?.message ?? `HTTP ${res.status}`,
    )
  }
  return payload as T
}

export const api = {
  get: <T>(url: string): Promise<T> => request<T>(url),
  put: <T>(url: string, body: unknown): Promise<T> =>
    request<T>(url, { method: 'PUT', body: JSON.stringify(body) }),
  post: <T>(url: string, body: unknown): Promise<T> =>
    request<T>(url, { method: 'POST', body: JSON.stringify(body) }),
  patch: <T>(url: string, body: unknown): Promise<T> =>
    request<T>(url, { method: 'PATCH', body: JSON.stringify(body) }),
  delete: (url: string): Promise<void> => request<void>(url, { method: 'DELETE' }),
}
