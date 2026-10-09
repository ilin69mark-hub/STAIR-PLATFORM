// Передача владения проектом в панели участников.
//
// Проверяем решения, из-за которых передача вообще применима: участник — это
// человек (имя и почта, а не UUID), передать можно только участнику проекта,
// и пока не выбран получатель — кнопка не работает.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ProjectMember } from '@shared/types'

import { MembersPanel } from './MembersPanel'
import { projectsApi } from '../api/projects'
import { AuthContext, type AuthContextValue } from '../auth/context'
import { ApiError, type User } from '@shared/types'

const members: ProjectMember[] = [
  {
    project_id: 'p1',
    user_id: 'u-owner',
    role: 'owner',
    created_at: '2026-10-01T00:00:00Z',
    name: 'Иван Петров',
    email: 'ivan@example.ru',
  },
  {
    project_id: 'p1',
    user_id: 'u-editor',
    role: 'editor',
    created_at: '2026-10-01T01:00:00Z',
    name: 'Мария',
    email: 'maria@example.ru',
  },
]

const ownerUser = { id: 'u-owner', email: 'ivan@example.ru', tenant_id: 't-1' }

const transferOwnership = vi.fn(async () => ({ status: 'ok' }))
const listMembers = vi.fn(async () => members)

const authCtx: AuthContextValue = {
  user: { id: ownerUser.id, email: ownerUser.email, tenant_id: ownerUser.tenant_id } as User,
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

beforeEach(() => {
  vi.clearAllMocks()
  listMembers.mockResolvedValue(members)
  transferOwnership.mockResolvedValue({ status: 'ok' })
  vi.spyOn(projectsApi, 'listMembers').mockImplementation(listMembers)
  vi.spyOn(projectsApi, 'transferOwnership').mockImplementation(transferOwnership)
})

describe('передача владения проектом', () => {
  it('показывает участника человеком, а не идентификатором', async () => {
    renderPanel()
    await screen.findByText('Иван Петров — ivan@example.ru')
    expect(screen.queryByText('u-owner')).not.toBeInTheDocument()
    expect(screen.getByText('Мария — maria@example.ru')).toBeInTheDocument()
  })

  it('владельцу предлагает только участников проекта', async () => {
    renderPanel()
    const select = await screen.findByLabelText('Новый владелец проекта')
    // Владельца в списке нет: передавать «самому себе» нечего, а новый
    // владелец всё равно один.
    expect(select.querySelectorAll('option')).toHaveLength(2) // пустой + Мария
    expect(screen.getByRole('option', { name: /maria@example.ru/ })).toBeInTheDocument()
  })

  it('не передаёт, пока не выбран получатель', async () => {
    renderPanel()
    const button = await screen.findByRole('button', { name: 'Передать' })
    expect(button).toBeDisabled()

    fireEvent.click(button)
    expect(transferOwnership).not.toHaveBeenCalled()
  })

  it('передаёт владение выбранному участнику', async () => {
    renderPanel()
    const select = await screen.findByLabelText('Новый владелец проекта')
    fireEvent.change(select, { target: { value: 'u-editor' } })

    const button = screen.getByRole('button', { name: 'Передать' })
    await waitFor(() => expect(button).toBeEnabled())
    fireEvent.click(button)

    await waitFor(() => expect(transferOwnership).toHaveBeenCalledWith('p1', 'u-editor'))
    // После передачи список перечитывается: роли в нём уже другие.
    await waitFor(() => expect(listMembers).toHaveBeenCalledTimes(2))
  })

  it('предупреждает, что прежний владелец станет редактором', async () => {
    renderPanel()
    await screen.findByLabelText('Новый владелец проекта')
    expect(
      screen.getByText(/прежний владелец станет редактором/i),
    ).toBeInTheDocument()
  })

  it('показывает ошибку сервера и не сбрасывает выбор', async () => {
    transferOwnership.mockRejectedValueOnce(
      new ApiError(404, 'not_found', 'Проект или участник не найдены.'),
    )
    renderPanel()
    const select = await screen.findByLabelText('Новый владелец проекта')
    fireEvent.change(select, { target: { value: 'u-editor' } })
    fireEvent.click(screen.getByRole('button', { name: 'Передать' }))

    await waitFor(() =>
      expect(screen.getByText('Проект или участник не найдены.')).toBeInTheDocument(),
    )
    expect(screen.getByLabelText('Новый владелец проекта')).toHaveValue('u-editor')
  })
})
