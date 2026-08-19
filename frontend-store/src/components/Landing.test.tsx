import { fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Landing } from './Landing'
import { consultationsApi, testimonialsApi } from '../api/store'
import { render } from '@testing-library/react'

afterEach(() => {
  vi.restoreAllMocks()
})

function renderLanding() {
  return render(<Landing onStart={vi.fn()} />)
}

describe('Landing', () => {
  it('не показывает контакты компании в hero', () => {
    vi.spyOn(testimonialsApi, 'published').mockResolvedValue([])
    renderLanding()
    expect(screen.queryByText('info@stair-platform.ru')).not.toBeInTheDocument()
    expect(screen.queryByText('+7 (___) ___-__-__')).not.toBeInTheDocument()
  })

  it('показывает опубликованные отзывы', async () => {
    vi.spyOn(testimonialsApi, 'published').mockResolvedValue([
      { id: 't-1', author: 'Иван', text: 'Отличная лестница', rating: 5, published: true, created_at: '2026-08-01T00:00:00Z' },
    ])
    renderLanding()
    expect(await screen.findByText('Отличная лестница')).toBeInTheDocument()
    expect(screen.getByText('Иван')).toBeInTheDocument()
    expect(screen.getByLabelText('Оценка 5 из 5')).toBeInTheDocument()
  })

  it('скрывает секцию отзывов, если опубликованных нет', async () => {
    vi.spyOn(testimonialsApi, 'published').mockResolvedValue([])
    renderLanding()
    await waitFor(() => expect(testimonialsApi.published).toHaveBeenCalled())
    expect(screen.queryByText('Отзывы клиентов')).not.toBeInTheDocument()
  })

  it('отправляет консультацию и показывает подтверждение', async () => {
    vi.spyOn(testimonialsApi, 'published').mockResolvedValue([])
    const create = vi.spyOn(consultationsApi, 'create').mockResolvedValue({
      id: 'ord-c', kind: 'consultation', status: 'new',
      contact: { name: 'Мария', email: 'm@ex.ru' }, config: {}, created_at: '', updated_at: '',
    })
    renderLanding()

    fireEvent.change(screen.getByLabelText('Имя'), { target: { value: 'Мария' } })
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'm@ex.ru' } })
    fireEvent.change(screen.getByLabelText('Ваш вопрос'), { target: { value: 'Сколько стоит лестница?' } })
    fireEvent.click(screen.getByRole('button', { name: 'Отправить заявку' }))

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({
        contact: { name: 'Мария', email: 'm@ex.ru', phone: '' },
        question: 'Сколько стоит лестница?',
      }),
    )
    expect(await screen.findByText(/Заявка отправлена/)).toBeInTheDocument()
  })

  it('показывает ошибку при неудачной отправке консультации', async () => {
    vi.spyOn(testimonialsApi, 'published').mockResolvedValue([])
    vi.spyOn(consultationsApi, 'create').mockRejectedValue(new Error('boom'))
    renderLanding()

    fireEvent.change(screen.getByLabelText('Имя'), { target: { value: 'Мария' } })
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'm@ex.ru' } })
    fireEvent.change(screen.getByLabelText('Ваш вопрос'), { target: { value: 'Вопрос' } })
    fireEvent.click(screen.getByRole('button', { name: 'Отправить заявку' }))

    expect(await screen.findByText(/Не удалось отправить заявку/)).toBeInTheDocument()
  })

  it('показывает блок «О нас» под консультацией', () => {
    vi.spyOn(testimonialsApi, 'published').mockResolvedValue([])
    renderLanding()
    expect(screen.getByText('О нас')).toBeInTheDocument()
    expect(screen.getByText('Собственное производство')).toBeInTheDocument()
    expect(screen.getByText('Инженерный расчёт')).toBeInTheDocument()
    expect(screen.getByText('Сроки и гарантия')).toBeInTheDocument()
  })
})