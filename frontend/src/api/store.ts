// Контракты и ручки вкладки «Магазин» — зеркало DTO бэкенда
// (internal/application/store, internal/transport/http). Перенесено из
// отдельного приложения frontend-store-admin при слиянии кабинетов.
//
// Заказы здесь намеренно НЕ дублируются: те же два эндпоинта уже есть в
// api/admin.ts (listOrders / updateOrderStatus), а экран один — табличный.

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

// FastenerPrice — позиция прайса крепежа, ₽ за штуку (MNT-0001).
//
// name_ru/diameter_mm/length_mm приходят с бэкенда из каталога: встроенного
// плюс добавленные магазином. Админка не держит свою копию названий, поэтому
// позиция, добавленная через интерфейс, появляется здесь сразу.
export interface FastenerPrice {
  code: string
  name_ru: string
  kind: string
  diameter_mm: number
  length_mm: number
  price_per_pc_rub: number
  overridden: boolean
}

// FastenerPosition — своя позиция крепежа, добавленная магазином (000048).
//
// Отдельный тип, а не переиспользование FastenerPrice: у позиции есть вид,
// размер и материал, а у строки прайса — только код и цена. Смешивать их в
// одной форме значило бы заполнять неиспользуемые поля нулями.
export interface FastenerPosition {
  code: string
  name_ru: string
  kind: string
  diameter_mm: number
  length_mm: number
  base: string
  price_per_pc_rub: number
}

import { del, get, post, put } from './client'

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

// SealantPrice — позиция прайса герметика, ₽ за картридж (CONF-SEALANT).
//
// name_ru/volume_ml/layer_mm приходят с бэкенда из встроенного каталога
// движка: админка не держит свою копию описаний, поэтому новая позиция в
// коде появляется здесь сама.
// PurchasedPrice — закупочная цена покупной детали ограждения, ₽ за штуку.
//
// kind — ВИД детали, а не код позиции: у каждого экземпляра спецификации свой
// номер (PST-01, PST-02, …), и прайс по нему построить нельзя.
export interface PurchasedPrice {
  kind: string
  catalog_id: string
  name_ru: string
  price_per_pc_rub: number
  overridden: boolean
}

export interface SealantPrice {
  code: string
  name_ru: string
  kind: string
  volume_ml: number
  layer_mm: number
  density_kg_per_l: number
  price_per_cartridge_rub: number
  overridden: boolean
}

export const storeApi = {
  // Настройки витрины — store.settings.*
  settings: () => get<StoreSettings>('/api/v1/admin/store/settings'),
  saveSettings: (settings: StoreSettings) =>
    put<StoreSettings>('/api/v1/admin/store/settings', settings),
  // Прайс материалов — store.prices.*
  // Сохранение ОДНОЙ группы настроек (EDR-0041 §6): без этого «только SEO»
  // означало бы «и реквизиты тоже» — PUT перезаписывал весь документ.
  saveSettingsGroup: (
    group: 'contacts' | 'company' | 'social' | 'seo' | 'rates',
    settings: StoreSettings,
  ) => put<StoreSettings>(`/api/v1/admin/store/settings/${group}`, settings),
  prices: () => get<MaterialPrice[]>('/api/v1/admin/store/prices'),
  setPrice: (code: string, pricePerKgRub: number) =>
    put<MaterialPrice>('/api/v1/admin/store/prices', { code, price_per_kg_rub: pricePerKgRub }),
  resetPrice: (code: string) => del<void>(`/api/v1/admin/store/prices/${encodeURIComponent(code)}`),
  // Прайс крепежа — store.fasteners.*. Отдельный ресурс, а не поле у цен
  // материалов: единица другая (шт, а не кг).
  fasteners: () => get<FastenerPrice[]>('/api/v1/admin/store/fasteners'),
  setFastenerPrice: (code: string, pricePerPcRub: number) =>
    put<FastenerPrice>('/api/v1/admin/store/fasteners', { code, price_per_pc_rub: pricePerPcRub }),
  resetFastenerPrice: (code: string) =>
    del<void>(`/api/v1/admin/store/fasteners/${encodeURIComponent(code)}`),
  // Свои позиции крепежа (000048). Отдельный ресурс, а не поле прайса: у
  // позиции есть вид, размер и материал, а у строки прайса — только цена.
  fastenerPositions: () => get<FastenerPosition[]>('/api/v1/admin/store/fastener-positions'),
  createFastenerPosition: (p: FastenerPosition) =>
    post<FastenerPosition>('/api/v1/admin/store/fastener-positions', p),
  deleteFastenerPosition: (code: string) =>
    del<void>(`/api/v1/admin/store/fastener-positions/${encodeURIComponent(code)}`),
  // Прайс герметика — store.sealants.*, ₽ за картридж.
  sealants: () => get<SealantPrice[]>('/api/v1/admin/store/sealants'),
  setSealantPrice: (code: string, pricePerCartridgeRub: number) =>
    put<SealantPrice>('/api/v1/admin/store/sealants', {
      code,
      price_per_cartridge_rub: pricePerCartridgeRub,
    }),
  resetSealantPrice: (code: string) =>
    del<void>(`/api/v1/admin/store/sealants/${encodeURIComponent(code)}`),
  // Закупочный прайс ограждения — store.purchased.*, ₽ за штуку по виду детали.
  purchased: () => get<PurchasedPrice[]>('/api/v1/admin/store/purchased'),
  setPurchasedPrice: (kind: string, pricePerPcRub: number) =>
    put<PurchasedPrice>('/api/v1/admin/store/purchased', { kind, price_per_pc_rub: pricePerPcRub }),
  resetPurchasedPrice: (kind: string) =>
    del<void>(`/api/v1/admin/store/purchased/${encodeURIComponent(kind)}`),
  // Каталоги — публичные эндпоинты, без проверки прав
  materials: () => get<MaterialCatalogItem[]>('/api/v1/public/materials'),
  tiers: () => get<PaymentTier[]>('/api/v1/public/payment-tiers'),
  // Платежи — payments.*
  payments: () => get<AdminPayment[]>('/api/v1/admin/payments'),
  refundPayment: (id: string) =>
    post<AdminPayment>(`/api/v1/admin/payments/${encodeURIComponent(id)}/refund`, undefined),
}
