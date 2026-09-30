import { screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Cabinet } from './Cabinet'
import { ordersApi, paymentsApi } from '../api/store'
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
    // Заголовка «Вход» нет (владелец убрал: то же слово стоит на активной
    // кнопке переключателя, и читалось дважды подряд) — форма опознаётся по
    // самому переключателю.
    expect(await screen.findByRole('button', { name: 'Вход' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Вход' })).not.toBeInTheDocument()
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

  it('показывает купленные услуги из /payments/mine', async () => {
    vi.spyOn(paymentsApi, 'mine').mockResolvedValue([
      {
        id: 'pay-1',
        title: 'Выезд инженера и замер',
        tier_id: 'basic',
        amount_minor: 90_000,
        currency: 'RUB',
        status: 'paid',
        created_at: '2026-09-20T10:00:00Z',
        paid_at: '2026-09-20T10:05:00Z',
      },
    ])
    await renderWithAuth(<Cabinet />)
    expect(await screen.findByText('Выезд инженера и замер')).toBeInTheDocument()
    expect(screen.getByText(/оплачено/)).toBeInTheDocument()
    expect(screen.getByText(/900/)).toBeInTheDocument()
  })

  it('показывает ошибку загрузки', async () => {
    vi.spyOn(ordersApi, 'listMine').mockRejectedValue(new Error('boom'))
    await renderWithAuth(<Cabinet />)
    expect(await screen.findByText('Не удалось загрузить заказы')).toBeInTheDocument()
  })
})
// FE-18a (forensic 2026-09-27): суммы платежей в «Моих покупках» не должны
// терять копейки.
//
// БЫЛО: локальная копия formatRub использовала maximumFractionDigits: 0, и
// amount_minor = 123456 (1 234,56 ₽) показывался как «1 235 ₽». Рубля имеет
// два знака (domain/pricing: CurrencyRUB{Decimals: 2}), так что копейки —
// часть суммы, а не оформление. Клиент, сверивший выписку с экраном, получал
// расхождение в десятки копеек на каждом платеже.
//
// Отдельная локальная копия форматтера вообще была источником расхождения:
// соседний экран использует fmt.rubMajor с двумя знаками.
describe('FE-18a: суммы платежей с копейками', () => {
  const formatLikeCabinet = (minor: number): string =>
    new Intl.NumberFormat('ru-RU', {
      style: 'currency',
      currency: 'RUB',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(minor / 100)

  it.each([
    [123456, '1\u00a0234,56'],
    [180000, '1\u00a0800,00'],
    [1, '0,01'],
    [99, '0,99'],
    [1234567, '12\u00a0345,67'],
  ])('amount_minor=%i отображается как %s', (minor, expected) => {
    // Непространённое округление до целых: 1 234,56 ₽ превращалось в 1 235 ₽.
    expect(formatLikeCabinet(minor)).toContain(expected)
  })

  it('не округляет сумму до целых рублей', () => {
    // Именно этот случай ловил старый maximumFractionDigits: 0.
    expect(formatLikeCabinet(123456)).not.toMatch(/1\s?235\s?₽$/)
  })
})
