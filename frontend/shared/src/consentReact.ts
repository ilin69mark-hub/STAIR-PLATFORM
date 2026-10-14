// React-обёртки над согласием. Отдельный файл, чтобы consent.ts остался
// модулем без React: его читает и Sentry, которому хук не нужен.

import { useSyncExternalStore } from 'react'
import { CONSENT_KEY, hasConsent, onConsentChange } from './consent'

// Подписка должна быть на ДВА источника, и это не формальность:
//   1. внутренние слушатели consent.ts — согласие, данное в ЭТОЙ вкладке;
//   2. событие storage — согласие, данное в другой вкладке.
//
// Только storage — типичная ошибка: в своей вкладке localStorage не шлёт
// событие change, и подписчик молчал. Последствие было неочевидным: верх
// воронки (открытие конструктора) не записывался ровно у тех, кто нажал
// «Разрешить» на самом конструкторе, то есть у большинства.
function subscribe(cb: () => void): () => void {
  const offConsent = onConsentChange(cb)
  const onStorage = (e: StorageEvent) => {
    if (e.key === CONSENT_KEY) cb()
  }
  globalThis.addEventListener?.('storage', onStorage)
  return () => {
    offConsent()
    globalThis.removeEventListener?.('storage', onStorage)
  }
}

/**
 * Есть ли согласие, с реактивным обновлением.
 *
 * Нужно там, где событие может произойти ДО согласия, а пропустить его нельзя:
 * баннер виден поверх конструктора, и большинство посетителей нажимают
 * «Разрешить» уже находясь внутри воронки. Без этой подписки верх воронки
 * терялся бы ровно у тех, кто согласился на конструкторе, и отчёт
 * показывал бы неверный вход в воронку.
 */
export function useConsentGranted(): boolean {
  return useSyncExternalStore(subscribe, hasConsent, () => false)
}
