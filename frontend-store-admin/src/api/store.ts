// Контракты данных панели магазина — зеркало DTO бэкенда
// (internal/application/store, internal/transport/http).

export interface StoreContacts {
  phone: string
  email: string
  address: string
  work_hours: string
}

export interface StoreCompany {
  name: string
  legal_name: string
  inn: string
  ogrn: string
  email: string
  website: string
}

export interface StoreSocial {
  telegram: string
  vk: string
  whatsapp: string
  youtube: string
}

export interface StoreSEO {
  default_title: string
  default_description: string
  og_image: string
}

export interface StoreCounters {
  yandex_metrika_id: string
  ga4_measurement_id: string
}

export interface StoreRates {
  machine_per_hour_rub: number
  labor_per_hour_rub: number
  overhead_percent: number
  margin_percent: number
  discount_percent: number
  tax_percent: number
}

export interface StoreSettings {
  contacts: StoreContacts
  company: StoreCompany
  social: StoreSocial
  seo: StoreSEO
  counters: StoreCounters
  rates: StoreRates
  updated_by?: string
  updated_at: string
}

export interface MaterialPrice {
  code: string
  price_per_kg_rub: number
  overridden: boolean
}

import { api } from './client'

export interface MaterialCatalogItem {
  code: string
  name: string
  name_ru?: string
  category: string
  density_kg_m3: number
  min_thickness_mm: number
  max_thickness_mm: number
  max_width_mm?: number
  max_height_mm?: number
  price_per_kg_rub: number
  swatch_url: string
  finishes?: string[]
}

export interface PaymentTier {
  id: string
  title: string
  description?: string
  amount_minor: number
  amount_rub: number
  currency: string
}

export interface OrderContact {
  name: string
  email: string
  phone?: string
}

export const ORDER_STATUSES = [
  'new',
  'priced',
  'confirmed',
  'in_progress',
  'completed',
  'cancelled',
] as const

export type OrderStatus = (typeof ORDER_STATUSES)[number]
export type OrderKind = 'order' | 'consultation'

export interface AdminOrder {
  id: string
  kind: OrderKind | string
  status: OrderStatus
  contact: OrderContact
  config: unknown
  price: unknown
  project_id?: string
  created_at: string
  updated_at: string
}

export const PAYMENT_STATUSES = ['pending', 'paid', 'failed', 'refunded'] as const

export type PaymentStatus = (typeof PAYMENT_STATUSES)[number]

export interface AdminPayment {
  id: string
  tier_id?: string
  project_id?: string
  amount_minor: number
  currency: string
  status: PaymentStatus
  provider: string
  created_at: string
  paid_at?: string
}

export const storeApi = {
  settings: () => api.get<StoreSettings>('/api/v1/admin/store/settings'),
  saveSettings: (settings: StoreSettings) => api.put<StoreSettings>('/api/v1/admin/store/settings', settings),
  prices: () => api.get<MaterialPrice[]>('/api/v1/admin/store/prices'),
  setPrice: (code: string, pricePerKgRub: number) =>
    api.put<MaterialPrice>('/api/v1/admin/store/prices', { code, price_per_kg_rub: pricePerKgRub }),
  resetPrice: (code: string) => api.delete(`/api/v1/admin/store/prices/${encodeURIComponent(code)}`),
  materials: () => api.get<MaterialCatalogItem[]>('/api/v1/public/materials'),
  tiers: () => api.get<PaymentTier[]>('/api/v1/public/payment-tiers'),
  orders: () => api.get<AdminOrder[]>('/api/v1/admin/orders'),
  updateOrderStatus: (id: string, status: OrderStatus) =>
    api.patch<AdminOrder>(`/api/v1/admin/orders/${encodeURIComponent(id)}/status`, { status }),
  payments: () => api.get<AdminPayment[]>('/api/v1/admin/payments'),
  refundPayment: (id: string) =>
    api.post<AdminPayment>(`/api/v1/admin/payments/${encodeURIComponent(id)}/refund`, undefined),
}
