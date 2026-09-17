import { describe, expect, it, vi, afterEach } from 'vitest'
import * as client from './client'

describe('authApi', () => {
  afterEach(() => vi.restoreAllMocks())

  it('register posts and returns user', async () => {
    const user = { id: 'u1', email: 'a@ex.ru' }
    vi.spyOn(client, 'post').mockResolvedValue({ user, token: 't' } as any)
    const { authApi } = await import('./auth')
    const got = await authApi.register({ email: 'a@ex.ru', name: 'A', password: 'p' })
    expect(got).toEqual(user)
    expect(client.post).toHaveBeenCalledWith('/api/v1/auth/register', expect.any(Object))
  })

  it('login returns user', async () => {
    const user = { id: 'u1' }
    vi.spyOn(client, 'post').mockResolvedValue({ user } as any)
    const { authApi } = await import('./auth')
    expect(await authApi.login({ email: 'a@ex.ru', password: 'p' })).toEqual(user)
  })

  it('logout posts', async () => {
    vi.spyOn(client, 'post').mockResolvedValue(undefined as any)
    const { authApi } = await import('./auth')
    await authApi.logout()
    expect(client.post).toHaveBeenCalledWith('/api/v1/auth/logout', {})
  })

  it('me gets', async () => {
    vi.spyOn(client, 'get').mockResolvedValue({ id: 'u1' } as any)
    const { authApi } = await import('./auth')
    expect(await authApi.me()).toEqual({ id: 'u1' })
  })

  it('ssoUrl encodes', async () => {
    const { authApi } = await import('./auth')
    expect(authApi.ssoUrl('/a b')).toBe('/api/v1/auth/sso?redirect=%2Fa%20b')
    expect(authApi.ssoUrl()).toBe('/api/v1/auth/sso?redirect=%2F')
  })

  it('ssoConfig gets', async () => {
    vi.spyOn(client, 'get').mockResolvedValue({ enabled: true } as any)
    const { authApi } = await import('./auth')
    expect(await authApi.ssoConfig()).toEqual({ enabled: true })
  })
})
