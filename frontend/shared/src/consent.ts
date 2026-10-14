// Согласие на обработку данных посетителя.
//
// Отдельный модуль, потому что согласие — это ЕДИНСТВЕННЫЙ выключатель, через
// который проходят и аналитика витрины, и Sentry. Пока его нет, ни то, ни
// другое не отправляется вообще: не «потом включим», а по умолчанию выключено.
//
// Решение хранится в localStorage (не в cookie) и С ВЕРСИЕЙ ПОЛИТИКИ.
// Версия нужна для одной вещи: когда в политике появляется новый получатель
// данных, все, кто согласился раньше, обязаны спросить заново. Без версии
// баннер показался бы один раз в жизни и больше никогда, а новый счётчик
// молча собирал бы данные у тех, кто на него не соглашался.
//
// Политика хранится как { v: <версия>, at: <ISO-дата> }.

/** Ключ в localStorage. Прежний баннер писал сюда просто 'accepted' — такое
 *  значение считается согласием на версию 1, чтобы у тех, кто нажал кнопку
 *  до появления версий, баннер не начал появляться заново. */
export const CONSENT_KEY = 'stair-platform-cookie-consent'

/** Текущая версия политики. Поднимается при каждом изменении состава
 *  получателей данных или способов обработки.
 *
 *  v2 — 2026-09-29: добавлена Яндекс.Метрика (её cookie и обработка IP).
 *      Поднятие обязательно: согласие, данное до этого, было дано на
 *      политику БЕЗ упоминания Метрики, и без новой версии её cookie поставили
 *      бы тем, кто её не видел. Ровно для этого случая версия и существует.
 *
 *  ВАЖНО: значение обязано совпадать с funnel.CurrentConsentVersion на бэке
 *  (миграция 000035). Расхождение не ломает интерфейс, а молча выбрасывает
 *  ВСЕ события: сервер отвергает версию, которой нет в его каталоге. Синхрон
 *  проверяется тестом TestConsentVersionMatchesBackend. */
export const CONSENT_VERSION = 2

export interface ConsentRecord {
  /** Версия политики, под которой дано согласие. */
  v: number
  /** Момент согласия, ISO-8601. Для «согласие N месяцев назад». */
  at: string
}

export type ConsentListener = (granted: boolean) => void

const listeners = new Set<ConsentListener>()

function notify(granted: boolean) {
  for (const l of listeners) {
    try {
      l(granted)
    } catch {
      // слушатель не должен ронять выдачу согласия
    }
  }
}

/** Читает записанное согласие. Любое повреждение значения трактуется как
 *  отсутствие согласия: сомневаться надо в сторону «не отправляем». */
export function readConsent(): ConsentRecord | null {
  try {
    const raw = localStorage.getItem(CONSENT_KEY)
    if (!raw) return null
    // Старый формат: просто строка 'accepted' — согласие на версию 1, на
    // которую тогда не было ни Метрики, ни сбора событий.
    if (raw === 'accepted') return { v: 1, at: '' }
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null) return null
    const rec = parsed as Partial<ConsentRecord>
    if (typeof rec.v !== 'number' || !Number.isFinite(rec.v)) return null
    return { v: rec.v, at: typeof rec.at === 'string' ? rec.at : '' }
  } catch {
    // localStorage недоступен (приватный режим, отключённые куки) — согласия
    // нет, и значит ничего не отправляем.
    return null
  }
}

/** Есть ли действующее согласие (записанное под текущую версию политики). */
export function hasConsent(): boolean {
  const rec = readConsent()
  return rec != null && rec.v === CONSENT_VERSION
}

/** Записывает согласие под текущую версию политики. */
export function grantConsent(): void {
  const rec: ConsentRecord = { v: CONSENT_VERSION, at: new Date().toISOString() }
  try {
    localStorage.setItem(CONSENT_KEY, JSON.stringify(rec))
  } catch {
    // Не записалось — согласие всё равно действует в этой вкладке, иначе
    // баннер «моргал» бы при каждом рендере.
  }
  notify(true)
}

/** Отзывает согласие: события перестают уходить немедленно, а не по
 *  истечении срока хранения. */
export function revokeConsent(): void {
  try {
    localStorage.removeItem(CONSENT_KEY)
  } catch {
    // см. grantConsent
  }
  notify(false)
}

/** Подписка на изменение согласия (в этой вкладке). Возвращает отписку. */
export function onConsentChange(fn: ConsentListener): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}
