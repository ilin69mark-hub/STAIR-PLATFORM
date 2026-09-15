// Общая логика double-submit CSRF-токена (SEC-0003).
// Токен читается из cookie и прикладывается заголовком X-CSRF-Token на
// мутирующие запросы. Единый источник — один раз описан здесь,
// используется клиентом v1 и аудит-клиентом.

export const CSRF_COOKIE = 'csrf'
export const CSRF_HEADER = 'X-CSRF-Token'

// csrfToken возвращает CSRF-токен из cookie ('' — токена нет).
export function csrfToken(): string {
  return (
    document.cookie
      .split('; ')
      .find((c) => c.startsWith(`${CSRF_COOKIE}=`))
      ?.slice(CSRF_COOKIE.length + 1) ?? ''
  )
}

// csrfHeaders возвращает заголовок X-CSRF-Token, если токен есть.
// Используется только на мутирующих запросах (double-submit, SEC-0003).
export function csrfHeaders(): Record<string, string> {
  const token = csrfToken()
  return token ? { [CSRF_HEADER]: token } : {}
}