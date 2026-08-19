// Константы клиентского сайта (store): контактные данные компании.
// Реальные значения менеджер подставляет позже — сейчас плейсхолдеры.
export const CONTACTS = {
  phone: '+7 (___) ___-__-__',
  phoneHref: 'tel:+70000000000',
  email: 'info@stair-platform.ru',
  emailHref: 'mailto:info@stair-platform.ru',
  address: 'Москва, Ленинградский проспект, 36с1',
} as const

// Домен «политики» cookie: ключ localStorage согласия.
export const COOKIE_CONSENT_KEY = 'stair-platform-cookie-consent'