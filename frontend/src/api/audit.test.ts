// Аудит (Phase G, EDR-0013): реальные HTTP-обёртки auditApi + подписи.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { actionLabels, auditApi } from './audit'

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

describe('auditApi', () => {
  it('listProjectAudit шлёт GET /api/v1/projects/{id}/audit и парсит события', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([{ id: 'e-1', action: 'auth.login' }])))
    const events = await auditApi.listProjectAudit('p-1')
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/projects/p-1/audit')
    expect(init.method).toBeUndefined()
    expect(events).toEqual([{ id: 'e-1', action: 'auth.login' }])
  })

  it('listTenantAudit шлёт GET /api/v1/audit', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([{ id: 'e-2', action: 'user.role_changed' }])))
    const events = await auditApi.listTenantAudit()
    expect(fn.mock.calls[0][0]).toBe('/api/v1/audit')
    expect(events).toHaveLength(1)
  })

  it('кидает ApiError при не-2xx', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(
      jsonResponse(403, JSON.stringify({ error: { code: 'forbidden', message: 'Нет доступа' } })),
    )
    const err = (await auditApi.listTenantAudit().catch((e: unknown) => e)) as { code: string }
    expect(err.code).toBe('forbidden')
  })

  it('actionLabels содержит подписи ключевых действий аудита', () => {
    expect(actionLabels['auth.register']).toBe('Регистрация')
    expect(actionLabels['user.role_changed']).toBe('Смена роли пользователя')
    expect(actionLabels['api_key.created']).toBe('Создание API-ключа')
    expect(actionLabels['stair.calculated']).toBe('Расчёт лестницы')
    expect(actionLabels['review.signed']).toBe('Подпись ревью')
  })
})