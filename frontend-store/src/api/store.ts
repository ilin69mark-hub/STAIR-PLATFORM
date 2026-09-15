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
  TestimonialDTO,
} from '@shared/types'

// Backend дедуплицирует тяжёлые POST (quote) по IP+метод+путь и отвечает
// 429 «duplicate request in progress», пока первый расчёт в обработке.
// Ретраим (короткая пауза), чтобы параллельные расчёты с одного IP (две
// вкладки, e2e) не падали с «HTTP 429».
const quoteSleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))

const QUOTE_RETRY_DELAYS_MS = [500, 1000]

export const quoteApi = {
  // POST /api/v1/public/stairs:quote — предварительный расчёт (анонимно).
  // При 429 (dedup/лимитирование) делает до 2 повторных попыток.
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
