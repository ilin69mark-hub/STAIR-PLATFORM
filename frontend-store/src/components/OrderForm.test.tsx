import { fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { OrderForm } from './OrderForm'
import { ordersApi } from '../api/store'
import { renderWithAuth, testUser } from '../test/render'
import type { QuoteResult, OrderDTO } from '@shared/types'

const quote: QuoteResult = {
  validation: { valid: true, blocking: false, issues: [] },
  flight: { step_count: 15, step_height_mm: 180, tread_depth_mm: 270, run_mm: 4050, stringer_mm: 4867.49, angle_deg: 33.69, width_mm: 900, step_thickness_mm: 40, railing_height_mm: 900, riser: true, stringer_thickness_mm: 50 },
  pricing: { currency: 'RUB', material_rub: 1, machine_rub: 2, labor_rub: 3, overhead_rub: 4, production_cost_rub: 10, margin_rub: 5, discount_rub: 0, pre_tax_rub: 15, tax_rub: 3, final_price_rub: 18, lines: [] },
}

const config = { width_mm: 900, height_mm: 2700, flight: 'straight' }

const created: OrderDTO = {
  id: 'order-1',
  kind: 'order',
  status: 'new',
  contact: { name: 'Иван', email: 'buyer@example.com', phone: '+7 900 000-00-00' },
  config: {},
  price: { final_price_rub: 18 },
  created_at: '2026-08-17T10:00:00Z',
  updated_at: '2026-08-17T10:00:00Z',
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('OrderForm', () => {
  it('требует вход: для анонимуса показывает форму аутентификации', async () => {
    await renderWithAuth(<OrderForm quote={quote} config={config} />, null)
    expect(await screen.findByRole('heading', { name: 'Вход' })).toBeInTheDocument()
    expect(screen.queryByText('Оформление заказа')).not.toBeInTheDocument()
  })

  it('авторизованный пользователь отправляет заказ с ценой', async () => {
    const spy = vi.spyOn(ordersApi, 'create').mockResolvedValue(created)
    const onCreated = vi.fn()
    await renderWithAuth(<OrderForm quote={quote} config={config} onCreated={onCreated} />)
    expect(await screen.findByText('Оформление заказа')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Имя'), { target: { value: 'Иван' } })
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: testUser.email } })
    fireEvent.change(screen.getByLabelText('Телефон'), { target: { value: '+7 900 000-00-00' } })
    fireEvent.click(screen.getByRole('button', { name: 'Отправить заказ' }))

    await waitFor(() =>
      expect(spy).toHaveBeenCalledWith({
        contact: { name: 'Иван', email: testUser.email, phone: '+7 900 000-00-00' },
        config,
        price: expect.objectContaining({ final_price_rub: 18 }),
      }),
    )
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(created))
  })

  it('валидирует обязательные имя и email', async () => {
    const spy = vi.spyOn(ordersApi, 'create').mockResolvedValue(created)
    await renderWithAuth(<OrderForm quote={quote} config={config} />)
    await screen.findByText('Оформление заказа')
    fireEvent.click(screen.getByRole('button', { name: 'Отправить заказ' }))
    expect(await screen.findByText('Укажите имя и email')).toBeInTheDocument()
    expect(spy).not.toHaveBeenCalled()
  })
})