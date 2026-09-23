// Тесты Sentry-интеграции (S-140): ленивый инит, очередь, PII-скраб.
// @sentry/react замокан — тесты не грузят реальный SDK и не шлют события.
import { webcrypto } from 'node:crypto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// jsdom — «небезопасный контекст»: crypto.subtle там отсутствует. Подменяем
// глобальный crypto на Node WebCrypto, чтобы hashEmail (SHA-256) работал.
Object.defineProperty(globalThis, 'crypto', { value: webcrypto, configurable: true })

const initMock = vi.fn()
const captureExceptionMock = vi.fn((_e: unknown, _c?: unknown) => 'evt-1')

vi.mock('@sentry/react', () => ({
  init: initMock,
  captureException: captureExceptionMock,
}))

// Модуль-под-тест импортируем ПОСЛЕ vi.mock (hoisted).
import { captureError, getSentryDsn, hashEmail, initSentry, isSentryEnabled, scrubEvent } from './sentry'
import { resetSentryModule } from './sentry'

async function flushMicrotasks(): Promise<void> {
  // Дождаться цепочки promise внутри loadAndInit/flushQueue.
  for (let i = 0; i < 5; i++) {
    await Promise.resolve()
  }
}

describe('sentry lazy-init (S-140)', () => {
  beforeEach(() => {
    vi.resetModules()
    resetSentryModule()
    initMock.mockClear()
    captureExceptionMock.mockClear()
    vi.unstubAllEnvs()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('без DSN: isSentryEnabled=false, initSentry не грузит SDK (no-op)', () => {
    vi.stubEnv('VITE_SENTRY_DSN', '')
    expect(getSentryDsn()).toBeNull()
    expect(isSentryEnabled()).toBe(false)

    initSentry()
    expect(initMock).not.toHaveBeenCalled()
  })

  it('без DSN: captureError возвращает undefined и не копит очередь', async () => {
    vi.stubEnv('VITE_SENTRY_DSN', '')
    const eventId = await captureError(new Error('boom'))
    expect(eventId).toBeUndefined()
    expect(captureExceptionMock).not.toHaveBeenCalled()
  })

  it('с DSN: SDK грузится лениво по idle/timeout и init вызывается ровно один раз', async () => {
    vi.useFakeTimers()
    vi.stubEnv('VITE_SENTRY_DSN', 'https://abc@example.ingest.sentry.io/1')
    expect(isSentryEnabled()).toBe(true)

    initSentry()
    // ДО таймаута — SDK ещё не инициализирован.
    expect(initMock).not.toHaveBeenCalled()

    vi.advanceTimersByTime(3000)
    await vi.advanceTimersByTimeAsync(0)
    await flushMicrotasks()

    expect(initMock).toHaveBeenCalledTimes(1)
    expect(initMock).toHaveBeenCalledWith(
      expect.objectContaining({ dsn: 'https://abc@example.ingest.sentry.io/1' }),
    )

    // Повторный initSentry не порождает второй загрузки.
    initSentry()
    vi.advanceTimersByTime(3000)
    await vi.advanceTimersByTimeAsync(0)
    await flushMicrotasks()
    expect(initMock).toHaveBeenCalledTimes(1)
  })

  it('ошибка до инициализации уходит в очередь и сливается после init', async () => {
    vi.useFakeTimers()
    vi.stubEnv('VITE_SENTRY_DSN', 'https://abc@example.ingest.sentry.io/1')

    // Ошибка ДО того, как сработал idle-таймаут.
    const p1 = captureError(new Error('early-1'))
    const p2 = captureError(new Error('early-2'), { feature: 'quote' })
    expect(captureExceptionMock).not.toHaveBeenCalled()

    vi.advanceTimersByTime(3000)
    await vi.advanceTimersByTimeAsync(0)
    await flushMicrotasks()

    expect(await p1).toBeUndefined() // очередь → eventId теряется до init (допустимо)
    expect(await p2).toBeUndefined()
    expect(captureExceptionMock).toHaveBeenCalledTimes(2)
    expect(captureExceptionMock).toHaveBeenCalledWith(expect.any(Error), {
      extra: { feature: 'quote' },
    })

    // После инициализации — события уходят сразу, с реальным eventId.
    const eventId = await captureError(new Error('late'))
    expect(eventId).toBe('evt-1')
  })

  it('очередь ограничена QUEUE_LIMIT (не растёт бесконечно)', async () => {
    vi.useFakeTimers()
    vi.stubEnv('VITE_SENTRY_DSN', 'https://abc@example.ingest.sentry.io/1')

    for (let i = 0; i < 50; i++) {
      await captureError(new Error(`err-${i}`))
    }
    vi.advanceTimersByTime(3000)
    await vi.advanceTimersByTimeAsync(0)
    await flushMicrotasks()

    // 50 событий, но в очереди максимум 20 — остальные отброшены до init.
    expect(captureExceptionMock).toHaveBeenCalledTimes(20)
  })
})

describe('sentry PII-scrubbing (S-140)', () => {
  it('hashEmail: SHA-256, lowercase, первые 12 hex', async () => {
    const h1 = await hashEmail('User@Example.COM')
    const h2 = await hashEmail('user@example.com')
    expect(h1).toBe(h2)
    expect(h1).toMatch(/^[0-9a-f]{12}$/)
    // Известный вектор: SHA-256("user@example.com").
    expect(h1).toBe('b4c9a289323b')
  })

  it('scrubEvent: дропает IP, хеширует email, вырезает парольные поля/формы', async () => {
    const event = {
      request: {
        url: 'https://x/api/v1/check',
        ip: '203.0.113.9',
        data: {
          email: 'client@example.com',
          password: 'hunter2',
          nested: { token: 'abc', ok: 1 },
          message: 'login failed password=mega',
        },
      },
      user: { id: 'u1', email: 'Client@Example.com', ip_address: '203.0.113.10' },
      extra: { credential: 'x', safe: 'y' },
    } as Parameters<typeof scrubEvent>[0]

    const scrubbed = (await scrubEvent(event))!
    expect(scrubbed.request).not.toHaveProperty('ip')
    expect(scrubbed.user).not.toHaveProperty('ip_address')
    expect(scrubbed.user?.email).toBe('f93fa2e5fb59') // хеш, не открытый email
    expect(scrubbed.request?.data).toEqual({
      email: 'f93fa2e5fb59',
      nested: { ok: 1 }, // nested.token вырезан
      // password и message (содержит password=... форму) удалены
    })
    expect(scrubbed.extra).toEqual({ safe: 'y' })
  })

  it('scrubEvent: email хешируется один раз (см. выше), null -> null-совместим', async () => {
    // beforeSend может вернуть null (отбросить событие) — проверяем, что
    // наш обработчик всегда возвращает событие (не дропает).
    const event = { extra: {} } as Parameters<typeof scrubEvent>[0]
    expect(await scrubEvent(event)).not.toBeNull()
  })
})