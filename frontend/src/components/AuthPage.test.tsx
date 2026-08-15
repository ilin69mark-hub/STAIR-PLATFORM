import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthPage } from './AuthPage'
import { authApi, type SsoConfig } from '../api/auth'
import { AuthProvider } from '../auth/AuthContext'

const config = (enabled: boolean, provider = 'example-idp'): SsoConfig => ({
  enabled,
  provider,
})

function renderAuth() {
  return render(
    <AuthProvider>
      <AuthPage />
    </AuthProvider>,
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('AuthPage / SSO (EDR-0017)', () => {
  it('не показывает кнопку SSO, когда провайдер выключен', async () => {
    vi.spyOn(authApi, 'ssoConfig').mockResolvedValue(config(false))
    renderAuth()
    await waitFor(() => expect(authApi.ssoConfig).toHaveBeenCalled())
    await waitFor(() =>
      expect(screen.queryByText(/Войти через/)).not.toBeInTheDocument(),
    )
  })

  it('показывает кнопку SSO с именем провайдера, когда включён', async () => {
    vi.spyOn(authApi, 'ssoConfig').mockResolvedValue(config(true, 'corp-idp'))
    renderAuth()
    const btn = await screen.findByText(/Войти через corp-idp/)
    expect(btn).toBeInTheDocument()
    expect(btn.closest('a')).toHaveAttribute(
      'href',
      '/api/v1/auth/sso?redirect=%2F',
    )
  })

  it('показывает ошибку из query-параметра колбэка', async () => {
    vi.spyOn(authApi, 'ssoConfig').mockResolvedValue(config(false))
    const url = new URL(window.location.href)
    url.search = '?error=sso_error&message=Login%20denied'
    window.history.replaceState({}, '', url.toString())
    renderAuth()
    await waitFor(() => expect(screen.getByText('Login denied')).toBeInTheDocument())
    // Параметр очищен после отображения.
    await waitFor(() =>
      expect(window.location.search).toBe(''),
    )
  })
})
