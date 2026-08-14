import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { VersionsPanel } from './VersionsPanel'
import { projectsApi } from '../api/projects'
import { ApiError, type Configuration, type ProjectMember, type User } from '../api/types'
import { AuthContext, type AuthContextValue } from '../auth/context'

const authCtx: AuthContextValue = {
  user: { id: 'u-owner' } as User,
  loading: false,
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
  clear: vi.fn(),
}

const member: ProjectMember = {
  project_id: 'p1',
  user_id: 'u-owner',
  role: 'owner',
  created_at: '2026-08-14T12:00:00Z',
}

const makeConfig = (overrides: Partial<Configuration>): Configuration => ({
  id: 'cfg-1',
  project_id: 'p1',
  revision: 1,
  width_mm: 1000,
  height_mm: 2600,
  flight: 'straight',
  step_height_mm: 170,
  stringer_thickness_mm: 5,
  step_thickness_mm: 4,
  clearance_mm: 3,
  railing_height_mm: 900,
  comfort_step_mm: 630,
  landing_width_mm: 0,
  lower_step_count: 0,
  outer_radius_mm: 0,
  current: false,
  created_at: '2026-08-14T12:00:00Z',
  ...overrides,
})

function renderPanel() {
  return render(
    <AuthContext.Provider value={authCtx}>
      <VersionsPanel projectId="p1" />
    </AuthContext.Provider>,
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('VersionsPanel', () => {
  it('показывает список ревизий и признак текущей', async () => {
    vi.spyOn(projectsApi, 'listConfigurations').mockResolvedValue([
      makeConfig({ id: 'cfg-1', revision: 1, current: false }),
      makeConfig({ id: 'cfg-2', revision: 2, current: true, height_mm: 2700 }),
    ])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([member])
    renderPanel()
    expect(await screen.findByText('Версия 1 · Прямой')).toBeInTheDocument()
    expect(screen.getByText('Версия 2 · Прямой · текущая')).toBeInTheDocument()
  })

  it('пустое состояние', async () => {
    vi.spyOn(projectsApi, 'listConfigurations').mockResolvedValue([])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([member])
    renderPanel()
    expect(await screen.findByText('Версий пока нет.')).toBeInTheDocument()
  })

  it('восстанавливает версию и показывает успех', async () => {
    vi.spyOn(projectsApi, 'listConfigurations').mockResolvedValue([
      makeConfig({ id: 'cfg-1', revision: 1, current: false }),
      makeConfig({ id: 'cfg-2', revision: 2, current: true }),
    ])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([member])
    const restore = vi
      .spyOn(projectsApi, 'restoreConfiguration')
      .mockResolvedValue(makeConfig({ id: 'cfg-1', revision: 1, current: true }))
    renderPanel()
    const btn = await screen.findByRole('button', { name: 'Восстановить' })
    fireEvent.click(btn)
    expect(restore).toHaveBeenCalledWith('p1', 'cfg-1')
    expect(await screen.findByText('Версия восстановлена.')).toBeInTheDocument()
  })

  it('viewer не видит кнопку восстановления', async () => {
    vi.spyOn(projectsApi, 'listConfigurations').mockResolvedValue([
      makeConfig({ id: 'cfg-1', revision: 1, current: false }),
    ])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([
      { ...member, user_id: 'u-other', role: 'viewer' },
    ])
    authCtx.user = { id: 'u-other' } as User
    renderPanel()
    await screen.findByText('Версия 1 · Прямой')
    expect(screen.queryByRole('button', { name: 'Восстановить' })).toBeNull()
    authCtx.user = { id: 'u-owner' } as User
  })

  it('показывает ошибку API', async () => {
    vi.spyOn(projectsApi, 'listConfigurations').mockRejectedValue(
      new ApiError(404, 'not_found', 'Проект не найден'),
    )
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([member])
    renderPanel()
    expect(await screen.findByText('Проект не найден')).toBeInTheDocument()
  })
})