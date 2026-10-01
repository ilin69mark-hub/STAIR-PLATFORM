export type SectionId =
  | 'overview'
  | 'prices'
  | 'settings'
  | 'materials'
  | 'services'
  | 'orders'
  | 'content'
  | 'media'
  | 'metrics'
  | 'emails'
  | 'payments'

export interface SectionDef {
  id: SectionId
  title: string
}

// Порядок разделов панели магазина (волна 0). Разделы без backend-ручек
// помечены в SECTIONS_META и показывают заглушку с планом волны.
export const SECTIONS: SectionDef[] = [
  { id: 'overview', title: 'Обзор' },
  { id: 'prices', title: 'Цены' },
  { id: 'settings', title: 'Настройки' },
  { id: 'materials', title: 'Материалы' },
  { id: 'services', title: 'Услуги' },
  { id: 'orders', title: 'Заказы' },
  { id: 'content', title: 'Контент' },
  { id: 'media', title: 'Медиа' },
  { id: 'metrics', title: 'Метрики' },
  { id: 'emails', title: 'Письма' },
  { id: 'payments', title: 'Платежи' },
]

export const SECTIONS_META: Record<SectionId, { ready: boolean; note?: string }> = {
  overview: { ready: true },
  prices: { ready: true },
  settings: { ready: true },
  materials: { ready: true, note: 'Каталог материалов (MFG-0005) редактируется в прайсе и движке.' },
  services: { ready: true, note: 'Каталог услуг витрины доступен только для чтения; платежи и возвраты управляются в разделе «Платежи».' },
  orders: { ready: true },
  content: { ready: false, note: 'Секции сайта (тексты, баннеры) — волна 2.' },
  media: { ready: false, note: 'Загрузка медиа в объектное хранилище — волна 2.' },
  metrics: { ready: false, note: 'Метрики витрины (Recharts) — волна 2.' },
  emails: { ready: false, note: 'SMTP-шаблоны писем — волна 3.' },
  payments: { ready: true },
}
