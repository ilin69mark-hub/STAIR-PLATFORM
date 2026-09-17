// Общая логика double-submit CSRF-токена (SEC-0003).
// Токен читается из cookie и прикладывается заголовком X-CSRF-Token на
// мутирующие запросы. Имена cookie виртуально origin-специфичны (локал
// «localhost» — host-only, RFC 6265 §5.1.3, порт в scope не входит):
// admin-приложение (:5174) читает «csrf_admin», store (:3000, дефолт) —
// «csrf». Выбор origin делается один раз на клиент и передаётся сюда.

export const CSRF_COOKIE = 'csrf'
export const CSRF_HEADER = 'X-CSRF-Token'

// adminOriginCookieSuffix — суффикс имени csrf-cookie админ-приложения.
const adminOriginCookieSuffix = '_admin'

export type AppOrigin = 'store' | 'admin'

// csrfCookieFor возвращает имя csrf-cookie для приложения (origin).
function csrfCookieFor(origin: AppOrigin): string {
  return origin === 'admin' ? `${CSRF_COOKIE}${adminOriginCookieSuffix}` : CSRF_COOKIE
}

// csrfToken возвращает CSRF-токен из cookie ('' — токена нет).
export function csrfToken(origin: AppOrigin = 'store'): string {
  const name = csrfCookieFor(origin)
  return (
    document.cookie
      .split('; ')
      .find((c) => c.startsWith(`${name}=`))
      ?.slice(name.length + 1) ?? ''
  )
}

// csrfHeaders возвращает заголовок X-CSRF-Token, если токен есть.
// Используется только на мутирующих запросах (double-submit, SEC-0003).
export function csrfHeaders(origin: AppOrigin = 'store'): Record<string, string> {
  const token = csrfToken(origin)
  return token ? { [CSRF_HEADER]: token } : {}
}
