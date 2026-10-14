// Яндекс.Метрика на витрине — ТОЛЬКО после согласия пользователя.
//
// Три решения, которые здесь зафиксированы:
//
//  1. Гейт согласия. Метрика ставит свои cookie (YM_ID, _ym_d и другие) и
//     без согласия — это обработка персональных данных. Скрипт не
//     подгружается вообще, пока посетитель не согласился; отзыв согласия
//     убирает его из DOM и запрещает повторную инициализацию.
//
//  2. ВЕБВИЗОР ВЫКЛЮЧЁН. Он записывает всё движение по странице, включая
//     ввод в формы — а в оформлении заявки есть имя, почта и телефон.
//     Ниже это не «по умолчанию», а явными флагами clickmap:false и
//     trackLinks:false, чтобы отключение нельзя было случайно вернуть
//     правкой настроек по умолчанию.
//
//  3. ID счётчика приходит из настроек магазина (там же, где его вводят
//     в админке магазина) и проверяется: в подтверждение попадает только
//     цифры. Поле в настройках — свободный текст, и без проверки в него
//     можно было бы подставить произвольную строку.
//
// Чего Метрика НЕ даст и что остаётся на нашей воронке: какое поле человек
// оставил пустым и на каком шаге бросил. Эти данные у нас собирает первый
// party-коллектор (миграция 000035), и именно они отвечают на вопрос
// «где спотыкается».

import { hasConsent, onConsentChange } from '@shared/consent'
import { get } from '@shared/storefront/api/client'

const SCRIPT_SRC = 'https://mc.yandex.ru/metrika/tag.js'
const SCRIPT_ID = 'sp-metrika'

/** Минимальный срез публичных настроек, который нужен здесь. */
interface PublicStoreSettings {
  counters?: { yandex_metrika_id?: string; ga4_measurement_id?: string }
}

let currentId = ''
let started = false

/**
 * Проверяет значение счётчика и возвращает ID либо пустую строку.
 *
 * В настройках магазина поле — свободный текст, а ID уходит в URL
 * стороннего скрипта и в вызов счётчика, поэтому берём ТОЛЬКО чистые цифры
 * длиной 1–12.
 *
 * Важно, что не «выкинуть не-цифры», а отвергнуть целиком. Наивный вариант
 * превращал `<img onerror=alert(1)>` в ID «1» и молча подключал ЧУЖОЙ
 * счётчик — опечатка в настройках превращалась бы в чужую аналитику. Пустая
 * строка означает «счётчик не настроен», и витрина просто работает без него.
 */
export function normalizeCounterId(raw: string | null | undefined): string {
  const value = String(raw ?? '').trim()
  return /^\d{1,12}$/.test(value) ? value : ''
}

/** Текущий загруженный ID (для тестов и диагностики). */
export function loadedCounterId(): string {
  return currentId
}

/** Загружен ли счётчик прямо сейчас. */
export function isMetrikaLoaded(): boolean {
  return currentId !== ''
}

/**
 * Подключает счётчик. Возвращает false, если подключать нечего: нет
 * согласия, нет ID или скрипт уже стоит.
 */
export function loadMetrika(rawId: string | null | undefined): boolean {
  const id = normalizeCounterId(rawId)
  if (id === '' || !hasConsent() || currentId === id) return false
  if (typeof document === 'undefined') return false

  const w = window as unknown as { ym?: (id: number | string, ...args: unknown[]) => void }
  // Метрика складывает вызовы в очередь до загрузки скрипта, поэтому порядок
  // такой: очередь → скрипт → init.
  w.ym = w.ym || function (...args: unknown[]) {
    ;((w.ym as unknown as { a?: unknown[] }).a ??= []).push(args)
  }
  w.ym(id, 'init', {
    // Вебвизор и производные от него карты — выключены осознанно.
    clickmap: false,
    trackLinks: false,
    accurateTrackBounce: false,
    webvisor: false,
  })

  if (!document.getElementById(SCRIPT_ID)) {
    const s = document.createElement('script')
    s.id = SCRIPT_ID
    s.async = true
    s.defer = true
    s.src = SCRIPT_SRC
    document.head.appendChild(s)
  }
  currentId = id
  return true
}

/**
 * Снимает счётчик после отзыва согласия.
 *
 * Ограничение честно проговорено: загруженный сторонний скрипт нельзя
 * выгрузить из памяти. Поэтому здесь мы убираем тег из DOM, чистим очередь
 * и запрещаем повторную инициализацию, а фактическое прекращение сбора
 * происходит при следующей загрузке страницы — это же сказано в политике.
 */
export function unloadMetrika(): void {
  if (typeof document !== 'undefined') {
    document.getElementById(SCRIPT_ID)?.remove()
  }
  const w = window as unknown as { ym?: unknown; _ym?: unknown }
  delete w.ym
  delete w._ym
  currentId = ''
}

/**
 * Подписывается на согласие и подключает счётчик, когда оно есть.
 * Вызывается один раз при старте витрины.
 */
export function startMetrika(): void {
  if (started) return
  started = true

  const load = () => {
    if (!hasConsent()) return
    void get<PublicStoreSettings>('/api/v1/public/store-settings')
      .then((s) => {
        if (!hasConsent()) return // согласие могли отозвать, пока летел запрос
        loadMetrika(s?.counters?.yandex_metrika_id)
      })
      // Настройки не получились — витрина просто работает без счётчика.
      .catch(() => {})
  }

  if (hasConsent()) load()
  onConsentChange((granted) => {
    if (granted) load()
    else unloadMetrika()
  })
}

/** Сброс для тестов. */
export function resetMetrikaModule(): void {
  currentId = ''
  started = false
}
