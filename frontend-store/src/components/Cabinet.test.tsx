import { screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Cabinet } from './Cabinet'
import { ordersApi } from '../api/store'
import { renderWithAuth, testUser } from '../test/render'
import type { OrderDTO } from '@shared/types'

const orders: OrderDTO[] = [
  {
    id: 'order-1',
    kind: 'order',
    status: 'new',
    contact: { name: 'Иван', email: testUser.email },
    config: { width_mm: 900, height_mm: 2700 },
    price: { final_price_rub: 180000 },
    created_at: '2026-08-17T10:00:00Z',
    updated_at: '2026-08-17T10:00:00Z',
  },
  {
    id: 'order-2',
    kind: 'order',
    status: 'confirmed',
    contact: { name: 'Иван', email: testUser.email },
    config: { width_mm: 1200, height_mm: 3000 },
    price: { final_price_rub: 250000 },
    created_at: '2026-08-16T09:00:00Z',
    updated_at: '2026-08-16T09:00:00Z',
  },
]

afterEach(() => {
  vi.restoreAllMocks()
})

describe('Cabinet', () => {
  it('анонимум видит предложение войти', async () => {
    await renderWithAuth(<Cabinet />, null)
    expect(await screen.findByRole('heading', { name: 'Вход' })).toBeInTheDocument()
    expect(screen.queryByText('Мои заказы')).not.toBeInTheDocument()
  })

  it('показывает заказы пользователя со статусами и ценами', async () => {
    vi.spyOn(ordersApi, 'listMine').mockResolvedValue(orders)
    await renderWithAuth(<Cabinet />)
    expect(await screen.findByText('Мои заказы')).toBeInTheDocument()
    expect(await screen.findByText('order-1'.slice(0, 8))).toBeInTheDocument()
    expect(screen.getByText('Новый')).toBeInTheDocument()
    expect(screen.getByText('Подтверждён')).toBeInTheDocument()
    expect(screen.getByText('900 × 2700 мм')).toBeInTheDocument()
    expect(screen.getByText('180 000,00 ₽')).toBeInTheDocument()
    await waitFor(() => expect(ordersApi.listMine).toHaveBeenCalledTimes(1))
  })

  it('показывает пустое состояние', async () => {
    vi.spyOn(ordersApi, 'listMine').mockResolvedValue([])
    await renderWithAuth(<Cabinet />)
    expect(await screen.findByText(/Заказов пока нет/)).toBeInTheDocument()
  })

  it('показывает ошибку загрузки', async () => {
    vi.spyOn(ordersApi, 'listMine').mockRejectedValue(new Error('boom'))
    await renderWithAuth(<Cabinet />)
    expect(await screen.findByText('Не удалось загрузить заказы')).toBeInTheDocument()
  })
})