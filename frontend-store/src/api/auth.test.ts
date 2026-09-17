import { describe, expect, it, vi, afterEach } from 'vitest'
import * as client from './client'

describe('store authApi', () => {
  afterEach(() => vi.restoreAllMocks())
  it('register', async () => {
    vi.spyOn(client, 'post').mockResolvedValue({ user: { id: 'u1' } } as any)
    const { authApi } = await import('./auth')
    expect(await authApi.register({ email: 'a@ex.ru', name: 'A', password: 'p' })).toEqual({ id: 'u1' })
  })
  it('login', async () => {
    vi.spyOn(client, 'post').mockResolvedValue({ user: { id: 'u1' } } as any)
    const { authApi } = await import('./auth')
    expect(await authApi.login({ email: 'a@ex.ru', password: 'p' })).toEqual({ id: 'u1' })
  })
  it('logout', async () => {
    vi.spyOn(client, 'post').mockResolvedValue(undefined as any)
    const { authApi } = await import('./auth')
    await authApi.logout()
    expect(client.post).toHaveBeenCalled()
  })
  it('me', async () => {
    vi.spyOn(client, 'get').mockResolvedValue({ id: 'u1' } as any)
    const { authApi } = await import('./auth')
    expect(await authApi.me()).toEqual({ id: 'u1' })
  })
})
