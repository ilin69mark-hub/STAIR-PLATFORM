import { ApiError } from '@shared/types'

// apiErrorMessage извлекает человекочитаемое сообщение об ошибке.
export function apiErrorMessage(e: unknown, fallback: string): string {
  return e instanceof ApiError ? e.message : fallback
}

/** Код ошибки для аналитики: код ответа, а не текст. Текст меняется вместе с
 *  формулировками и в отчёте превратился бы в «почти разные» причины. */
export function apiErrorCode(e: unknown): string {
  if (e instanceof ApiError) return String(e.status)
  return 'network'
}
