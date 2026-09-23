// ErrorBoundary (S-140) — общий boundary для admin и store. Ловит любую
// ошибку рендера в subtree, логирует в Sentry (ленивый SDK) и показывает
// русский fallback с кнопкой «Попробовать снова» и eventId при наличии.
//
// Заменяет локальные административные обёртки: админка раньше вовсе не имела
// boundary, store имел собственный (src/ErrorBoundary.tsx) — теперь единый
// shared-компонент, поведение fallback-пропа сохранено для совместимости
// (QuoteResult передаёт кастомный fallback для 3D-вьювера).
import { Component, type ErrorInfo, type ReactNode } from 'react'
import { captureError } from './sentry'

interface Props {
  children: ReactNode
  /** Кастомный fallback (совместимость со старым store-ErrorBoundary). */
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  eventId?: string
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  async componentDidCatch(error: unknown, info: ErrorInfo) {
    // Sentry-захват ошибки (PII-чистый контекст: componentStack не содержит
    // пользовательских данных). eventId появится, только если SDK уже готов.
    const eventId = await captureError(error, { componentStack: info.componentStack })
    if (eventId) {
      this.setState({ eventId })
    }
  }

  private handleRetry = (): void => {
    this.setState({ hasError: false, eventId: undefined })
  }

  render() {
    if (!this.state.hasError) {
      return this.props.children
    }
    if (this.props.fallback) {
      return this.props.fallback
    }
    return (
      <div className="sentry-boundary" role="alert">
        <h2 className="sentry-boundary__title">Что-то пошло не так</h2>
        <p className="sentry-boundary__text">Произошла непредвиденная ошибка. Попробуйте снова.</p>
        {this.state.eventId ? (
          <p className="sentry-boundary__event-id">Код ошибки: {this.state.eventId}</p>
        ) : null}
        <button type="button" className="sentry-boundary__retry" onClick={this.handleRetry}>
          Попробовать снова
        </button>
      </div>
    )
  }
}