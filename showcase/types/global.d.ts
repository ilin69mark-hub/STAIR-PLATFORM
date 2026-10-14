// Окружение витрины повторяет Vite-витрину: shared-код читает VITE_* через
// import.meta.env, в Next это process.env. Значения по умолчанию — безопасные
// (Sentry выключен, тестовый API — локальный Go).
interface ImportMetaEnv {
  readonly VITE_SENTRY_DSN?: string
  readonly VITE_API_URL?: string
  readonly VITE_SENTRY_ENV?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.css'
