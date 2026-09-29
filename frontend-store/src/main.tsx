import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { AuthProvider } from './auth/AuthContext.tsx'
import { ErrorBoundary } from '@shared/sentry/ErrorBoundary'
import { initSentry } from '@shared/sentry/sentry'
import { start as startFunnel } from '@shared/analytics'

// Ленивая Sentry-инициализация (S-140): SDK грузится по requestIdleCallback/
// 3s или при первой ошибке; пустой VITE_SENTRY_DSN = режим без Sentry.
// На витрине Sentry дополнительно ждёт согласия (shared/sentry: гейт).
initSentry()

// Воронка витрины: session.start + отправка на уход со страницы. Без
// согласия start() не отправит ничего — гейт внутри analytics.ts.
startFunnel()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <AuthProvider>
        <App />
      </AuthProvider>
    </ErrorBoundary>
  </StrictMode>,
)