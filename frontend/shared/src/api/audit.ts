// API-клиент аудита действий (EDR-0013 §4.2): клиентские события
// (клики, применение вариантов/советов, изменение полей) отправляются
// на POST /api/v1/audit. Актор и tenant берутся на сервере из сессии —
// в теле передаём только действие и контекст. Отправка best-effort:
// сбой журнала не должен ломать UX пользователя.

export interface LogActionInput {
  action: string
  resource_type?: string
  resource_id?: string
  detail?: string
}

const CSRF_COOKIE = 'csrf'

function csrfToken(): string {
  return (
    document.cookie
      .split('; ')
      .find((c) => c.startsWith(`${CSRF_COOKIE}=`))
      ?.slice(CSRF_COOKIE.length + 1) ?? ''
  )
}

export function logAction(input: LogActionInput): void {
  try {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    const csrf = csrfToken()
    if (csrf) headers['X-CSRF-Token'] = csrf
    void fetch('/api/v1/audit', {
      method: 'POST',
      credentials: 'include',
      headers,
      body: JSON.stringify(input),
    }).catch(() => {
      // best-effort: тихо игнорируем ошибки аудита
    })
  } catch {
    // best-effort: тихо игнорируем ошибки аудита
  }
}
