// API-клиент v1 (FE-0011 API Client): тонкие fetch-обёртки к REST-эндпоинтам
// Go-бэкенда. Ошибки нормализуются в ApiError { status, code, message }.

import type { ApiErrorBody } from './types'
import { ApiError } from './types'

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
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