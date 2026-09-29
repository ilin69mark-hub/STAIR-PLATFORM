import { fireEvent, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { CookieBanner } from './CookieBanner'
import { InfoPage } from './InfoPage'
import { render } from '@testing-library/react'
import { CONSENT_KEY, CONSENT_VERSION, grantConsent } from '@shared/consent'

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
    expect(screen.getByLabelText('Согласие на обработку данных')).toBeInTheDocument()
  })

  it('скрывается после согласия и пишет версию политики', () => {
    localStorage.clear()
    render(<CookieBanner onOpenPolicy={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Разрешить' }))
    expect(screen.queryByLabelText('Согласие на обработку данных')).not.toBeInTheDocument()
    // Именно объект с версией, а не строка 'accepted': при новом получателе
    // данных версия поднимается, и все, кто согласился раньше, должны
    // увидеть вопрос заново.
    const raw = localStorage.getItem(CONSENT_KEY)
    expect(raw).toBeTruthy()
    expect(JSON.parse(raw as string)).toMatchObject({ v: CONSENT_VERSION })
  })

  it('не показывается, если согласие уже дано', () => {
    grantConsent()
    render(<CookieBanner onOpenPolicy={vi.fn()} />)
    expect(screen.queryByLabelText('Согласие на обработку данных')).not.toBeInTheDocument()
  })

  // Старый баннер писал просто 'accepted'. Такой формат считается согласием
  // на версию 1, иначе баннер начал бы появляться у всех, кто нажал кнопку
  // до появления версий.
  it('старое значение "accepted" считается согласием', () => {
    localStorage.setItem(CONSENT_KEY, 'accepted')
    render(<CookieBanner onOpenPolicy={vi.fn()} />)
    expect(screen.queryByLabelText('Согласие на обработку данных')).not.toBeInTheDocument()
  })

  // Согласие под СТАРОЙ версией политики — вопрос заново: появился новый
  // получатель данных, и у этого посетителя его не спрашивали.
  it('согласие под старой версией политики не подходит', () => {
    localStorage.setItem(CONSENT_KEY, JSON.stringify({ v: CONSENT_VERSION - 1, at: '' }))
    render(<CookieBanner onOpenPolicy={vi.fn()} />)
    expect(screen.getByLabelText('Согласие на обработку данных')).toBeInTheDocument()
  })

  it('открывает политику по ссылке', () => {
    localStorage.clear()
    const open = vi.fn()
    render(<CookieBanner onOpenPolicy={open} />)
    fireEvent.click(screen.getByText(/Подробнее о политике/))
    expect(open).toHaveBeenCalled()
  })
})

describe('политика cookie', () => {
  it('позволяет отозвать согласие', () => {
    localStorage.clear()
    grantConsent()
    render(<InfoPage kind="cookies" onBack={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Отозвать согласие' }))
    expect(localStorage.getItem(CONSENT_KEY)).toBeNull()
    expect(screen.getByText(/Согласия нет/)).toBeInTheDocument()
  })

  it('честно перечисляет, что собирается и что нет', () => {
    localStorage.clear()
    render(<InfoPage kind="cookies" onBack={vi.fn()} />)
    expect(screen.getByText(/на каком шаге вы ушли со страницы/)).toBeInTheDocument()
    expect(screen.getByText(/содержимое заполненных полей/)).toBeInTheDocument()
  })
})