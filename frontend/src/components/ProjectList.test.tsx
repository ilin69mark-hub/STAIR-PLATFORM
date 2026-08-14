import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ProjectList } from './ProjectList'
import { projectsApi } from '../api/projects'
import { ApiError, type Project } from '../api/types'

const projects: Project[] = [
  {
    id: 'p1',
    name: 'Лестница 1',
    description: '',
    status: 'active',
    created_at: '2026-08-14T10:00:00Z',
    updated_at: '2026-08-14T10:00:00Z',
  },
]

afterEach(() => {
  vi.restoreAllMocks()
})

describe('ProjectList', () => {
  it('показывает пустое состояние', () => {
    render(
      <ProjectList projects={[]} loading={false} onSelect={vi.fn()} onCreated={vi.fn()} />,
    )
    expect(screen.getByText('Проектов пока нет. Создайте первый.')).toBeInTheDocument()
  })

  it('показывает загрузку', () => {
    render(<ProjectList projects={[]} loading onSelect={vi.fn()} onCreated={vi.fn()} />)
    expect(screen.getByText('Загрузка…')).toBeInTheDocument()
  })

  it('рендерит проекты и вызывает onSelect', () => {
    const onSelect = vi.fn()
    render(
      <ProjectList projects={projects} loading={false} onSelect={onSelect} onCreated={vi.fn()} />,
    )
    fireEvent.click(screen.getByText('Лестница 1'))
    expect(onSelect).toHaveBeenCalledWith('p1')
  })

  it('создаёт проект и очищает поля', async () => {
    const created = { ...projects[0] }
    const create = vi.spyOn(projectsApi, 'create').mockResolvedValue(created)
    const onCreated = vi.fn()
    render(
      <ProjectList projects={projects} loading={false} onSelect={vi.fn()} onCreated={onCreated} />,
    )

    fireEvent.change(screen.getByLabelText(/Название/), { target: { value: '  Новый  ' } })
    fireEvent.change(screen.getByLabelText(/Описание/), { target: { value: 'тест' } })
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }))

    expect(create).toHaveBeenCalledWith({ name: 'Новый', description: 'тест' })
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(created))
    expect((screen.getByLabelText(/Название/) as HTMLInputElement).value).toBe('')
    expect((screen.getByLabelText(/Описание/) as HTMLTextAreaElement).value).toBe('')
  })

  it('кнопка создания заблокирована без названия', () => {
    render(
      <ProjectList projects={projects} loading={false} onSelect={vi.fn()} onCreated={vi.fn()} />,
    )
    expect(screen.getByRole('button', { name: 'Создать' })).toBeDisabled()
  })

  it('показывает сообщение ApiError при неудачном создании', async () => {
    vi.spyOn(projectsApi, 'create').mockRejectedValue(
      new ApiError(500, 'db', 'База недоступна'),
    )
    render(<ProjectList projects={[]} loading={false} onSelect={vi.fn()} onCreated={vi.fn()} />)
    fireEvent.change(screen.getByLabelText(/Название/), { target: { value: 'X' } })
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }))
    expect(await screen.findByText('База недоступна')).toBeInTheDocument()
  })

  it('показывает общее сообщение при неизвестной ошибке', async () => {
    vi.spyOn(projectsApi, 'create').mockRejectedValue(new Error('boom'))
    render(<ProjectList projects={[]} loading={false} onSelect={vi.fn()} onCreated={vi.fn()} />)
    fireEvent.change(screen.getByLabelText(/Название/), { target: { value: 'X' } })
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }))
    expect(await screen.findByText('Не удалось создать проект')).toBeInTheDocument()
  })
})