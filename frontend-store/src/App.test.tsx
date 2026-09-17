import { fireEvent, screen, within, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { authApi } from './api/auth'
import { renderWithAuth } from './test/render'

afterEach(() => {
  vi.restoreAllMocks()
  window.history.replaceState(null, '', '/')
})

const setHash = (hash: string) => window.history.replaceState(null, '', hash)

describe('App', () => {
  it('показывает телефон и почту в шапке', async () => {
    await renderWithAuth(<App />)
    waitFor(() => expect(authApi.me).toHaveBeenCalled())
    const header = screen.getByRole('banner')
    expect(within(header).getByText('info@stair-platform.ru')).toBeInTheDocument()
    expect(within(header).getByText('+7 (___) ___-__-__')).toBeInTheDocument()
  })

  it('deep-link #constructor открывает конструктор с хэшем', async () => {
    setHash('#constructor')
    await renderWithAuth(<App />)
    expect(screen.getByRole('heading', { name: 'Конструктор лестницы' })).toBeInTheDocument()
    expect(window.location.hash).toBe('#constructor')
  })

  it('deep-link #cabinet открывает вход в кабинет без авторизации', async () => {
    setHash('#cabinet')
    await renderWithAuth(<App />, null)
    expect(screen.getByRole('heading', { name: 'Личный кабинет' })).toBeInTheDocument()
  })

  it('deep-link #privacy открывает правовую страницу', async () => {
    setHash('#privacy')
    await renderWithAuth(<App />)
    expect(
      screen.getByRole('heading', { name: 'Политика конфиденциальности' }),
    ).toBeInTheDocument()
  })

  it('клик по футер-ссылке открывает правовую страницу и синхронизирует хэш', async () => {
    await renderWithAuth(<App />)
    fireEvent.click(screen.getByText('Публичная оферта'))
    expect(screen.getByText('1. Предмет договора')).toBeInTheDocument()
    expect(window.location.hash).toBe('#offer')
  })

  it('клик по нав-ссылке конструктора открывает конструктор', async () => {
    await renderWithAuth(<App />)
    fireEvent.click(screen.getByRole('link', { name: 'Конструктор' }))
    expect(screen.getByRole('heading', { name: 'Конструктор лестницы' })).toBeInTheDocument()
  })
})