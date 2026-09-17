import { describe, expect, it, vi, afterEach } from 'vitest'
import * as client from './client'

describe('projectsApi', () => {
  afterEach(() => vi.restoreAllMocks())

  it('list maps PaginatedResponse', async () => {
    vi.spyOn(client, 'get').mockResolvedValue({ data: [{ id: 'p1' }] } as any)
    const { projectsApi } = await import('./projects')
    expect(await projectsApi.list()).toEqual([{ id: 'p1' }])
  })
  it('create posts', async () => {
    vi.spyOn(client, 'post').mockResolvedValue({ id: 'p1' } as any)
    const { projectsApi } = await import('./projects')
    expect(await projectsApi.create({ name: 'n' } as any)).toEqual({ id: 'p1' })
  })
  it('get fetches', async () => {
    vi.spyOn(client, 'get').mockResolvedValue({ id: 'p1' } as any)
    const { projectsApi } = await import('./projects')
    expect(await projectsApi.get('p1')).toEqual({ id: 'p1' })
  })
  it('exportUrl sync', async () => {
    const { projectsApi } = await import('./projects')
    expect(projectsApi.exportUrl('p1')).toBe('/api/v1/projects/p1/export')
  })
  it('listMembers', async () => {
    vi.spyOn(client, 'get').mockResolvedValue([{ id: 'm1' }] as any)
    const { projectsApi } = await import('./projects')
    expect(await projectsApi.listMembers('p1')).toEqual([{ id: 'm1' }])
  })
  it('addMember / updateMemberRole / removeMember', async () => {
    vi.spyOn(client, 'post').mockResolvedValue({ status: 'ok' } as any)
    vi.spyOn(client, 'patch').mockResolvedValue({ status: 'ok' } as any)
    vi.spyOn(client, 'del').mockResolvedValue(undefined as any)
    const { projectsApi } = await import('./projects')
    await projectsApi.addMember('p1', { email: 'a@ex.ru' } as any)
    await projectsApi.updateMemberRole('p1', 'u1', 'viewer' as any)
    await projectsApi.removeMember('p1', 'u1')
    expect(client.post).toHaveBeenCalled()
    expect(client.patch).toHaveBeenCalled()
    expect(client.del).toHaveBeenCalled()
  })
  it('comments', async () => {
    vi.spyOn(client, 'get').mockResolvedValue({ data: [{ id: 'c1' }] } as any)
    vi.spyOn(client, 'post').mockResolvedValue({ id: 'c1' } as any)
    vi.spyOn(client, 'del').mockResolvedValue(undefined as any)
    const { projectsApi } = await import('./projects')
    expect(await projectsApi.listComments('p1')).toEqual([{ id: 'c1' }])
    await projectsApi.addComment('p1', { body: 'hi' } as any)
    await projectsApi.deleteComment('p1', 'c1')
  })
  it('reviews', async () => {
    vi.spyOn(client, 'post').mockResolvedValue({ id: 'r1' } as any)
    vi.spyOn(client, 'get').mockResolvedValue({ data: [{ id: 'r1' }] } as any)
    const { projectsApi } = await import('./projects')
    await projectsApi.requestReview('p1', { body: 'x' } as any)
    expect(await projectsApi.listReviews('p1')).toEqual([{ id: 'r1' }])
  })
  it('configurations', async () => {
    vi.spyOn(client, 'get').mockResolvedValue([{ id: 'cfg1' }] as any)
    vi.spyOn(client, 'post').mockResolvedValue({ id: 'cfg1' } as any)
    const { projectsApi } = await import('./projects')
    expect(await projectsApi.listConfigurations('p1')).toEqual([{ id: 'cfg1' }])
    await projectsApi.restoreConfiguration('p1', 'cfg1')
  })
})
