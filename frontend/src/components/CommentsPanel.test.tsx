import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { CommentsPanel } from './CommentsPanel'
import { projectsApi } from '../api/projects'
import { ApiError, type ProjectComment, type User } from '../api/types'
import { AuthContext, type AuthContextValue } from '../auth/context'

const authCtx: AuthContextValue = {
  user: { id: 'u-owner' } as User,
  loading: false,
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
  clear: vi.fn(),
}

const makeComment = (overrides: Partial<ProjectComment> = {}): ProjectComment => ({
  id: 'c-1',
  project_id: 'p1',
  author_id: 'u-owner',
  body: 'Сделать перила выше?',
  created_at: '2026-08-14T12:00:00Z',
  ...overrides,
})

function renderPanel() {
  return render(
    <AuthContext.Provider value={authCtx}>
      <CommentsPanel projectId="p1" />
    </AuthContext.Provider>,
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('CommentsPanel', () => {
  it('показывает комментарии', async () => {
    vi.spyOn(projectsApi, 'listComments').mockResolvedValue([
      makeComment(),
      makeComment({ id: 'c-2', author_id: 'u-2', body: 'Согласен' }),
    ])
    renderPanel()
    expect(await screen.findByText('Сделать перила выше?')).toBeInTheDocument()
    expect(screen.getByText('Согласен')).toBeInTheDocument()
  })

  it('показывает пустое состояние', async () => {
    vi.spyOn(projectsApi, 'listComments').mockResolvedValue([])
    renderPanel()
    expect(await screen.findByText('Комментариев пока нет.')).toBeInTheDocument()
  })

  it('отправляет комментарий и очищает поле', async () => {
    vi.spyOn(projectsApi, 'listComments').mockResolvedValue([])
    const add = vi
      .spyOn(projectsApi, 'addComment')
      .mockResolvedValue(makeComment())
    renderPanel()
    await screen.findByText('Комментариев пока нет.')
    fireEvent.change(screen.getByPlaceholderText('Комментарий…'), {
      target: { value: 'Уменьшить зазор' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Отправить' }))
    expect(add).toHaveBeenCalledWith('p1', { body: 'Уменьшить зазор' })
  })

  it('показывает ошибку API', async () => {
    vi.spyOn(projectsApi, 'listComments').mockRejectedValue(
      new ApiError(403, 'forbidden', 'Нет доступа'),
    )
    renderPanel()
    expect(await screen.findByText('Нет доступа')).toBeInTheDocument()
  })
})