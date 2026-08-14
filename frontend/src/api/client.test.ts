import { afterEach, describe, expect, it, vi } from 'vitest'
import { get, post } from './client'
import { ApiError } from './types'

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

describe('get', () => {
  it('возвращает распарсенный JSON при 200', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, '{"id":"p1"}'))
    const data = await get<{ id: string }>('/api/v1/projects')
    expect(data).toEqual({ id: 'p1' })
  })

  it('возвращает undefined при 204', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(204))
    await expect(get('/x')).resolves.toBeUndefined()
  })

  it('возвращает null при пустом теле успешного ответа', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200))
    const data = await get<null>('/x')
    expect(data).toBeNull()
  })

  it('прокидывает Content-Type', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, '{}'))
    await get('/x')
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/x')
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
  })

  it('кидает ApiError с данными тела ошибки', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(
      jsonResponse(404, JSON.stringify({ error: { code: 'not_found', message: 'Проект не найден' } })),
    )
    const err = (await get('/missing').catch((e: unknown) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(404)
    expect(err.code).toBe('not_found')
    expect(err.message).toBe('Проект не найден')
  })

  it('кидает ApiError unknown/HTTP при не-JSON ошибке', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(500, 'internal error'))
    const err = (await get('/x').catch((e: unknown) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe('unknown')
    expect(err.message).toBe('HTTP 500')
  })
})

describe('post', () => {
  it('шлёт метод POST и JSON-тело', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(201, '{"id":"p1"}'))
    const out = await post('/api/v1/projects', { name: 'n' })
    expect(out).toEqual({ id: 'p1' })
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/projects')
    expect(init.method).toBe('POST')
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(init.body).toBe(JSON.stringify({ name: 'n' }))
  })

  it('передаёт ошибки через ApiError', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(
      jsonResponse(400, JSON.stringify({ error: { code: 'validation', message: 'Плохой запрос' } })),
    )
    const err = (await post('/x', {}).catch((e: unknown) => e)) as ApiError
    expect(err.code).toBe('validation')
    expect(err.message).toBe('Плохой запрос')
  })
})