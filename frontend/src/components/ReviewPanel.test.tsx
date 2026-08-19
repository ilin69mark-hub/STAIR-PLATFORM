import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ReviewPanel } from './ReviewPanel'
import { projectsApi } from '../api/projects'
import { ApiError, type ProjectMember, type ProjectReview, type User } from '@shared/types'
import { AuthContext, type AuthContextValue } from '../auth/context'

const authCtx = (user: Partial<User>): AuthContextValue => ({
  user: { id: 'u-owner', ...user } as User,
  loading: false,
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
  clear: vi.fn(),
})

const makeReview = (overrides: Partial<ProjectReview> = {}): ProjectReview => ({
  id: 'rv-1',
  project_id: 'p1',
  requester_id: 'u-editor',
  reviewer_id: '',
  decision: 'requested',
  comment: 'проверьте расчёт',
  created_at: '2026-08-14T12:00:00Z',
  decided_at: null,
  ...overrides,
})

const makeMember = (overrides: Partial<ProjectMember> = {}): ProjectMember => ({
  project_id: 'p1',
  user_id: 'u-owner',
  role: 'owner',
  created_at: '2026-08-14T12:00:00Z',
  ...overrides,
})

function renderPanel(status: 'draft' | 'in_review' | 'approved' | 'changes_requested' = 'draft') {
  const onStatusChange = vi.fn()
  const view = render(
    <AuthContext.Provider value={authCtx({})}>
      <ReviewPanel projectId="p1" status={status} onStatusChange={onStatusChange} />
    </AuthContext.Provider>,
  )
  return { onStatusChange, view }
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('ReviewPanel', () => {
  it('показывает статус и кнопку запроса ревью для owner', async () => {
    vi.spyOn(projectsApi, 'listReviews').mockResolvedValue([])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([makeMember()])
    renderPanel('draft')
    expect(await screen.findByText('Черновик')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Запросить ревью' })).toBeInTheDocument()
  })

  it('owner запрашивает ревью и обновляет статус на in_review', async () => {
    vi.spyOn(projectsApi, 'listReviews').mockResolvedValue([])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([makeMember()])
    const request = vi.spyOn(projectsApi, 'requestReview').mockResolvedValue(makeReview())
    const { onStatusChange } = renderPanel('draft')
    await screen.findByText('Черновик')
    fireEvent.change(screen.getByPlaceholderText('Комментарий (необязательно)…'), {
      target: { value: 'проверьте' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Запросить ревью' }))
    expect(request).toHaveBeenCalledWith('p1', { comment: 'проверьте' })
    await waitFor(() => expect(onStatusChange).toHaveBeenCalledWith('in_review'))
  })

  it('owner подписывает ревью из статуса in_review', async () => {
    vi.spyOn(projectsApi, 'listReviews').mockResolvedValue([makeReview()])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([makeMember()])
    const sign = vi
      .spyOn(projectsApi, 'signOffReview')
      .mockResolvedValue(makeReview({ decision: 'approved', decided_at: '2026-08-14T13:00:00Z' }))
    const { onStatusChange } = renderPanel('in_review')
    await screen.findByText('На ревью')
    fireEvent.click(screen.getByRole('button', { name: 'Подписать' }))
    expect(sign).toHaveBeenCalledWith('p1', 'rv-1', { comment: '' })
    await waitFor(() => expect(onStatusChange).toHaveBeenCalledWith('approved'))
  })

  it('owner возвращает ревью на доработку', async () => {
    vi.spyOn(projectsApi, 'listReviews').mockResolvedValue([makeReview()])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([makeMember()])
    const changes = vi
      .spyOn(projectsApi, 'requestChanges')
      .mockResolvedValue(makeReview({ decision: 'changes_requested', decided_at: '2026-08-14T13:00:00Z' }))
    const { onStatusChange } = renderPanel('in_review')
    await screen.findByText('На ревью')
    fireEvent.click(screen.getByRole('button', { name: 'Вернуть на доработку' }))
    expect(changes).toHaveBeenCalledWith('p1', 'rv-1', { comment: '' })
    await waitFor(() => expect(onStatusChange).toHaveBeenCalledWith('changes_requested'))
  })

  it('editor не видит кнопки решения, только запрос', async () => {
    vi.spyOn(projectsApi, 'listReviews').mockResolvedValue([makeReview()])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([
      makeMember({ user_id: 'u-editor', role: 'editor' }),
    ])
    renderPanel('in_review')
    await screen.findByText('На ревью')
    expect(screen.queryByRole('button', { name: 'Подписать' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Запросить ревью' })).not.toBeInTheDocument()
  })

  it('показывает историю ревью', async () => {
    vi.spyOn(projectsApi, 'listReviews').mockResolvedValue([
      makeReview({ decision: 'requested', id: 'rv-1' }),
      makeReview({
        decision: 'approved',
        id: 'rv-2',
        reviewer_id: 'u-owner',
        decided_at: '2026-08-14T13:00:00Z',
      }),
    ])
    vi.spyOn(projectsApi, 'listMembers').mockResolvedValue([makeMember()])
    renderPanel('approved')
    expect(await screen.findByText('Запрошено')).toBeInTheDocument()
    expect(screen.getAllByText('Подписано').length).toBeGreaterThanOrEqual(2)
  })

  it('показывает ошибку API', async () => {
    vi.spyOn(projectsApi, 'listReviews').mockRejectedValue(
      new ApiError(403, 'forbidden', 'Нет доступа'),
    )
    vi.spyOn(projectsApi, 'listMembers').mockRejectedValue(
      new ApiError(403, 'forbidden', 'Нет доступа'),
    )
    renderPanel('draft')
    expect(await screen.findByText('Нет доступа')).toBeInTheDocument()
  })
})