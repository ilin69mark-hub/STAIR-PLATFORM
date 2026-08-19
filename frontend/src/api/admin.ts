// Enterprise admin API (Phase G, EDR-0016): пользователи, политики,
// экспорт данных tenant и API-ключи. Все эндпоинты требуют admin.

import { del, get, patch, post, put } from './client'
import type {
  AdminOverview,
  AdminPolicy,
  AdminUser,
  ApiKey,
  CreateTestimonialRequest,
  OrderDTO,
  TestimonialDTO,
} from '@shared/types'

export interface UpdateUserRequest {
  role?: 'user' | 'admin'
  status?: 'active' | 'disabled'
}

export interface CreateApiKeyRequest {
  name: string
  scopes: string[]
}

export interface CreateApiKeyResponse extends ApiKey {
  token?: string
}

export const adminApi = {
  overview: () => get<AdminOverview>('/api/v1/admin/overview'),

  listUsers: () => get<AdminUser[]>('/api/v1/admin/users'),

  updateUser: (id: string, body: UpdateUserRequest) =>
    patch<{ status: string }>(`/api/v1/admin/users/${id}`, body),

  getSettings: () => get<AdminPolicy>('/api/v1/admin/settings'),

  updateSettings: (body: AdminPolicy) =>
    put<AdminPolicy>('/api/v1/admin/settings', body),

  exportUrl: (scope: string, format: 'json' | 'csv') =>
    `/api/v1/admin/export?scope=${scope}&format=${format}`,

  listApiKeys: () => get<ApiKey[]>('/api/v1/admin/api-keys'),

  createApiKey: (body: CreateApiKeyRequest) =>
    post<CreateApiKeyResponse>('/api/v1/admin/api-keys', body),

  revokeApiKey: (id: string) =>
    del(`/api/v1/admin/api-keys/${id}`),

  // Заказы клиентского сайта (store): лиды для менеджера.
  listOrders: () => get<OrderDTO[]>('/api/v1/admin/orders'),

  updateOrderStatus: (id: string, status: string) =>
    patch<OrderDTO>(`/api/v1/admin/orders/${id}/status`, { status }),

  // Отзывы клиентов (store): CRUD в админке.
  listTestimonials: () => get<TestimonialDTO[]>('/api/v1/admin/testimonials'),

  createTestimonial: (body: CreateTestimonialRequest) =>
    post<TestimonialDTO>('/api/v1/admin/testimonials', body),

  updateTestimonial: (id: string, body: CreateTestimonialRequest) =>
    patch<TestimonialDTO>(`/api/v1/admin/testimonials/${id}`, body),

  deleteTestimonial: (id: string) =>
    del(`/api/v1/admin/testimonials/${id}`),
}
