import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ProjectDetail } from './ProjectDetail'
import { projectsApi } from '../api/projects'
import { ApiError } from '../api/types'
import { makeCalculation, makeOptimize, makeProject } from '../test/fixtures'

afterEach(() => {
  vi.restoreAllMocks()
})

function renderDetail() {
  const onChanged = vi.fn()
  const onBack = vi.fn()
  const view = render(
    <ProjectDetail projectId="p1" onBack={onBack} onChanged={onChanged} />,
  )
  return { onChanged, onBack, view }
}

describe('ProjectDetail', () => {
  it('загружает и показывает проект', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    renderDetail()
    expect(await screen.findByRole('heading', { name: 'Лестница на второй этаж' })).toBeInTheDocument()
    expect(screen.getByText(/создан/)).toBeInTheDocument()
  })

  it('показывает ошибку при неудачной загрузке', async () => {
    vi.spyOn(projectsApi, 'get').mockRejectedValue(new ApiError(404, 'not_found', 'Нет проекта'))
    renderDetail()
    expect(await screen.findByText('Нет проекта')).toBeInTheDocument()
  })

  it('блокирует расчёт при жёстких ошибках валидации', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    renderDetail()
    const width = await screen.findByLabelText(/Ширина марша/)
    fireEvent.change(width, { target: { value: '' } })

    expect(screen.getByText('Укажите значение')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Рассчитать' })).toBeDisabled()
    expect(screen.getByText(/Исправьте нечисловые или пустые поля/)).toBeInTheDocument()
  })

  it('выполняет расчёт, показывает индикатор сохранения и зовёт onChanged', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    const calculate = vi.spyOn(projectsApi, 'calculate').mockResolvedValue(makeCalculation())
    const { onChanged } = renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    expect(screen.getByRole('button', { name: 'Экспорт JSON' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))

    expect(await screen.findByText(/Расчёт сохранён/)).toBeInTheDocument()
    expect(onChanged).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: 'Экспорт JSON' })).toBeEnabled()

    const [id, body] = calculate.mock.calls[0]
    expect(id).toBe('p1')
    expect((body as Record<string, unknown>).width_mm).toBe(900)
    expect((body as Record<string, unknown>).rates).toBeUndefined()
  })

  it('включает rates в тело расчёта при заполненных ставках', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    const calculate = vi.spyOn(projectsApi, 'calculate').mockResolvedValue(makeCalculation())
    renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.change(screen.getByLabelText(/Сталь/), { target: { value: '200' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт сохранён/)

    const body = calculate.mock.calls[0][1] as { rates?: { material_per_kg_rub?: Record<string, number> } }
    expect(body.rates?.material_per_kg_rub?.['STEEL-S235']).toBe(200)
  })

  it('показывает ошибку при неудачном расчёте', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    vi.spyOn(projectsApi, 'calculate').mockRejectedValue(
      new ApiError(500, 'pipeline', 'Не удалось рассчитать'),
    )
    renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText('Не удалось рассчитать')).toBeInTheDocument()
  })

  it('оптимизирует, применяет лучшую конфигурацию и зовёт onChanged', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    const optimize = vi.spyOn(projectsApi, 'optimize').mockResolvedValue(makeOptimize())
    const { onChanged } = renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.click(screen.getByRole('button', { name: 'Оптимизировать' }))

    expect(await screen.findByText(/Оптимум найден/)).toBeInTheDocument()
    expect(onChanged).toHaveBeenCalled()
    expect(screen.getByLabelText(/Целевая высота ступени/)).toHaveValue(168.75)

    const body = optimize.mock.calls[0][1] as Record<string, unknown>
    expect(body.target).toBe('price')
    expect(body.width_mm).toBe(900)
  })

  it('показывает сообщение, когда оптимизация не находит конфигурацию', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    vi.spyOn(projectsApi, 'optimize').mockResolvedValue(
      makeOptimize({ valid: false, best: undefined, saved: false }),
    )
    renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.click(screen.getByRole('button', { name: 'Оптимизировать' }))
    expect(await screen.findByText(/не нашла допустимую конфигурацию/)).toBeInTheDocument()
  })
})