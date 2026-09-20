// Централизованная реакция на 401 (S3-2, EDR-0012 Session Management):
// «сессия протухла → единый редирект на логин». Один владелец логики — этот
// файл; админ и store не дублируют её в своих AuthContext.
//
// Поток:
//   api client (admin/store) ловит res.status === 401 на ЗАЩИЩЁННОМ запросе
//   → fireUnauthorized().
//   AuthProvider каждого приложения подписан через onUnauthorized(() => clear()):
//   user → null, App рендерит login (admin: AuthPage; store: #cabinet →
//   AuthForm). Редирект «на логин» — это и есть переход в !user-состояние.

type UnauthorizedHandler = () => void

let handlers: UnauthorizedHandler[] = []

// onUnauthorized подписывает обработчик на событие «401 по защищённому
// эндпоинту». Возвращает функцию отписки (для strict-mode-cleanup в dev).
export function onUnauthorized(handler: UnauthorizedHandler): () => void {
  handlers = [...handlers, handler]
  return () => {
    handlers = handlers.filter((h) => h !== handler)
  }
}

// fireUnauthorized вызывает всех подписчиков (сброс сессии во всех
// подписанных приложениях). Перехватывает исключения подписчиков, чтобы один
// сбойный обработчик не ломал сброс в остальных.
export function fireUnauthorized(): void {
  const snapshot = [...handlers]
  for (const h of snapshot) {
    try {
      h()
    } catch {
      // один упавший обработчик не должен ронять остальных подписчиков
    }
  }
}

// isAuthEndpoint — true для /api/v1/auth/*: их 401 = «неверные/повторные
// креды», а НЕ истёкшая сессия. Для auth-вызовов 401 обрабатывается формой
// логина штатно, без сброса — иначе зацикливание (провалился логин → сброс →
// снова логин).
const AUTH_ENDPOINT_RE = /\/api\/v1\/auth\//

export function isAuthEndpoint(url: string): boolean {
  return AUTH_ENDPOINT_RE.test(url)
}
