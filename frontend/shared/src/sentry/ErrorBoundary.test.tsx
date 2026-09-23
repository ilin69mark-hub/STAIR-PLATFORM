// Тесты shared ErrorBoundary (S-140): ловит ошибку рендера -> русский
// fallback с кнопкой «Попробовать снова» + eventId при наличии.
// captureError из sentry-модуля замокан — тесты не зависят от SDK/DSN.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { ErrorBoundary } from './ErrorBoundary'

const captureErrorMock = vi.fn()
vi.mock('./sentry', () => ({
  captureError: (...args: unknown[]) => captureErrorMock(...args),
}))

// «Взрывающийся» компонент для проверки catch.
function Bomb(): never {
  throw new Error('render boom')
}

// Подавить console.error — React печатает пойманную ошибку в консоль.
function silenceConsoleError(): void {
  vi.spyOn(console, 'error').mockImplementation(() => {})
}

describe('ErrorBoundary (S-140)', () => {
  beforeEach(() => {
    captureErrorMock.mockClear()
    captureErrorMock.mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('без ошибок рендерит children как есть', () => {
    render(
      <ErrorBoundary>
        <p>ok</p>
      </ErrorBoundary>,
    )
    expect(screen.getByText('ok')).toBeInTheDocument()
  })

  it('ловит ошибку -> русский fallback с кнопкой «Попробовать снова»', () => {
    silenceConsoleError()
    render(
      <ErrorBoundary>
        <Bomb />
      </ErrorBoundary>,
    )
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText('Что-то пошло не так')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Попробовать снова' })).toBeInTheDocument()
    // Ошибка ушла в Sentry (с componentStack в контексте).
    expect(captureErrorMock).toHaveBeenCalledTimes(1)
    expect(captureErrorMock.mock.calls[0]?.[1]).toHaveProperty('componentStack')
  })

  it('fallback проп сохранён (QuoteResult передаёт кастомный)', () => {
    silenceConsoleError()
    render(
      <ErrorBoundary fallback={<p>fallback-3d</p>}>
        <Bomb />
      </ErrorBoundary>,
    )
    expect(screen.getByText('fallback-3d')).toBeInTheDocument()
  })

  it('кнопка «Попробовать снова» сбрасывает состояние и рендерит children', () => {
    silenceConsoleError()
    let crashed = true
    function Flaky(): 'flaky-ok' {
      if (crashed) throw new Error('boom')
      return 'flaky-ok'
    }
    render(
      <ErrorBoundary>
        <Flaky />
      </ErrorBoundary>,
    )
    expect(screen.getByRole('button', { name: 'Попробовать снова' })).toBeInTheDocument()

    crashed = false
    fireEvent.click(screen.getByRole('button', { name: 'Попробовать снова' }))
    expect(screen.getByText('flaky-ok')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('показывает eventId, когда captureError вернул его', async () => {
    silenceConsoleError()
    captureErrorMock.mockResolvedValue('evt-abc123')
    render(
      <ErrorBoundary>
        <Bomb />
      </ErrorBoundary>,
    )
    // eventId приходит после async componentDidCatch.
    expect(await screen.findByText('Код ошибки: evt-abc123')).toBeInTheDocument()
  })
})