import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  CONSENT_KEY,
  CONSENT_VERSION,
  grantConsent,
  hasConsent,
  onConsentChange,
  readConsent,
  revokeConsent,
} from './consent'
import { EVENTS, flush, start, track } from './analytics'

// Гейт согласия и отправка событий.
//
// Проверяется ровно то, что ломается молча: без согласия не уходит НИЧЕГО,
// с согласием уходит, и отказ согласия прекращает отправку немедленно.

function stubFetch() {
  const calls: Array<{ url: string; init?: RequestInit }> = []
  const f = vi.fn(async (url: string, init?: RequestInit) => {
    calls.push({ url, init })
    return new Response('{"accepted":1,"dropped":0}', { status: 202 })
  })
  vi.stubGlobal('fetch', f)
  return calls
}

beforeEach(() => {
  localStorage.clear()
  sessionStorage.clear()
  vi.useFakeTimers()
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('согласие', () => {
  it('отсутствует по умолчанию', () => {
    expect(hasConsent()).toBe(false)
    expect(readConsent()).toBeNull()
  })

  it('записывается с версией политики', () => {
    grantConsent()
    expect(hasConsent()).toBe(true)
    const rec = readConsent()
    expect(rec?.v).toBe(CONSENT_VERSION)
    expect(rec?.at).toBeTruthy()
  })

  it('отзыв снимает согласие', () => {
    grantConsent()
    revokeConsent()
    expect(hasConsent()).toBe(false)
    expect(localStorage.getItem(CONSENT_KEY)).toBeNull()
  })

  // Согласие под старой версией политики — это НЕ согласие: появился новый
  // получатель данных, и у этого посетителя его не спрашивали.
  it('согласие под другой версией не действует', () => {
    localStorage.setItem(CONSENT_KEY, JSON.stringify({ v: CONSENT_VERSION + 1, at: '' }))
    expect(hasConsent()).toBe(false)
  })

  // Старый баннер писал просто 'accepted'. Такой формат засчитываем как
  // версию 1, иначе баннер начал бы выскакивать у всех, кто нажал кнопку
  // до появления версий.
  it('старый формат accepted считается согласием версии 1', () => {
    localStorage.setItem(CONSENT_KEY, 'accepted')
    expect(hasConsent()).toBe(CONSENT_VERSION === 1)
  })

  // Испорченное значение трактуется как отсутствие согласия: сомневаться
  // надо в сторону «не отправляем».
  it('испорченное значение — это отсутствие согласия', () => {
    localStorage.setItem(CONSENT_KEY, '{это не json')
    expect(hasConsent()).toBe(false)
    localStorage.setItem(CONSENT_KEY, '"строка"')
    expect(hasConsent()).toBe(false)
  })

  it('уведомляет подписчиков', () => {
    const seen: boolean[] = []
    const off = onConsentChange((g) => seen.push(g))
    grantConsent()
    revokeConsent()
    off()
    revokeConsent()
    expect(seen).toEqual([true, false])
  })
})

describe('сбор событий', () => {
  it('БЕЗ согласия не отправляет ничего', async () => {
    const calls = stubFetch()
    track(EVENTS.pageView, { screen: 'landing' })
    await flush()
    expect(calls).toHaveLength(0)
  })

  it('с согласием отправляет пачку с версией политики', async () => {
    const calls = stubFetch()
    grantConsent()
    track(EVENTS.pageView, { screen: 'landing' })
    track(EVENTS.constructorOpen)
    await flush()

    expect(calls).toHaveLength(1)
    const body = JSON.parse(String(calls[0].init?.body))
    expect(body.consent_version).toBe(CONSENT_VERSION)
    expect(body.session_id).toMatch(/^[0-9a-f-]{36}$/)
    // Три события: первое обращение к track() само открывает визит
    // (session.start), иначе визит начался бы в никуда — отчёт увидел бы
    // шаги без начала и посчитал их частью чужой сессии.
    expect(body.events.map((e: { name: string }) => e.name)).toEqual([
      EVENTS.sessionStart,
      EVENTS.pageView,
      EVENTS.constructorOpen,
    ])
    expect(body.events[1].props.screen).toBe('landing')
    expect(calls[0].url).toContain('/api/v1/public/analytics:events')
  })

  it('отказ согласия останавливает отправку', async () => {
    const calls = stubFetch()
    grantConsent()
    revokeConsent()
    track(EVENTS.pageView)
    await flush()
    expect(calls).toHaveLength(0)
  })

  it('session.leave несёт последнее событие и длительность', async () => {
    const beacons: Blob[] = []
    vi.stubGlobal('navigator', {
      ...globalThis.navigator,
      sendBeacon: (_url: string, blob: Blob) => {
        beacons.push(blob)
        return true
      },
      language: 'ru-RU',
    })
    grantConsent()
    start()
    track(EVENTS.constructorOpen)
    track(EVENTS.blockerField, { reason: 'widthMM' })

    vi.advanceTimersByTime(5000)
    globalThis.dispatchEvent(new Event('pagehide'))

    expect(beacons).toHaveLength(1)
    const payload = JSON.parse(await beacons[0].text())
    const leave = payload.events.find((e: { name: string }) => e.name === EVENTS.sessionLeave)
    expect(leave).toBeTruthy()
    // Последнее осмысленное событие — ответ на вопрос «где бросил».
    expect(leave.props.last).toBe(EVENTS.blockerField)
    expect(typeof leave.props.session_seconds).toBe('number')
  })

  it('режет длинные значения и лишние ключи', async () => {
    const calls = stubFetch()
    grantConsent()
    const long: Record<string, string> = {}
    for (let i = 0; i < 30; i++) long[`k${i}`] = 'v'
    long.long = 'я'.repeat(200)
    track(EVENTS.blockerField, long)
    await flush()

    const props = JSON.parse(String(calls[0].init?.body)).events[0].props
    expect(Object.keys(props).length).toBeLessThanOrEqual(12)
    expect(String(props.long).length).toBeLessThanOrEqual(64)
  })

  it('start() без согласия не шлёт session.start', async () => {
    const calls = stubFetch()
    start()
    await flush()
    expect(calls).toHaveLength(0)
  })
})
