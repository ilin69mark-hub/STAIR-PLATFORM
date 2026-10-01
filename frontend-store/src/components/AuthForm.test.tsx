import { describe, expect, it, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { AuthForm } from '@shared/storefront/components/AuthForm'

const login = vi.fn(() => Promise.resolve({ id: 'u1' } as any))
const register = vi.fn(() => Promise.resolve({ id: 'u1' } as any))
// Мок на реальный модуль общего пакета: AuthForm переехал туда и берёт
// useAuth именно оттуда (фасад магазина не участвует в графе импортов).
vi.mock('@shared/storefront/auth/context', () => ({
  useAuth: () => ({ login, register }),
}))

describe('AuthForm', () => {
  afterEach(() => { vi.clearAllMocks() })
  it('validates empty', async () => {
    render(<AuthForm />)
    fireEvent.click(screen.getByText('Войти'))
    expect(screen.getByText('Заполните все поля')).toBeInTheDocument()
  })
  it('switches mode', () => {
    render(<AuthForm />)
    fireEvent.click(screen.getByText('Регистрация'))
    expect(screen.getByText('Зарегистрироваться')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Вход'))
    expect(screen.getByText('Войти')).toBeInTheDocument()
  })
  it('login success shows done', async () => {
    render(<AuthForm />)
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'a@ex.ru' } })
    fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('Войти'))
    await waitFor(() => expect(screen.getByText(/Теперь можно/)).toBeInTheDocument())
  })
  it('error shows', async () => {
    login.mockRejectedValueOnce(new Error('bad'))
    render(<AuthForm />)
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'a@ex.ru' } })
    fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('Войти'))
    await waitFor(() => expect(screen.getByText(/Не удалось/)).toBeInTheDocument())
  })
})
