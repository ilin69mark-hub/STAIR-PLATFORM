import { describe, expect, it } from 'vitest'

import { ApiError } from '@shared/types'

import { apiErrorMessage } from './errors'

describe('apiErrorMessage', () => {
  it('отдаёт сообщение ApiError', () => {
    expect(apiErrorMessage(new ApiError(403, 'forbidden', 'нет прав'), 'fallback')).toBe('нет прав')
  })

  it('подставляет fallback для всего остального', () => {
    // Сервер может прислать что угодно: не-ApiError, строку, undefined.
    // Показывать пользователю сырое тело ответа нельзя, но и молчать нельзя —
    // нужен текст-заглушка.
    expect(apiErrorMessage('ECONNRESET', 'Сеть недоступна')).toBe('Сеть недоступна')
    expect(apiErrorMessage(new Error('boom'), 'Сеть недоступна')).toBe('Сеть недоступна')
    expect(apiErrorMessage(undefined, 'Сеть недоступна')).toBe('Сеть недоступна')
    expect(apiErrorMessage(null, 'Сеть недоступна')).toBe('Сеть недоступна')
    expect(apiErrorMessage(0, 'Сеть недоступна')).toBe('Сеть недоступна')
  })

  it('не путает ApiError с похожим объектом', () => {
    // instanceof обязан отличать настоящую ошибку API от простого объекта с
    // полями message и status — иначе в интерфейс попал бы текст сервера
    // без кода ответа.
    const impostor = { message: 'подделка', status: 500 }
    expect(apiErrorMessage(impostor, 'fallback')).toBe('fallback')
  })

  it('пустое сообщение ApiError не подменяется fallback', () => {
    // Сообщение пришло пустым — показываем как есть: это данные сервера,
    // и подмена была бы уже нашим вымыслом.
    expect(apiErrorMessage(new ApiError(500, 'boom', ''), 'fallback')).toBe('')
  })
})