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
  for (const name of ['session', 'csrf', 'csrf_admin']) {
    document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
  }
})

describe('logAction', () => {
  it('без сессии и токена не отправляет аудит', () => {
    const fn = fetchMock()
    logAction({ action: 'project.created' })
    expect(fn).not.toHaveBeenCalled()
  })

  // session-cookie HttpOnly и из JS не видна: признак входа — csrf-cookie,
  // который сервер выставляет вместе с сессией (SEC-0003).
  it('с csrf-cookie шлёт POST /api/v1/audit с телом и credentials', () => {
    const fn = fetchMock()
    fn.mockResolvedValue({ ok: true })
    document.cookie = 'csrf=csrf-1; path=/'
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
    document.cookie = 'csrf=csrf-1; path=/'
    expect(() => logAction({ action: 'auth.login' })).not.toThrow()
    await Promise.resolve()
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('добавляет X-CSRF-Token из cookie csrf', () => {
    const fn = fetchMock()
    fn.mockResolvedValue({ ok: true })
    document.cookie = 'csrf=csrf-1; path=/'
    logAction({ action: 'auth.logout' })
    expect(fn.mock.calls[0][1].headers['X-CSRF-Token']).toBe('csrf-1')
  })

  // Регрессия (волна 0): админка слала аудит без X-App-Origin и со store-CSRF,
  // поэтому сервер искал session (а не session_admin) → 401/403 и потери событий.
  it('для origin=admin шлёт X-App-Origin и admin csrf-токен', () => {
    const fn = fetchMock()
    fn.mockResolvedValue({ ok: true })
    document.cookie = 'csrf_admin=csrf-admin-1; path=/'
    logAction({ action: 'stair.config_changed', resource_type: 'stair' }, 'admin')
    const headers = fn.mock.calls[0][1].headers
    expect(headers['X-App-Origin']).toBe('admin')
    expect(headers['X-CSRF-Token']).toBe('csrf-admin-1')
  })

  it('для origin=store не прикладывает admin csrf-токен', () => {
    const fn = fetchMock()
    fn.mockResolvedValue({ ok: true })
    document.cookie = 'csrf=csrf-store-1; path=/'
    document.cookie = 'csrf_admin=csrf-admin-1; path=/'
    logAction({ action: 'quote.requested' })
    const headers = fn.mock.calls[0][1].headers
    expect(headers['X-App-Origin']).toBe('store')
    expect(headers['X-CSRF-Token']).toBe('csrf-store-1')
  })
})