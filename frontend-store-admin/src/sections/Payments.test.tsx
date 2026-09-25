import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { AdminPayment } from '../api/store'
import { PaymentsSection } from './Payments'

const paidPayment: AdminPayment = {
  id: 'pay-paid',
  tier_id: 'design',
  amount_minor: 500000,
  currency: 'RUB',
  status: 'paid',
  provider: 'stripe',
  created_at: '2026-09-25T12:00:00Z',
  paid_at: '2026-09-25T12:05:00Z',
}

const pendingPayment: AdminPayment = {
  id: 'pay-pending',
  project_id: 'project-1',
  amount_minor: 90000,
  currency: 'RUB',
  status: 'pending',
  provider: 'mock',
  created_at: '2026-09-25T11:00:00Z',
}

function jsonResponse(payload: unknown, status = 200): Promise<Response> {
  return Promise.resolve(
    new Response(JSON.stringify(payload), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

function responseGate() {
  let resolve!: (response: Promise<Response>) => void
  const promise = new Promise<Response>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  document.cookie = 'csrf_admin=; Max-Age=0; path=/'
})

describe('PaymentsSection', () => {
  it('показывает состояние загрузки', () => {
    const gate = responseGate()
    vi.stubGlobal('fetch', vi.fn(() => gate.promise))

    render(<PaymentsSection />)

    expect(screen.getByRole('status')).toHaveTextContent('Загрузка платежей…')
  })

  it('показывает пустой список', async () => {
    vi.stubGlobal('fetch', vi.fn(() => jsonResponse([])))

    render(<PaymentsSection />)

    expect(await screen.findByText('Платежей пока нет.')).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it('показывает ошибку загрузки', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        jsonResponse({ error: { code: 'internal', message: 'Платежи недоступны' } }, 500),
      ),
    )

    render(<PaymentsSection />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Платежи недоступны')
  })

  it('показывает сумму, статус, провайдера, дату и услугу', async () => {
    vi.stubGlobal('fetch', vi.fn(() => jsonResponse([paidPayment, pendingPayment])))

    render(<PaymentsSection />)

    const paidRow = await screen.findByRole('row', { name: /pay-paid/ })
    expect(paidRow).toHaveTextContent('₽')
    expect(paidRow).toHaveTextContent('Оплачен')
    expect(paidRow).toHaveTextContent('stripe')
    expect(paidRow).toHaveTextContent(new Date(paidPayment.created_at).toLocaleString('ru-RU'))
    expect(paidRow).toHaveTextContent('design')
    expect(screen.getByRole('row', { name: /pay-pending/ })).toHaveTextContent('Проект project-1')
    expect(screen.getAllByRole('button', { name: 'Оформировать возврат' })).toHaveLength(1)
  })

  it('оформляет возврат оплаченного платежа без повторного клика', async () => {
    document.cookie = 'csrf_admin=payment-token'
    const gate = responseGate()
    const fetchMock = vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === 'POST') return gate.promise
      if (String(url).endsWith('/api/v1/admin/payments')) {
        return jsonResponse([paidPayment, pendingPayment])
      }
      return jsonResponse({ error: { code: 'not_found' } }, 404)
    })
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<PaymentsSection />)

    const refundButton = await screen.findByRole('button', { name: 'Оформировать возврат' })
    await user.click(refundButton)
    expect(refundButton).toBeDisabled()
    await user.click(refundButton)

    const refundCalls = fetchMock.mock.calls.filter(([, init]) => init?.method === 'POST')
    expect(refundCalls).toHaveLength(1)
    const [url, init] = refundCalls[0]
    expect(String(url)).toBe('/api/v1/admin/payments/pay-paid/refund')
    expect(init?.body).toBeUndefined()
    const headers = init?.headers as Record<string, string>
    expect(headers['X-App-Origin']).toBe('admin')
    expect(headers['X-CSRF-Token']).toBe('payment-token')

    await act(async () => {
      gate.resolve(jsonResponse({ ...paidPayment, status: 'refunded' }))
    })

    expect(await screen.findByText('Возврат платежа pay-paid оформлен')).toBeInTheDocument()
    expect(screen.getByText('Возвращён')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Оформировать возврат' })).not.toBeInTheDocument()
  })

  it('показывает ошибку возврата и оставляет возможность повторить', async () => {
    const fetchMock = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === 'POST') {
        return jsonResponse({ error: { code: 'conflict', message: 'Платёж уже возвращён' } }, 409)
      }
      return jsonResponse([paidPayment])
    })
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<PaymentsSection />)

    await user.click(await screen.findByRole('button', { name: 'Оформировать возврат' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Платёж уже возвращён')
    expect(screen.getByRole('button', { name: 'Оформировать возврат' })).toBeEnabled()
  })
})
