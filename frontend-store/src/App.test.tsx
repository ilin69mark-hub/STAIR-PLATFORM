import { screen, within, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { authApi } from './api/auth'
import { renderWithAuth } from './test/render'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('App', () => {
  it('показывает телефон и почту в шапке', async () => {
    await renderWithAuth(<App />)
    waitFor(() => expect(authApi.me).toHaveBeenCalled())
    const header = screen.getByRole('banner')
    expect(within(header).getByText('info@stair-platform.ru')).toBeInTheDocument()
    expect(within(header).getByText('+7 (___) ___-__-__')).toBeInTheDocument()
  })
})