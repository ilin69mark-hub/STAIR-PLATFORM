import { fireEvent, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { CookieBanner } from './CookieBanner'
import { InfoPage } from './InfoPage'
import { render } from '@testing-library/react'
import { COOKIE_CONSENT_KEY } from '../config'

describe('InfoPage', () => {
  it('показывает публичную оферту', () => {
    render(<InfoPage kind="offer" onBack={vi.fn()} />)
    expect(screen.getByText('Публичная оферта')).toBeInTheDocument()
    expect(screen.getByText(/Предмет договора/)).toBeInTheDocument()
  })

  it('показывает политику конфиденциальности', () => {
    render(<InfoPage kind="privacy" onBack={vi.fn()} />)
    expect(screen.getByText('Политика конфиденциальности')).toBeInTheDocument()
    expect(screen.getByText(/Какие данные собираются/)).toBeInTheDocument()
  })

  it('показывает политику cookie', () => {
    render(<InfoPage kind="cookies" onBack={vi.fn()} />)
    expect(screen.getByText('Политика использования cookie')).toBeInTheDocument()
  })

  it('возвращает назад', () => {
    const back = vi.fn()
    render(<InfoPage kind="offer" onBack={back} />)
    fireEvent.click(screen.getByText(/Назад/))
    expect(back).toHaveBeenCalled()
  })
})

describe('CookieBanner', () => {
  it('показывается, если согласие не дано', () => {
    localStorage.clear()
    render(<CookieBanner onOpenPolicy={vi.fn()} />)
    expect(screen.getByLabelText('Согласие на cookie')).toBeInTheDocument()
  })

  it('скрывается после принятия', () => {
    localStorage.clear()
    render(<CookieBanner onOpenPolicy={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Принять' }))
    expect(screen.queryByLabelText('Согласие на cookie')).not.toBeInTheDocument()
    expect(localStorage.getItem(COOKIE_CONSENT_KEY)).toBe('accepted')
  })

  it('не показывается, если согласие уже дано', () => {
    localStorage.setItem(COOKIE_CONSENT_KEY, 'accepted')
    render(<CookieBanner onOpenPolicy={vi.fn()} />)
    expect(screen.queryByLabelText('Согласие на cookie')).not.toBeInTheDocument()
  })

  it('открывает политику cookie по ссылке', () => {
    localStorage.clear()
    const open = vi.fn()
    render(<CookieBanner onOpenPolicy={open} />)
    fireEvent.click(screen.getByText(/Подробнее о политике cookie/))
    expect(open).toHaveBeenCalled()
  })
})