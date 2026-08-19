import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AssistantPanel } from './AssistantPanel'
import { projectsApi } from '../api/projects'
import { ApiError, type AssistantResult } from '@shared/types'
import { defaultConfig, defaultRates } from '@shared/config'

const makeResult = (overrides: Partial<AssistantResult> = {}): AssistantResult => ({
  kind: 'design',
  response: {
    recommendation: 'Рекомендуется L-образный марш.',
    rating: 0.83,
    alternatives: [{ title: 'Прямой марш', rating: 0.62, reason: 'дешевле' }],
    findings: [{ severity: 'warning', element: 'clearance_mm', message: 'Малый зазор' }],
    suggestions: [{ message: 'Уменьшить шаг', rationale: 'лучше комфорт' }],
    tradeoffs: ['Цена против комфорта'],
    notes: ['Спираль без радиуса приведена к R=2W'],
  },
  commentary: 'Локальная модель: рекомендация сформирована.',
  ...overrides,
})

function renderPanel() {
  return render(<AssistantPanel config={defaultConfig} rates={defaultRates} />)
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('AssistantPanel', () => {
  it('показывает четыре вкладки и спрашивает design по умолчанию', () => {
    renderPanel()
    expect(screen.getByRole('tab', { name: /Проектирование/ })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /Инжиниринг/ })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /Производство/ })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /Ценообразование/ })).toBeInTheDocument()
    expect(screen.getByLabelText(/Приоритет/)).toBeInTheDocument()
  })

  it('отправляет design-запрос с приоритетом и показывает результат', async () => {
    const ask = vi.spyOn(projectsApi, 'assistant').mockResolvedValue(makeResult())
    renderPanel()

    fireEvent.click(screen.getByRole('button', { name: /Спросить ассистента/ }))
    expect(await screen.findByText(/Рекомендуется L-образный марш/)).toBeInTheDocument()
    expect(screen.getByText(/Рейтинг/)).toBeInTheDocument()
    expect(screen.getByText(/Малый зазор/)).toBeInTheDocument()
    expect(screen.getByText(/Уменьшить шаг/)).toBeInTheDocument()
    expect(screen.getByText(/Альтернативные варианты/)).toBeInTheDocument()
    expect(screen.getByText(/Локальная модель/)).toBeInTheDocument()

    const [kind, body] = ask.mock.calls[0]
    expect(kind).toBe('design')
    expect((body as Record<string, unknown>).priority).toBe('price')
    expect((body as Record<string, unknown>).width_mm).toBe(900)
  })

  it('шлёт анализ без приоритета для прочих kind', async () => {
    const ask = vi.spyOn(projectsApi, 'assistant').mockResolvedValue(
      makeResult({
        kind: 'engineering',
        response: {
          recommendation: 'Конструкция соответствует СТАНДАРТ.',
          rating: 0.9,
        },
      }),
    )
    renderPanel()
    fireEvent.click(screen.getByRole('tab', { name: /Инжиниринг/ }))
    expect(screen.queryByLabelText(/Приоритет/)).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Спросить ассистента/ }))
    expect(await screen.findByText(/Конструкция соответствует/)).toBeInTheDocument()

    const [kind, body] = ask.mock.calls[0]
    expect(kind).toBe('engineering')
    expect((body as Record<string, unknown>).priority).toBeUndefined()
  })

  it('меняет приоритет design на comfort', async () => {
    const ask = vi.spyOn(projectsApi, 'assistant').mockResolvedValue(makeResult())
    renderPanel()
    fireEvent.change(screen.getByLabelText(/Приоритет/), { target: { value: 'comfort' } })
    fireEvent.click(screen.getByRole('button', { name: /Спросить ассистента/ }))
    await screen.findByText(/Рекомендуется L-образный марш/)
    expect((ask.mock.calls[0][1] as Record<string, unknown>).priority).toBe('comfort')
  })

  it('включает rates при заполненных ставках', async () => {
    const ask = vi.spyOn(projectsApi, 'assistant').mockResolvedValue(makeResult())
    render(
      <AssistantPanel
        config={defaultConfig}
        rates={{ ...defaultRates, steel: '200' }}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: /Спросить ассистента/ }))
    await screen.findByText(/Рекомендуется L-образный марш/)
    const body = ask.mock.calls[0][1] as {
      rates?: { material_per_kg_rub?: Record<string, number> }
    }
    expect(body.rates?.material_per_kg_rub?.['STEEL-S235']).toBe(200)
  })

  it('показывает пустое состояние без альтернатив', async () => {
    vi.spyOn(projectsApi, 'assistant').mockResolvedValue(
      makeResult({
        response: { recommendation: 'Ок.', rating: 1 },
      }),
    )
    renderPanel()
    fireEvent.click(screen.getByRole('button', { name: /Спросить ассистента/ }))
    await screen.findByText(/Ок\./)
    expect(screen.queryByText(/Альтернативные варианты/)).not.toBeInTheDocument()
  })

  it('показывает ошибку API', async () => {
    vi.spyOn(projectsApi, 'assistant').mockRejectedValue(
      new ApiError(422, 'invalid_input', 'Некорректная конфигурация'),
    )
    renderPanel()
    fireEvent.click(screen.getByRole('button', { name: /Спросить ассистента/ }))
    expect(await screen.findByText('Некорректная конфигурация')).toBeInTheDocument()
  })
})