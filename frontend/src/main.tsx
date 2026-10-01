import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { AuthProvider } from './auth/AuthContext.tsx'
import { ErrorBoundary } from '@shared/sentry/ErrorBoundary'
import { initSentry, setSentryRequiresConsent } from '@shared/sentry/sentry'

// Ленивая Sentry-инициализация (S-140): SDK грузится по requestIdleCallback/
// 3s или при первой ошибке; пустой VITE_SENTRY_DSN = режим без Sentry.
//
// Гейт согласия НЕ применяется: админкой пользуются свои сотрудники, а не
// анонимные посетители, и требовать у них согласие на сбор ошибок было бы
// просто неудобством. Гейт стоит в витрине (frontend-store), где решения
// принимает случайный посетитель.
setSentryRequiresConsent(false)
initSentry()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <AuthProvider>
        <App />
      </AuthProvider>
    </ErrorBoundary>
  </StrictMode>,
)
