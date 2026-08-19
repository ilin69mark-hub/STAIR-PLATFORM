import { ApiError } from '@shared/types'

// apiErrorMessage извлекает человекочитаемое сообщение об ошибке.
export function apiErrorMessage(e: unknown, fallback: string): string {
  return e instanceof ApiError ? e.message : fallback
}
