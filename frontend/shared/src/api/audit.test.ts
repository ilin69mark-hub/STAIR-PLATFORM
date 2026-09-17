// logAction (EDR-0013 §4.2): клиентский аудит действий best-effort.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { logAction } from './audit'

// jsdom в этом окружении отдаёт нерабочий localStorage (без getItem/setItem),
// из-за чего logAction уходит в catch{} и всё равно шлёт запрос. Для тестов
// подставляем детерминированное in-memory хранилище.
class MemoryStorage {
  private map = new Map<string, string>()
  get length() {
    return this.map.size
  }
  getItem(key: string) {
    return this.map.has(key) ? (this.map.get(key) as string) : null
  }
  setItem(key: string, value: string) {
    this.map.set(key, String(value))
  }
  removeItem(key: string) {
    this.map.delete(key)
  }
  clear() {
    this.map.clear()
  }
  key(i: number) {
    return Array.from(this.map.keys())[i] ?? null
  }
}

function fetchMock() {
  const fn = vi.fn()
  vi.stubGlobal('fetch', fn)
  return fn
}

beforeEach(() => {
  vi.stubGlobal('localStorage', new MemoryStorage())
})

afterEach(() => {
  vi.unstubAllGlobals()
  document.cookie = 'session=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/'
  document.cookie = 'csrf=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/'
})

describe('logAction', () => {
  it('без сессии и токена не отправляет аудит', () => {
    const fn = fetchMock()
    logAction({ action: 'project.created' })
    expect(fn).not.toHaveBeenCalled()
  })

  it('с cookie сессии шлёт POST /api/v1/audit с телом и credentials', () => {
    const fn = fetchMock()
    fn.mockResolvedValue({ ok: true })
    document.cookie = 'session=abc; path=/'
    logAction({
      action: 'stair.config_changed',
      resource_type: 'stair',
      resource_id: 'p-1',
      detail: '{"user":true}',
    })
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe('/api/v1/audit')
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('include')
    expect(init.headers['Content-Type']).toBe('application/json')
    expect(init.body).toBe(
      JSON.stringify({
        action: 'stair.config_changed',
        resource_type: 'stair',
        resource_id: 'p-1',
        detail: '{"user":true}',
      }),
    )
  })

  it('с токеном в localStorage отправляет аудит', () => {
    const fn = fetchMock()
    fn.mockResolvedValue({ ok: true })
    localStorage.setItem('token', 't-1')
    logAction({ action: 'api_key.created' })
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('тихо игнорирует ошибку сети (best-effort)', async () => {
    const fn = fetchMock()
    fn.mockRejectedValue(new Error('network'))
    document.cookie = 'session=abc; path=/'
    expect(() => logAction({ action: 'auth.login' })).not.toThrow()
    await Promise.resolve()
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('добавляет X-CSRF-Token из cookie csrf', () => {
    const fn = fetchMock()
    fn.mockResolvedValue({ ok: true })
    document.cookie = 'session=abc; path=/'
    document.cookie = 'csrf=csrf-1; path=/'
    logAction({ action: 'auth.logout' })
    expect(fn.mock.calls[0][1].headers['X-CSRF-Token']).toBe('csrf-1')
  })
})