// Сбор событий витрины («где идёт трафик, где затык, где бросает»).
//
// Правило номер один: без согласия здесь НИЧЕГО не отправляется. Не
// «отправляется, но не сохраняется» — не отправляется вообще, чтобы
// отключить сбор можно было одной переменной, а не правками в каждом
// обработчике. Проверка стоит в track() и в send(), а не в вызывающем коде.
//
// Что собираем (каталог имён повторяет application/funnel.AllEvents —
// сервер отбрасывает то, чего в каталоге нет, и присылать новое имя имеет
// смысл только вместе с правкой каталога):
//   page.view              — открытие экрана;
//   funnel.constructor_open — человек дошёл до конструктора;
//   funnel.step_done        — осмысленный шаг сделан;
//   blocker.field_invalid   — поле не прошло проверку (reason = имя поля);
//   blocker.api_error       — сервер отверг запрос (reason = код);
//   cta.quote_clicked       — нажал «Рассчитать»;
//   cta.order_clicked       — отправил заявку;
//   session.start / session.leave — границы визита.
//
// Отдельно важно session.leave: он уходит на pagehide/visibilitychange через
// sendBeacon и несёт последнее событие визита — именно по нему отчёт
// отвечает на вопрос «где человек бросил».
//
// Данных о человеке здесь нет: ни IP, ни User-Agent (сервер берёт их из
// запроса, если вообще берёт), ни введённых значений полей. В пропсы
// попадают только имена полей и коды, которые выбирает код, а не человек.

import { CONSENT_VERSION, hasConsent, onConsentChange } from './consent'

/** Адрес приёма. Относительный — витрина и API на одном домене (dev: vite
 *  proxy, прод: nginx), и абсолютный URL здесь означал бы лишнюю
 *  настройку окружения. */
const ENDPOINT = '/api/v1/public/analytics:events'

/** Имена событий. Держатся рядом с кодом: сервер сверяется с этим же списком
 *  (application/funnel.AllEvents), и расхождение видно сразу. */
export const EVENTS = {
  pageView: 'page.view',
  constructorOpen: 'funnel.constructor_open',
  stepDone: 'funnel.step_done',
  blockerField: 'blocker.field_invalid',
  blockerApi: 'blocker.api_error',
  ctaQuote: 'cta.quote_clicked',
  ctaOrder: 'cta.order_clicked',
  sessionStart: 'session.start',
  sessionLeave: 'session.leave',
} as const

export type EventName = (typeof EVENTS)[keyof typeof EVENTS]

type Props = Record<string, string | number | boolean>

interface QueuedEvent {
  name: string
  props: Props
  path: string
  ts: string
}

const SESSION_KEY = 'stair-platform-funnel-session'
/** Ограничения дублируют серверные (application/funnel): переполненная
 *  пачка сервер отвергнет целиком, и визит потеряется. */
const MAX_BATCH = 20
const MAX_PROPS = 12
const MAX_VALUE_LEN = 64
const MAX_KEY_LEN = 32
/** Пачка копится в буфере и уходит по таймеру либо по размеру: на каждый
 *  чих слать запрос — это и трафик, и лимит запросов на IP. */
const FLUSH_MS = 4000

let buffer: QueuedEvent[] = []
let timer: ReturnType<typeof setTimeout> | null = null
let sessionId = ''
let lastEvent = ''
let startedAt = 0
let leaveSent = false
let started = false

// ---- сессия ----

function uuid(): string {
  const c = globalThis.crypto
  if (c?.randomUUID) return c.randomUUID()
  // Запасной путь для контекстов без randomUUID: RFC 4122 v4 из
  // Math.random. Идентификатор нужен только чтобы связать события визита,
  // криптостойкость тут не требуется.
  const hex = '0123456789abcdef'
  let out = ''
  for (let i = 0; i < 36; i++) {
    if (i === 8 || i === 13 || i === 18 || i === 23) out += '-'
    else if (i === 14) out += '4'
    else if (i === 19) out += hex[(Math.random() * 4) | (0 + 8)]
    else out += hex[(Math.random() * 16) | 0]
  }
  return out
}

function readSession(): string {
  try {
    const v = sessionStorage.getItem(SESSION_KEY)
    if (v) return v
  } catch {
    // приватный режим: сессия живёт только в памяти вкладки
  }
  return ''
}

function writeSession(id: string) {
  try {
    sessionStorage.setItem(SESSION_KEY, id)
  } catch {
    // см. readSession
  }
}

/** Заметка для отчёта: последнее осмысленное событие визита. Оно и есть
 *  ответ на «на чём человек бросил». */
function remember(name: string) {
  lastEvent = name
}

function currentPath(): string {
  const p = globalThis.location?.pathname ?? '/'
  return (p + (globalThis.location?.hash ?? '')).slice(0, 128)
}

// ---- очистка пропсов ----

function cleanProps(props?: Props): Props | undefined {
  if (!props) return undefined
  const keys = Object.keys(props).slice(0, MAX_PROPS)
  if (keys.length === 0) return undefined
  const out: Props = {}
  for (const k of keys) {
    const key = k.slice(0, MAX_KEY_LEN)
    const v = props[k]
    if (typeof v === 'number') out[key] = v
    else if (typeof v === 'boolean') out[key] = v
    else out[key] = String(v).slice(0, MAX_VALUE_LEN)
  }
  return out
}

// ---- отправка ----

function schedule() {
  if (timer != null) return
  timer = setTimeout(() => {
    timer = null
    void flush()
  }, FLUSH_MS)
}

/** Обычная отправка пачки. Ошибки молча проглатываются: телеметрия не имеет
 *  права ломать интерфейс, и посетитель не должен видеть её поломок. */
export async function flush(): Promise<void> {
  if (timer != null) {
    clearTimeout(timer)
    timer = null
  }
  if (!hasConsent() || buffer.length === 0) {
    buffer = []
    return
  }
  const events = buffer
  buffer = []
  const body = {
    session_id: sessionId,
    consent_version: CONSENT_VERSION,
    events: events.slice(0, MAX_BATCH),
  }
  try {
    await fetch(ENDPOINT, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      keepalive: true,
      credentials: 'omit',
    })
  } catch {
    // потерянные события — не повод для ошибки у пользователя
  }
}

/** Отправка на уход со страницы. sendBeacon — единственный способ, который
 *  переживает закрытие вкладки, но он не умеет ставить заголовки, а сервер
 *  принимает тело только с Content-Type: application/json. Поэтому Blob с
 *  типом: тип в Blob и есть Content-Type, который уходит вместе с телом. */
function sendBeacon(events: QueuedEvent[]) {
  if (!hasConsent() || events.length === 0 || !sessionId) return
  const payload = {
    session_id: sessionId,
    consent_version: CONSENT_VERSION,
    events: events.slice(0, MAX_BATCH),
  }
  try {
    const blob = new Blob([JSON.stringify(payload)], { type: 'application/json' })
    globalThis.navigator?.sendBeacon?.(ENDPOINT, blob)
  } catch {
    // нет Blob или нет navigator.sendBeacon — событие просто потеряется
  }
}

// ---- публичный API ----

/** Отправляет событие. Молча ничего не делает без согласия. */
export function track(name: EventName | string, props?: Props): void {
  if (!hasConsent()) return
  if (!started) start()
  remember(name)
  const ev: QueuedEvent = {
    name: String(name).slice(0, 64),
    props: cleanProps(props) ?? {},
    path: currentPath(),
    ts: new Date().toISOString(),
  }
  buffer.push(ev)
  if (buffer.length >= MAX_BATCH) void flush()
  else schedule()
}

/** Запуск сбора: session.start + подписки на уход и на изменение согласия.
 *  Вызывается один раз из корня приложения. */
export function start(): void {
  if (started) return
  started = true
  startedAt = Date.now()
  sessionId = readSession() || uuid()
  if (!sessionId) return
  writeSession(sessionId)

  track(EVENTS.sessionStart, { referrer_kind: referrerKind() })

  const onHide = () => {
    if (leaveSent) return
    leaveSent = true
    const seconds = Math.round((Date.now() - startedAt) / 1000)
    const ev: QueuedEvent = {
      name: EVENTS.sessionLeave,
      props: {
        last: lastEvent || EVENTS.sessionStart,
        session_seconds: seconds,
        browser: navigatorLanguage(),
      },
      path: currentPath(),
      ts: new Date().toISOString(),
    }
    // На уход уходит всё, что накопилось, вместе с session.leave: буфер,
    // иначе последние шаги визита терялись бы именно там, где они ценнее
    // всего.
    sendBeacon([...buffer, ev])
    buffer = []
  }

  // pagehide надёжнее unload (unload не срабатывает в мобильных браузерах и
  // при bfcache), visibilitychange — страховка: страница могла уйти в фон.
  globalThis.addEventListener?.('pagehide', onHide)
  globalThis.document?.addEventListener?.('visibilitychange', () => {
    if (globalThis.document?.visibilityState === 'hidden') onHide()
  })

  onConsentChange((granted) => {
    if (granted && !leaveSent) {
      // Согласие только что дали в этой же вкладке: визит начался ДО него, и
      // без session.start отчёт не увидит начала сессии.
      //
      // Что было до согласия, не отправляется НИКОГДА — иначе «мы не собираем
      // ничего без согласия» было бы ложью. Поэтому верх воронки у
      // посетителя, согласившегося уже внутри конструктора, добирается
      // отдельными событиями оттуда, где стоит useConsentGranted.
      track(EVENTS.sessionStart, { referrer_kind: referrerKind() })
    }
  })
}

function referrerKind(): string {
  const r = globalThis.document?.referrer ?? ''
  if (!r) return 'direct'
  try {
    const host = new URL(r).hostname
    if (host === globalThis.location?.hostname) return 'internal'
    return 'external'
  } catch {
    return 'other'
  }
}

function navigatorLanguage(): string {
  const l = globalThis.navigator?.language ?? ''
  return l.slice(0, 2)
}
