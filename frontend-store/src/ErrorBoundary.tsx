import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
}

// ErrorBoundary — защита от «белого экрана»: любой необработанный сбой
// внутри subtree рендерит локальный фолбэк вместо краха всего приложения
// (P0-6). Обязателен вокруг корня, 3D-вьювера (WebGL, ленивый чанк) и
// кабинета.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: unknown, info: ErrorInfo) {
    console.error('ErrorBoundary:', error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      return (
        this.props.fallback ?? (
          <div className="alert alert--error" role="alert">
            <p>Что-то пошло не так. Перезагрузите страницу, чтобы продолжить расчёт.</p>
            <button type="button" className="sp-btn" onClick={() => window.location.reload()}>
              Перезагрузить
            </button>
          </div>
        )
      )
    }
    return this.props.children
  }
}