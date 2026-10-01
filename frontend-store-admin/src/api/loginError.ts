import { ApiError } from './client'

export function loginErrorMessage(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.status === 401) return 'Неверный email или пароль.'
    if (e.status === 403) return 'У пользователя нет прав администратора магазина.'
    return e.message
  }
  return 'Не удалось войти. Проверьте соединение с сервером.'
}
