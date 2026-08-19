// API-клиент v1 (FE-0011 API Client): тонкие fetch-обёртки к REST-эндпоинтам
// Go-бэкенда. Ошибки нормализуются в ApiError { status, code, message }.
// Сессионная cookie передаётся автоматически (same-origin через vite proxy);
// на мутирующие запросы добавляется X-CSRF-Token (double-submit, SEC-0003).

import type { ApiErrorBody } from '@shared/types'
import { ApiError } from '@shared/types'

const CSRF_COOKIE = 'csrf'

function csrfToken(): string {
  return document.cookie
    .split('; ')
    .find((c) => c.startsWith(`${CSRF_COOKIE}=`))
    ?.slice(CSRF_COOKIE.length + 1) ?? ''
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string> | undefined),
  }
  const method = (init?.method ?? 'GET').toUpperCase()
  const isMutating = !['GET', 'HEAD', 'OPTIONS'].includes(method)
  if (isMutating) {
    const csrf = csrfToken()
    if (csrf) headers['X-CSRF-Token'] = csrf
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