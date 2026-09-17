import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApprovalsPanel } from './ApprovalsPanel'
import { projectsApi } from '../api/projects'
import { ApiError, type ConfigurationApproval } from '@shared/types'

const makeApproval = (overrides: Partial<ConfigurationApproval> = {}): ConfigurationApproval => ({
  id: 'a-1',
  project_id: 'p1',
  configuration_id: 'cfg-9',
  approved_by: 'u-owner',
  comment: 'итоговая',
  created_at: '2026-08-14T12:00:00Z',
  ...overrides,
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('ApprovalsPanel', () => {
  it('показывает историю утверждений', async () => {
    vi.spyOn(projectsApi, 'listApprovals').mockResolvedValue([
      makeApproval(),
      makeApproval({ id: 'a-2', approved_by: 'u-2', comment: 'согласен' }),
    ])
    render(<ApprovalsPanel projectId="p1" configurationId="cfg-9" />)
    expect(await screen.findByText('итоговая')).toBeInTheDocument()
    expect(screen.getByText('согласен')).toBeInTheDocument()
    expect(screen.getByText('Утвердил: u-owner')).toBeInTheDocument()
  })

  it('показывает пустое состояние', async () => {
    vi.spyOn(projectsApi, 'listApprovals').mockResolvedValue([])
    render(<ApprovalsPanel projectId="p1" />)
    expect(await screen.findByText('Ревизий ещё не утверждали.')).toBeInTheDocument()
  })

  it('скрывает форму без configurationId', async () => {
    vi.spyOn(projectsApi, 'listApprovals').mockResolvedValue([])
    render(<ApprovalsPanel projectId="p1" />)
    await screen.findByText('Ревизий ещё не утверждали.')
    expect(screen.queryByRole('button', { name: 'Утвердить ревизию' })).toBeNull()
  })

  it('утверждает ревизию и показывает успех', async () => {
    vi.spyOn(projectsApi, 'listApprovals').mockResolvedValue([])
    const approve = vi
      .spyOn(projectsApi, 'approveConfiguration')
      .mockResolvedValue(makeApproval())
    render(<ApprovalsPanel projectId="p1" configurationId="cfg-9" />)
    await screen.findByText('Ревизий ещё не утверждали.')
    fireEvent.change(screen.getByPlaceholderText('Комментарий (необязательно)…'), {
      target: { value: 'итоговая' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Утвердить ревизию' }))
    expect(approve).toHaveBeenCalledWith('p1', 'cfg-9', { comment: 'итоговая' })
    expect(await screen.findByText('Ревизия утверждена.')).toBeInTheDocument()
  })

  it('показывает ошибку API', async () => {
    vi.spyOn(projectsApi, 'listApprovals').mockRejectedValue(
      new ApiError(404, 'not_found', 'Проект не найден'),
    )
    render(<ApprovalsPanel projectId="p1" configurationId="cfg-9" />)
    expect(await screen.findByText('Проект не найден')).toBeInTheDocument()
  })
})