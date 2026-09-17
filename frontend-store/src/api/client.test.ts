import { afterEach, describe, expect, it, vi } from 'vitest'
import { get, post } from './client'
import { ApiError } from '@shared/types'

function fetchMock() {
  const fn = vi.fn()
  vi.stubGlobal('fetch', fn)
  return fn
}
function jsonResponse(status: number, body?: string) {
  return { status, ok: status >= 200 && status < 300, text: vi.fn().mockResolvedValue(body ?? '') }
}
afterEach(() => vi.unstubAllGlobals())

describe('store client', () => {
  it('get returns JSON', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, '{"id":"p1"}'))
    expect(await get('/x')).toEqual({ id: 'p1' })
  })
  it('204 undefined', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(204))
    await expect(get('/x')).resolves.toBeUndefined()
  })
  it('credentials include', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, '{}'))
    await get('/x')
    expect(fn.mock.calls[0][1].credentials).toBe('include')
  })
  it('no CSRF on GET', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, '{}'))
    document.cookie = 'csrf=abc; path=/'
    await get('/x')
    expect((fn.mock.calls[0][1].headers as any)['X-CSRF-Token']).toBeUndefined()
  })
  it('ApiError on 404', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(404, JSON.stringify({ error: { code: 'not_found', message: 'no' } })))
    const err = (await get('/x').catch((e: any) => e)) as ApiError
    expect(err.status).toBe(404)
    expect(err.code).toBe('not_found')
  })
  it('post adds CSRF', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(201, '{}'))
    document.cookie = 'csrf=secret; path=/'
    await post('/api/v1/orders', { a: 1 })
    expect((fn.mock.calls[0][1].headers as any)['X-CSRF-Token']).toBe('secret')
  })
  it('post 400 ApiError', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(400, JSON.stringify({ error: { code: 'validation', message: 'bad' } })))
    const err = (await post('/x', {}).catch((e: any) => e)) as ApiError
    expect(err.code).toBe('validation')
  })
})
