// Store API: публичный расчёт предварительной цены и заказы.
// Расчёт (quote) — анонимный, без аутентификации; заказ создаётся
// зарегистрированным пользователем (лид для менеджера).

import { get, post } from './client'
import type {
  CreateConsultationRequest,
  CreateOrderRequest,
  OrderDTO,
  QuoteResult,
  TestimonialDTO,
} from '@shared/types'

export const quoteApi = {
  // POST /api/v1/public/stairs:quote — предварительный расчёт (анонимно).
  calculate: (config: Record<string, unknown>) =>
    post<QuoteResult>('/api/v1/public/stairs:quote', config),
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
