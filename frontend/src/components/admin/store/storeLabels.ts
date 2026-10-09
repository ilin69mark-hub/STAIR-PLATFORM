import type { StoreSettings } from '../../../api/store'

// Подписи полей настроек магазина. Вынесены отдельным файлом, когда
// настройки разделились по группам: каждая группа — свой раздел плитки,
// а формулировки нужны обеим.
export const CONTACT_LABELS: Record<keyof StoreSettings['contacts'], string> = {
  phone: 'Телефон',
  email: 'Email',
  address: 'Адрес',
  work_hours: 'Часы работы',
}

export const COMPANY_LABELS: Record<keyof StoreSettings['company'], string> = {
  name: 'Название',
  legal_name: 'Юридическое лицо',
  inn: 'ИНН',
  ogrn: 'ОГРН',
  email: 'Email для писем',
  website: 'Сайт',
}

export const SOCIAL_LABELS: Record<keyof StoreSettings['social'], string> = {
  telegram: 'Telegram',
  vk: 'VK',
  whatsapp: 'WhatsApp',
  youtube: 'YouTube',
}

export const COUNTER_LABELS: Record<keyof StoreSettings['counters'], string> = {
  yandex_metrika_id: 'Яндекс.Метрика (ID)',
  ga4_measurement_id: 'GA4 (Measurement ID)',
}

export const RATE_MONEY_LABELS: Record<'machine_per_hour_rub' | 'labor_per_hour_rub', string> = {
  machine_per_hour_rub: 'Станок, ₽/час',
  labor_per_hour_rub: 'Работа, ₽/час',
}

export const RATE_PERCENT_LABELS: Record<'overhead_percent' | 'margin_percent' | 'discount_percent' | 'tax_percent', string> = {
  overhead_percent: 'Накладные, %',
  margin_percent: 'Маржа, %',
  discount_percent: 'Скидка, %',
  tax_percent: 'НДС, %',
}
