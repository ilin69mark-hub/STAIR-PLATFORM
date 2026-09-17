// Аналитика админки (Phase F, EDR-0028): реальные HTTP-обёртки analyticsApi.
// Проверяем построение query-string, URL и десериализацию всех 4 метрик.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { analyticsApi } from './analytics'

function fetchMock() {
  const fn = vi.fn()
  vi.stubGlobal('fetch', fn)
  return fn
}

function jsonResponse(status: number, body?: string) {
  return {
    status,
    ok: status >= 200 && status < 300,
    text: vi.fn().mockResolvedValue(body ?? ''),
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('analyticsApi', () => {
  it('usage без параметров не добавляет query-string', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(
      jsonResponse(200, JSON.stringify({ from: '2026-09-01', to: '2026-09-17', granularity: 'day' })),
    )
    const data = await analyticsApi.usage()
    expect(fn.mock.calls[0][0]).toBe('/api/v1/admin/analytics/usage')
    expect(data.granularity).toBe('day')
  })

  it('usage с параметрами экранирует их в URLSearchParams', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ granularity: 'week' })))
    await analyticsApi.usage({ from: '2026-09-01', to: '2026-09-17', granularity: 'week' })
    const url = fn.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/admin/analytics/usage?')
    const qs = new URLSearchParams(url.split('?')[1])
    expect(qs.get('granularity')).toBe('week')
    expect(qs.get('from')).toBe('2026-09-01')
  })

  it('usage отбрасывает пустые параметры', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({})))
    await analyticsApi.usage({ from: '', to: '' })
    expect(fn.mock.calls[0][0]).toBe('/api/v1/admin/analytics/usage')
  })

  it('projects шлёт GET /api/v1/admin/analytics/projects c from/to', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ from: 'a', to: 'b' })))
    const data = await analyticsApi.projects({ from: 'a', to: 'b' })
    const url = fn.mock.calls[0][0] as string
    expect(url).toMatch(/^\/api\/v1\/admin\/analytics\/projects\?/)
    expect(data).toEqual({ from: 'a', to: 'b' })
  })

  it('manufacturing шлёт GET /api/v1/admin/analytics/manufacturing', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ granularity: 'month' })))
    const data = await analyticsApi.manufacturing({ granularity: 'month' })
    const url = fn.mock.calls[0][0] as string
    expect(url).toContain('/api/v1/admin/analytics/manufacturing')
    expect(new URLSearchParams(url.split('?')[1]).get('granularity')).toBe('month')
    expect(data.granularity).toBe('month')
  })

  it('cost шлёт GET /api/v1/admin/analytics/cost и парсит метрики', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(
      jsonResponse(200, JSON.stringify({ granularity: 'day', totals: { final_price: 1234 } })),
    )
    const data = await analyticsApi.cost({ granularity: 'day' })
    expect(fn.mock.calls[0][0]).toBe('/api/v1/admin/analytics/cost?granularity=day')
    expect(data.totals.final_price).toBe(1234)
  })

  it('кидает ApiError через client при не-2xx', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(
      jsonResponse(403, JSON.stringify({ error: { code: 'forbidden', message: 'Нет доступа' } })),
    )
    const err = (await analyticsApi.usage().catch((e: unknown) => e)) as { code: string }
    expect(err.code).toBe('forbidden')
  })
})