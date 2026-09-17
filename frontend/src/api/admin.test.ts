// Админ-API (Phase G, EDR-0016): реальные HTTP-обёртки adminApi.
// Мокаем глобальный fetch (аналогично client.test.ts) и проверяем URL,
// метод, тело и десериализацию каждого эндпоинта.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { adminApi } from './admin'
import type { ApiKey } from '@shared/types'

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

describe('adminApi', () => {
  it('overview шлёт GET /api/v1/admin/overview и парсит JSON', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(
      jsonResponse(200, JSON.stringify({ tenant_id: 't-1', users: 2, active_users: 2 })),
    )
    const data = await adminApi.overview()
    expect(fn.mock.calls[0][0]).toBe('/api/v1/admin/overview')
    expect(fn.mock.calls[0][1].method).toBeUndefined()
    expect(data).toEqual({ tenant_id: 't-1', users: 2, active_users: 2 })
  })

  it('listUsers шлёт GET /api/v1/admin/users', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([{ id: 'u-1', role: 'user' }])))
    const users = await adminApi.listUsers()
    expect(fn.mock.calls[0][0]).toBe('/api/v1/admin/users')
    expect(users).toEqual([{ id: 'u-1', role: 'user' }])
  })

  it('updateUser шлёт PATCH c телом role/status', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ status: 'ok' })))
    const body = { role: 'admin' as const, status: 'active' as const }
    const out = await adminApi.updateUser('u-2', body)
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/admin/users/u-2')
    expect(init.method).toBe('PATCH')
    expect(init.body).toBe(JSON.stringify(body))
    expect(out).toEqual({ status: 'ok' })
  })

  it('getSettings шлёт GET /api/v1/admin/settings', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ min_password_length: 8 })))
    const policy = await adminApi.getSettings()
    expect(fn.mock.calls[0][0]).toBe('/api/v1/admin/settings')
    expect(policy).toEqual({ min_password_length: 8 })
  })

  it('updateSettings шлёт PUT с телом политики', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ min_password_length: 10 })))
    await adminApi.updateSettings({ min_password_length: 10 } as never)
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/admin/settings')
    expect(init.method).toBe('PUT')
    expect(init.body).toBe(JSON.stringify({ min_password_length: 10 }))
  })

  it('exportUrl строит URL с scope и format', () => {
    expect(adminApi.exportUrl('users', 'json')).toBe('/api/v1/admin/export?scope=users&format=json')
    expect(adminApi.exportUrl('audit', 'csv')).toBe('/api/v1/admin/export?scope=audit&format=csv')
  })

  it('listApiKeys шлёт GET /api/v1/admin/api-keys', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([{ id: 'key-1', name: 'CI' }])))
    const keys = await adminApi.listApiKeys()
    expect(fn.mock.calls[0][0]).toBe('/api/v1/admin/api-keys')
    expect(keys).toEqual([{ id: 'key-1', name: 'CI' }])
  })

  it('createApiKey шлёт POST и возвращает токен один раз', async () => {
    const fn = fetchMock()
    const created: ApiKey & { token?: string } = {
      id: 'key-2',
      name: 'CI',
      scopes: ['projects.list'],
      created_at: '2026-09-17T00:00:00Z',
      token: 'sk-secret-once',
    }
    fn.mockResolvedValue(jsonResponse(201, JSON.stringify(created)))
    const out = await adminApi.createApiKey({ name: 'CI', scopes: ['projects.list'] })
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/admin/api-keys')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify({ name: 'CI', scopes: ['projects.list'] }))
    expect(out.token).toBe('sk-secret-once')
  })

  it('revokeApiKey шлёт DELETE /api/v1/admin/api-keys/{id}', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(204))
    await adminApi.revokeApiKey('key-9')
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/admin/api-keys/key-9')
    expect(init.method).toBe('DELETE')
  })

  it('listOrders шлёт GET /api/v1/admin/orders', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([{ id: 'o-1' }])))
    const orders = await adminApi.listOrders()
    expect(fn.mock.calls[0][0]).toBe('/api/v1/admin/orders')
    expect(orders).toEqual([{ id: 'o-1' }])
  })

  it('updateOrderStatus шлёт PATCH /api/v1/admin/orders/{id}/status', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ id: 'o-1', status: 'approved' })))
    await adminApi.updateOrderStatus('o-1', 'approved')
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/admin/orders/o-1/status')
    expect(init.method).toBe('PATCH')
    expect(init.body).toBe(JSON.stringify({ status: 'approved' }))
  })

  it('listTestimonials шлёт GET /api/v1/admin/testimonials', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([{ id: 't-1' }])))
    const items = await adminApi.listTestimonials()
    expect(fn.mock.calls[0][0]).toBe('/api/v1/admin/testimonials')
    expect(items).toEqual([{ id: 't-1' }])
  })

  it('createTestimonial шлёт POST c телом', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(201, JSON.stringify({ id: 't-2', published: false })))
    const body = { author: 'Иван', text: 'Отлично', rating: 5 }
    const out = await adminApi.createTestimonial(body)
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/admin/testimonials')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify(body))
    expect(out).toEqual({ id: 't-2', published: false })
  })

  it('updateTestimonial шлёт PATCH /api/v1/admin/testimonials/{id}', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ id: 't-2', published: true })))
    await adminApi.updateTestimonial('t-2', { published: true } as never)
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/admin/testimonials/t-2')
    expect(init.method).toBe('PATCH')
  })

  it('deleteTestimonial шлёт DELETE /api/v1/admin/testimonials/{id}', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(204))
    await adminApi.deleteTestimonial('t-2')
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/admin/testimonials/t-2')
    expect(init.method).toBe('DELETE')
  })
})