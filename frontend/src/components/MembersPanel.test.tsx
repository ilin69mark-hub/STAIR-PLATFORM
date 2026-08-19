import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MembersPanel } from './MembersPanel'
import { projectsApi } from '../api/projects'
import { ApiError, type ProjectMember, type User } from '@shared/types'
import { AuthContext, type AuthContextValue } from '../auth/context'

const authCtx: AuthContextValue = {
  user: { id: 'u-owner' } as User,
  loading: false,
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
  clear: vi.fn(),
}

function renderPanel() {
  return render(
    <AuthContext.Provider value={authCtx}>
      <MembersPanel projectId="p1" />
    </AuthContext.Provider>,
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('MembersPanel', () => {
  it('показывает участников и признак владельца', async () => {
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([
      { user_id: 'u-owner', role: 'owner' } as ProjectMember,
      { user_id: 'u-2', role: 'viewer' } as ProjectMember,
    ])
    renderPanel()
    expect(await screen.findByText('u-owner')).toBeInTheDocument()
    expect(screen.getByText('u-2')).toBeInTheDocument()
    expect(screen.getAllByText('Владелец').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Наблюдатель').length).toBeGreaterThan(0)
  })

  it('отвечает 403 — показывает ошибку', async () => {
    vi.spyOn(projectsApi, 'listMembers').mockRejectedValue(
      new ApiError(403, 'forbidden', 'Доступ запрещён'),
    )
    renderPanel()
    expect(await screen.findByText('Доступ запрещён')).toBeInTheDocument()
  })

  it('owner может добавить участника', async () => {
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([
      { user_id: 'u-owner', role: 'owner' } as ProjectMember,
    ])
    const add = vi.spyOn(projectsApi, 'addMember').mockResolvedValue({ status: 'ok' })
    const list = vi
      .spyOn(projectsApi, 'listMembers')
      .mockResolvedValueOnce([{ user_id: 'u-owner', role: 'owner' } as ProjectMember])
      .mockResolvedValueOnce([
        { user_id: 'u-owner', role: 'owner' } as ProjectMember,
        { user_id: 'u-2', role: 'editor' } as ProjectMember,
      ])

    renderPanel()
    await screen.findByText('u-owner')
    fireEvent.change(screen.getByPlaceholderText('Email участника'), {
      target: { value: 'u-2' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Добавить' }))

    waitFor(() => expect(add).toHaveBeenCalled())
    expect(await screen.findByText('u-2')).toBeInTheDocument()
    expect(list).toHaveBeenCalledTimes(2)
  })

  it('не-owner видит роли, но без кнопок управления', async () => {
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([
      { user_id: 'u-owner', role: 'owner' } as ProjectMember,
      { user_id: 'u-2', role: 'editor' } as ProjectMember,
    ])
    const { user } = authCtx
    authCtx.user = { id: 'u-viewer' } as User
    renderPanel()
    expect(await screen.findByText('u-owner')).toBeInTheDocument()
    expect(screen.queryByText('Добавить')).not.toBeInTheDocument()
    authCtx.user = user
  })
})