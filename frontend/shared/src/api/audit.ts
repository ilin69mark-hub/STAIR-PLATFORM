// API-клиент аудита действий (EDR-0013 §4.2): клиентские события
// (клики, применение вариантов/советов, изменение полей) отправляются
// на POST /api/v1/audit. Актор и tenant берутся на сервере из сессии —
// в теле передаём только действие и контекст. Отправка best-effort:
// сбой журнала не должен ломать UX пользователя.

import { csrfHeaders } from './csrf'

export interface LogActionInput {
  action: string
  resource_type?: string
  resource_id?: string
  detail?: string
}

export function logAction(input: LogActionInput): void {
  try {
    void fetch('/api/v1/audit', {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        ...csrfHeaders(),
      },
      body: JSON.stringify(input),
    }).catch(() => {
      // best-effort: тихо игнорируем ошибки аудита
    })
  } catch {
    // best-effort: тихо игнорируем ошибки аудита
  }
}
