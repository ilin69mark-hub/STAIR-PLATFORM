// API-клиент аудита действий (EDR-0013 §4.2): клиентские события
// (клики, применение вариантов/советов, изменение полей) отправляются
// на POST /api/v1/audit. Актор и tenant берутся на сервере из сессии —
// в теле передаём только действие и контекст. Отправка best-effort:
// сбой журнала не должен ломать UX пользователя.

import { csrfHeaders, csrfToken, type AppOrigin } from './csrf'

export interface LogActionInput {
  action: string
  resource_type?: string
  resource_id?: string
  detail?: string
}

const APP_ORIGIN_HEADER = 'X-App-Origin'

// logAction отправляет клиентское событие в журнал аудита.
//
// origin обязателен: приложения различаются не только именем session-cookie,
// но и CSRF-токеном (store: «csrf», admin: «csrf_admin»), поэтому без
// origin-параметра админка слала бы запрос со store-токеном и получала 403,
// а сервер искал бы не ту session-cookie.
export function logAction(input: LogActionInput, origin: AppOrigin = 'store'): void {
  // Признак аутентификации — CSRF-cookie нужного origin: session-cookie
  // HttpOnly и из JS не видна (SEC-0003), а csrf-cookie выставляется вместе
  // с сессией и читается. Анонимный store (public quote) не отправляет аудит.
  let token = ''
  try {
    token = csrfToken(origin)
    const hasToken = typeof localStorage !== 'undefined' && !!localStorage.getItem('token')
    if (!token && !hasToken) return
  } catch {
    return
  }
  try {
    void fetch('/api/v1/audit', {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        [APP_ORIGIN_HEADER]: origin,
        ...(token ? csrfHeaders(origin) : {}),
      },
      body: JSON.stringify(input),
    }).catch(() => {
      // best-effort: тихо игнорируем ошибки аудита
    })
  } catch {
    // best-effort: тихо игнорируем ошибки аудита
  }
}
