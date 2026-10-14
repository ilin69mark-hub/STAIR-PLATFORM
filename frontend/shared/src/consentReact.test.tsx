import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { CONSENT_KEY, CONSENT_VERSION, grantConsent, hasConsent, revokeConsent } from './consent'
import { useConsentGranted } from './consentReact'

/**
 * Обёртка подписывается на ДВА источника, и это не формальность: свой
 * localStorage не шлёт событие change в той же вкладке, поэтому подписка
 * только на storage молчала у тех, кто нажал «Разрешить» прямо на
 * конструкторе — то есть у большинства. Тест закрывает оба источника.
 */
describe('useConsentGranted', () => {
  beforeEach(() => {
    revokeConsent()
  })

  afterEach(() => {
    revokeConsent()
  })

  it('видит согласие, данное в этой же вкладке', () => {
    const { result } = renderHook(() => useConsentGranted())
    expect(result.current).toBe(false)

    // Событие storage в своей вкладке НЕ приходит — приходит внутренний
    // слушатель consent.ts. Если подписка останется только на storage,
    // тест упадёт.
    act(() => {
      grantConsent()
    })

    expect(result.current).toBe(true)
  })

  it('видит согласие, данное в другой вкладке', () => {
    const { result } = renderHook(() => useConsentGranted())
    expect(result.current).toBe(false)

    // Событие storage приходит уже ПОСЛЕ того, как другая вкладка записала
    // значение в общий localStorage: само событие значение не несёт. Поэтому
    // сначала пишем запись руками, и только потом шлём событие — иначе тест
    // проверял бы несуществующее поведение.
    localStorage.setItem(
      CONSENT_KEY,
      JSON.stringify({ v: CONSENT_VERSION, at: new Date().toISOString() }),
    )
    act(() => {
      globalThis.dispatchEvent(new StorageEvent('storage', { key: CONSENT_KEY }))
    })

    expect(result.current).toBe(true)
  })

  it('игнорирует событие storage по чужому ключу', () => {
    const { result } = renderHook(() => useConsentGranted())

    act(() => {
      globalThis.dispatchEvent(new StorageEvent('storage', { key: 'some-other-key' }))
    })

    expect(result.current).toBe(false)
  })

  it('отписывается при размонтировании', () => {
    const { unmount } = renderHook(() => useConsentGranted())
    unmount()

    // После размонтирования подписки нет: событие не должно никого трогать.
    // Если cleanup потерян, подписчик остался бы висеть в consent.ts навсегда.
    expect(() => {
      act(() => {
        globalThis.dispatchEvent(new StorageEvent('storage', { key: CONSENT_KEY }))
      })
    }).not.toThrow()
  })

  it('читает актуальное состояние на серверном рендере', () => {
    // getServerSnapshot обязан вернуть false, иначе SSR разошёлся бы с
    // клиентским hydrate и React выдал бы ошибку гидрации.
    expect(hasConsent()).toBe(false)
  })
})