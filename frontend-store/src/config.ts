// Константы клиентского сайта (store): контактные данные компании.
// Значения подставляются из env на этапе сборки:
//   VITE_CONTACT_PHONE, VITE_CONTACT_EMAIL, VITE_CONTACT_ADDRESS.
// Пока env не задан — используются плейсхолдеры (dev/manual-сборка).
// Реальные контакты продакшена задаются в CI/окружении при сборке (P0-7).
const env = import.meta.env as {
  VITE_CONTACT_PHONE?: string
  VITE_CONTACT_EMAIL?: string
  VITE_CONTACT_ADDRESS?: string
}

function norm(raw: string | undefined, fallback: string): string {
  const v = raw?.trim()
  return v && v.length > 0 ? v : fallback
}

const phone = norm(env.VITE_CONTACT_PHONE, '+7 (___) ___-__-__')
const email = norm(env.VITE_CONTACT_EMAIL, 'info@stair-platform.ru')

export const CONTACTS = {
  phone,
  phoneHref: `tel:${phone.replace(/[^\d+]/g, '')}`,
  email,
  emailHref: `mailto:${email}`,
  address: norm(env.VITE_CONTACT_ADDRESS, 'Москва, Ленинградский проспект, 36с1'),
} as const

// Ключ localStorage согласия переехал в @shared/consent (CONSENT_KEY): им
// пользуются и витрина, и гейт Sentry. Дублировать константу в двух пакетах
// значило бы однажды получить два разных ключа и «вечное» согласие.
