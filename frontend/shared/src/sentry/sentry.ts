// Ленивая Sentry-интеграция фронтов (S-140).
//
// SDK не грузится в стартовый чанк: @sentry/react подтягивается динамическим
// import() после requestIdleCallback (timeout ~3s) ЛИБО при первой пойманной
// ошибке — что раньше. До готовности SDK ошибки копятся в маленькой очереди
// и сливаются после инициализации.
//
// DSN берётся из VITE_SENTRY_DSN; пустое значение = режим «без Sentry»
// (все функции no-op, приложение работает как раньше) — S-118 отложен,
// реальный DSN появится позже.
//
// PII-политика (как на бэке): IP и ip_address дропаются, «парольные» поля
// форм удаляются, email заменяется на SHA-256-хеш (первые 12 hex).
import type { BrowserOptions, ErrorEvent } from '@sentry/react'

// Лимит очереди ошибок до инициализации SDK (защита от неконтролируемого роста).
const QUEUE_LIMIT = 20
// Deadline для requestIdleCallback — SDK рано или поздно загрузится даже без наступления идла.
const IDLE_TIMEOUT_MS = 3000
// Максимальная глубина рекурсивного скраббинга вложенных объектов.
const MAX_SCRUB_DEPTH = 6

interface PendingError {
  error: unknown
  context?: Record<string, unknown>
}

// SdkModule — срез API SDK, которым пользуется модуль (полный тип из импорта).
type SdkModule = typeof import('@sentry/react')

let sdk: SdkModule | null = null
let sdkReady = false
let initStarted = false
const queue: PendingError[] = []

/** Вернуть DSN из окружения или null (Sentry выключен). */
export function getSentryDsn(): string | null {
  const raw = import.meta.env.VITE_SENTRY_DSN as unknown
  const dsn = typeof raw === 'string' ? raw.trim() : ''
  return dsn.length > 0 ? dsn : null
}

/** Sentry включён? Пустой VITE_SENTRY_DSN => выключен, всё no-op. */
export function isSentryEnabled(): boolean {
  return getSentryDsn() !== null
}

// ensureSdkLoaded — идемпотентный запуск загрузки SDK. Из captureError вызывается
// немедленно (первая ошибка = «раньше»), из initSentry — через idle/timeout.
function ensureSdkLoaded(): void {
  if (initStarted || !isSentryEnabled()) return
  initStarted = true
  const load = (): void => {
    void loadAndInit()
  }
  if (typeof window !== 'undefined' && typeof window.requestIdleCallback === 'function') {
    window.requestIdleCallback(load, { timeout: IDLE_TIMEOUT_MS })
  } else {
    window.setTimeout(load, IDLE_TIMEOUT_MS)
  }
}

async function loadAndInit(): Promise<void> {
  const dsn = getSentryDsn()
  if (!dsn) return
  try {
    // Динамический import: Sentry уходит в отдельный lazy-чанк, стартовый
    // чанк не растёт на ~100КБ (S4-1 бюджет entry gzip < 300KB).
    const mod = await import('@sentry/react')
    mod.init({
      dsn,
      environment: import.meta.env.MODE,
      // PII: IP/формы/email обрабатываются до отправки события.
      beforeSend: scrubEvent,
    } satisfies BrowserOptions)
    sdk = mod
    sdkReady = true
    flushQueue()
  } catch (error) {
    // SDK не загрузился/не инициализировался — не роняем приложение и
    // позволяем повторить попытку на следующей ошибке.
    if (import.meta.env.DEV) {
      console.warn('[sentry] init failed:', error)
    }
    initStarted = false
  }
}

function flushQueue(): void {
  while (queue.length > 0) {
    const item = queue.shift()
    if (item) {
      captureException(item.error, item.context)
    }
  }
}

function captureException(error: unknown, context?: Record<string, unknown>): void {
  if (!sdk) return
  sdk.captureException(error, context ? { extra: context } : undefined)
}

/**
 * Публичная точка входа: вызвать при старте приложения (root). SDK загрузится
 * лениво после idle/timeout; если до этого произойдёт ошибка — раньше.
 */
export function initSentry(): void {
  ensureSdkLoaded()
}

/**
 * Захватить ошибку для Sentry. Возвращает eventId (для показа пользователю),
 * если событие уже ушло в SDK; undefined — если SDK ещё не готов (ошибка
 * ушла в очередь, eventId появится после init, но показать его нельзя) или
 * Sentry выключен.
 */
export function captureError(error: unknown, context?: Record<string, unknown>): Promise<string | undefined> {
  if (!isSentryEnabled()) return Promise.resolve(undefined)
  ensureSdkLoaded()
  if (sdkReady && sdk) {
    return Promise.resolve(sdk.captureException(error, context ? { extra: context } : undefined))
  }
  if (queue.length < QUEUE_LIMIT) {
    queue.push({ error, context })
  }
  return Promise.resolve(undefined)
}

// --- PII-скраббинг ---------------------------------------------------------

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/
// Ключи/маркеры «парольной» информации — значение дропается целиком.
const SENSITIVE_MARKERS = [
  'password',
  'passwd',
  'pwd',
  'secret',
  'token',
  'authorization',
  'api_key',
  'apikey',
  'credential',
]

function isSensitiveKey(key: string): boolean {
  const lower = key.toLowerCase()
  return SENSITIVE_MARKERS.some((m) => lower.includes(m))
}

function containsSensitiveForm(value: unknown): boolean {
  if (typeof value !== 'string') return false
  const lower = value.toLowerCase()
  return SENSITIVE_MARKERS.some((m) => lower.includes(`${m}=`))
}

/** SHA-256(email).slice(0, 12) — как на бэке. Пустая строка = нет WebCrypto. */
export async function hashEmail(email: string): Promise<string> {
  const normalized = email.trim().toLowerCase()
  try {
    const data = new TextEncoder().encode(normalized)
    const digest = await crypto.subtle.digest('SHA-256', data)
    const hex = Array.from(new Uint8Array(digest))
      .map((b) => b.toString(16).padStart(2, '0'))
      .join('')
    return hex.slice(0, 12)
  } catch {
    // Нет WebCrypto (небезопасный контекст) — вернуть пусто, чтобы email
    // был удалён, а не утёк открытым текстом.
    return ''
  }
}

// scrubValue — рекурсивный обход: email-строки хешируются, «парольные» ключи
// и вложенные парольные формы вырезаются. undefined = значение удаляется.
async function scrubValue(value: unknown, depth = 0): Promise<unknown> {
  if (depth > MAX_SCRUB_DEPTH) return undefined
  if (typeof value === 'string') {
    if (containsSensitiveForm(value)) return undefined
    if (EMAIL_RE.test(value)) return hashEmail(value)
    return value
  }
  if (Array.isArray(value)) {
    const out: unknown[] = []
    for (const item of value) {
      const scrubbed = await scrubValue(item, depth + 1)
      if (scrubbed !== undefined) out.push(scrubbed)
    }
    return out
  }
  if (value !== null && typeof value === 'object') {
    const out: Record<string, unknown> = {}
    for (const [key, val] of Object.entries(value)) {
      if (isSensitiveKey(key)) continue
      const scrubbed = await scrubValue(val, depth + 1)
      if (scrubbed !== undefined) out[key] = scrubbed
    }
    return out
  }
  return value
}

/**
 * beforeSend-обработчик: финальный рубеж PII-скраббинга перед отправкой
 * в Sentry. Дропает IP, хеширует email, вырезает парольные поля/формы.
 */
export async function scrubEvent(event: ErrorEvent): Promise<ErrorEvent | null> {
  const next: ErrorEvent = { ...event }

  if (next.request) {
    const request = { ...next.request } as ErrorEvent['request'] & Record<string, unknown>
    // IP из request.data/query_string не дёргаем — они уже внутри data,
    // а отдельные IP-поля удаляем явно.
    delete request.ip
    if (request.data !== undefined) {
      const scrubbed = await scrubValue(request.data)
      request.data = scrubbed === undefined ? undefined : scrubbed
    }
    next.request = request
  }

  if (next.user) {
    const user = { ...next.user }
    // IP пользователя не собираем вовсе.
    if (user.ip_address !== undefined) {
      delete user.ip_address
    }
    if (typeof user.email === 'string') {
      const hash = await hashEmail(user.email)
      // Пустой хеш (нет WebCrypto) = удалить email, а не отправлять открытую строку.
      user.email = hash.length > 0 ? hash : undefined
    }
    next.user = user
  }

  if (next.extra !== undefined) {
    next.extra = (await scrubValue(next.extra)) as ErrorEvent['extra']
  }

  return next
}

// --- Test-only ---------------------------------------------------------------

/**
 * Сброс состояния модуля для тестов (не для продакшена). Тесты shared-слоя
 * живут в frontend/shared/src и не попадают в продакшен-бандл.
 */
export function resetSentryModule(): void {
  sdk = null
  sdkReady = false
  initStarted = false
  queue.length = 0
}