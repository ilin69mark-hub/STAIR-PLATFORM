// Store API: публичный расчёт предварительной цены и заказы.
// Расчёт (quote) — анонимный, без аутентификации; заказ создаётся
// зарегистрированным пользователем (лид для менеджера).

import { get, post } from './client'
import { ApiError } from '@shared/types'
import type {
  CreateConsultationRequest,
  CreateOrderRequest,
  OrderDTO,
  QuoteResult,
  QuoteValidation,
  TestimonialDTO,
} from '@shared/types'

// Backend дедуплицирует тяжёлые POST (quote) по IP+метод+путь+хэш тела и
// отвечает 429 «duplicate request in progress», пока первый расчёт с тем же
// телом в обработке. Ретраим (короткая пауза), чтобы параллельные расчёты
// с одного IP (две вкладки, e2e) не падали с «HTTP 429».
const quoteSleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))

const QUOTE_RETRY_DELAYS_MS = [500, 1000]

export const quoteApi = {
  // POST /api/v1/public/stairs:quote — предварительный расчёт (анонимно).
  // При 429 (dedup/лимитирование) делает до 2 повторных попыток.
  // POST /api/v1/public/stairs:validate — живая валидация при вводе (S-P5).
  // Анонимный эндпоинт: только блок validation (без геометрии, производства,
  // цены и записей). Вызывается из конструктора с дебаунсом при вводе.
  async validate(config: Record<string, unknown>): Promise<QuoteValidation> {
    const res = await post<{ validation: QuoteValidation }>(
      '/api/v1/public/stairs:validate',
      config,
    )
    return res.validation
  },

  async calculate(config: Record<string, unknown>): Promise<QuoteResult> {
    for (let attempt = 0; ; attempt++) {
      try {
        return await post<QuoteResult>('/api/v1/public/stairs:quote', config)
      } catch (e) {
        const delay = QUOTE_RETRY_DELAYS_MS[attempt]
        if (e instanceof ApiError && e.status === 429 && delay !== undefined) {
          await quoteSleep(delay)
          continue
        }
        throw e
      }
    }
  },
}

// Каталог платных услуг и покупка услуги (этап 4). Прайс приходит с сервера:
// клиент сумму не присылает (S-150).
export interface PaymentTier {
  id: string
  title: string
  description?: string
  amount_minor: number
  currency: string
  amount_rub: number
}

export interface MyPayment {
  id: string
  title: string
  tier_id?: string
  amount_minor: number
  currency: string
  status: 'pending' | 'paid' | 'failed' | 'refunded'
  created_at: string
  paid_at?: string
}

export interface ServiceCheckout {
  payment_id: string
  tier_id: string
  title: string
  amount_minor: number
  currency: string
  checkout_url: string
}

export const paymentsApi = {
  /** Каталог услуг для витрины (публичный GET). */
  async tiers(): Promise<PaymentTier[]> {
    return get<PaymentTier[]>('/api/v1/public/payment-tiers')
  },
  /** Покупка услуги: возвращает URL страницы оплаты PSP. */
  async checkout(tierId: string): Promise<ServiceCheckout> {
    return post<ServiceCheckout>('/api/v1/public/services/checkout', { tier_id: tierId })
  },
  /** Мои покупки (кабинет). */
  async mine(): Promise<MyPayment[]> {
    return get<MyPayment[]>('/api/v1/payments/mine')
  },
}

export const ordersApi = {
  // POST /api/v1/orders — создание заказа (auth+CSRF).
  create: (body: CreateOrderRequest) => post<OrderDTO>('/api/v1/orders', body),
  // GET /api/v1/orders — мои заказы.
  listMine: () => get<OrderDTO[]>('/api/v1/orders'),
}

export const testimonialsApi = {
  // GET /api/v1/public/testimonials — опубликованные отзывы для лендинга.
  published: () => get<TestimonialDTO[]>('/api/v1/public/testimonials'),
}

export const consultationsApi = {
  // POST /api/v1/public/orders — анонимная консультация (лид для менеджера).
  create: (body: CreateConsultationRequest) => post<OrderDTO>('/api/v1/public/orders', body),
}
