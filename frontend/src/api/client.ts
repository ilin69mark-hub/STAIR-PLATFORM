// API-клиент v1 (FE-0011 API Client): тонкие fetch-обёртки к REST-эндпоинтам
// Go-бэкенда. Ошибки нормализуются в ApiError { status, code, message }.
// Сессионная cookie передаётся автоматически (same-origin через vite proxy);
// на мутирующие запросы добавляется X-CSRF-Token (double-submit, SEC-0003).

import type { ApiErrorBody } from '@shared/types'
import { ApiError } from '@shared/types'
import { csrfHeaders, type AppOrigin } from '@shared/api/csrf'

// Admin-приложение (:5174) — origin «admin»: сервер пишет сессионные cookie
// с суффиксом «_admin» (session_admin/csrf_admin), чтобы не делить host-only
// cookie с store (:3000) — порт в scope не входит (RFC 6265 §5.1.3). Origin
// сообщается заголовком X-App-Origin на каждый запрос.
const APP_ORIGIN: AppOrigin = 'admin'

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const method = (init?.method ?? 'GET').toUpperCase()
  const isMutating = !['GET', 'HEAD', 'OPTIONS'].includes(method)
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-App-Origin': APP_ORIGIN,
    ...(isMutating ? csrfHeaders(APP_ORIGIN) : {}),
    ...(init?.headers as Record<string, string> | undefined),
  }

  const res = await fetch(url, {
    credentials: 'include',
    ...init,
    headers,
  })

  if (res.status === 204) {
    return undefined as T
  }

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

const jsonHeaders = { 'Content-Type': 'application/json' }

export const get = <T>(url: string): Promise<T> => request<T>(url)

export const post = <T>(url: string, body: unknown): Promise<T> =>
  request<T>(url, { method: 'POST', headers: jsonHeaders, body: JSON.stringify(body) })

export const patch = <T>(url: string, body: unknown): Promise<T> =>
  request<T>(url, { method: 'PATCH', headers: jsonHeaders, body: JSON.stringify(body) })

export const put = <T>(url: string, body: unknown): Promise<T> =>
  request<T>(url, { method: 'PUT', headers: jsonHeaders, body: JSON.stringify(body) })

export const del = <T = undefined>(url: string): Promise<T> =>
  request<T>(url, { method: 'DELETE' })