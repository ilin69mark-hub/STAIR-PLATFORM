// Общий каркас кабинета.
//
// Зачем он: раньше шапка с email и кнопкой «Выйти» рисовалась только на
// экране проектов, а у «Администрирования», «Аудита» и «Магазина» была своя
// шапка с кнопкой «Назад» — выйти из них было нельзя. Теперь одна шапка на
// весь кабинет: заголовок, имя пользователя, выход и, когда мы не на плитках,
// возврат к списку плиток.
import type { ReactNode } from 'react'

export interface CabinetShellProps {
  title: string
  subtitle?: string
  email: string
  /** Возврат к плиткам; null — мы на главном экране. */
  onBack?: (() => void) | null
  onLogout: () => void
  children: ReactNode
}

export function CabinetShell({ title, subtitle, email, onBack, onLogout, children }: CabinetShellProps) {
  return (
    <div className="page">
      <header className="page__header">
        <div>
          {onBack ? (
            <button type="button" className="btn btn--ghost" onClick={onBack}>
              ← Плитки
            </button>
          ) : null}
          <h1 className="page__title">{title}</h1>
          {subtitle && <p className="page__subtitle">{subtitle}</p>}
        </div>
        <div className="page__actions">
          <span className="muted">{email}</span>
          <button type="button" className="btn btn--ghost" onClick={onLogout}>
            Выйти
          </button>
        </div>
      </header>
      {children}
    </div>
  )
}