import type { ReactNode } from 'react'
import { AuthProvider } from './auth/AuthProvider'

// Общая обёртка витрины: конструктор и блок услуг читают одну и ту же сессию
// (cookie домена), поэтому вход в одном месте виден в обоих.
export function StorefrontProviders({ children }: { children: ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>
}
