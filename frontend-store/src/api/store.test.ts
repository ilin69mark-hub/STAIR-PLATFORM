import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@shared/types'
import type { QuoteResult } from '@shared/types'
import * as client from './client'
import { quoteApi } from './store'

afterEach(() => {
  vi.restoreAllMocks()
})

const okQuote = { validation: { valid: true, blocking: false, issues: [] } } as QuoteResult

describe('quoteApi.calculate', () => {
  it('возвращает результат с первого раза', async () => {
    const post = vi.spyOn(client, 'post').mockResolvedValue(okQuote)
    await expect(quoteApi.calculate({ width_mm: 900 })).resolves.toBe(okQuote)
    expect(post).toHaveBeenCalledWith('/api/v1/public/stairs:quote', { width_mm: 900 })
  })

  it('ретраит при 429 (dedup) и возвращает результат', async () => {
    vi.useFakeTimers()
    try {
      const post = vi
        .spyOn(client, 'post')
        .mockRejectedValueOnce(new ApiError(429, 'rate_limited', 'Слишком много запросов'))
        .mockResolvedValueOnce(okQuote)
      const p = quoteApi.calculate({ width_mm: 900 })
      await vi.advanceTimersByTimeAsync(500)
      await expect(p).resolves.toBe(okQuote)
      expect(post).toHaveBeenCalledTimes(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('не ретраит при других ошибках', async () => {
    const post = vi
      .spyOn(client, 'post')
      .mockRejectedValue(new ApiError(500, 'internal', 'Внутренняя ошибка'))
    await expect(quoteApi.calculate({})).rejects.toBeInstanceOf(ApiError)
    expect(post).toHaveBeenCalledTimes(1)
  })
})