import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { AuthProvider } from './auth/AuthContext.tsx'
import { ErrorBoundary } from '@shared/sentry/ErrorBoundary'
import { initSentry } from '@shared/sentry/sentry'

// Ленивая Sentry-инициализация (S-140): SDK грузится по requestIdleCallback/
// 3s или при первой ошибке; пустой VITE_SENTRY_DSN = режим без Sentry.
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
