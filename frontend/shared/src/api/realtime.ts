// Realtime WS-клиент (S-132b): framework-free обёртка над WebSocket для
// подписок на pipeline-комнаты бэкенда (протокол S-132a).
//
// Сервер (internal/transport/websocket): client→server {type:'subscribe'/
// 'unsubscribe', payload:{room}} + ack {type:'subscribed'/'unsubscribed'};
// server→client {type:'pipeline_status'/'analysis_progress'/
// 'document_generated'/'notification', payload:{configId,...}}.
// Комнаты — только `pipeline:<uuid>` (валидирует сервер).
// Аутентификация — сессионная cookie браузера (S-110: никакого ?token=).

// ServerEventType — типы событий сервер→клиент.
export type ServerEventType =
  | 'pipeline_status'
  | 'analysis_progress'
  | 'document_generated'
  | 'notification'

// RealtimeEvent — событие от сервера (payload уже распарсен).
export interface RealtimeEvent {
  type: ServerEventType | string
  payload: unknown
  time?: string
}

export type ConnectionStatus = 'connecting' | 'open' | 'closed'

// WebSocketLike — минимальный срез WebSocket для тестового seam
// (в проде передаётся глобальный WebSocket).
export interface WebSocketLike {
  send: (data: string) => void
  close: () => void
  onopen: ((ev: unknown) => void) | null
  onmessage: ((ev: { data: string }) => void) | null
  onclose: ((ev: unknown) => void) | null
  onerror: ((ev: unknown) => void) | null
}

export interface RealtimeClientOptions {
  // buildUrl строит ws(s)-URL (приложение: same-origin + /ws или /ws/admin).
  buildUrl: () => string
  onEvent: (e: RealtimeEvent) => void
  onStatus?: (s: ConnectionStatus) => void
  createSocket?: (url: string) => WebSocketLike
  pingIntervalMs?: number
  ackTimeoutMs?: number
  reconnectBaseMs?: number
  reconnectMaxMs?: number
}

interface PendingAck {
  resolve: () => void
  reject: (e: Error) => void
  timer: ReturnType<typeof setTimeout>
}

const PING = 'ping'

// createRealtimeClient — клиент с автопереподключением, ping-keepalive и
// ожиданием ack на subscribe/unsubscribe. Подписки (rooms) переживают
// реконнект: после open все комнаты переподписываются автоматически.
export function createRealtimeClient(opts: RealtimeClientOptions) {
  const pingIntervalMs = opts.pingIntervalMs ?? 30000
  const ackTimeoutMs = opts.ackTimeoutMs ?? 5000
  const reconnectBaseMs = opts.reconnectBaseMs ?? 1000
  const reconnectMaxMs = opts.reconnectMaxMs ?? 30000
  const createSocket =
    opts.createSocket ??
    ((url: string) => new WebSocket(url) as unknown as WebSocketLike)

  let socket: WebSocketLike | null = null
  let status: ConnectionStatus = 'closed'
  let explicitClose = false
  let reconnectAttempt = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let pingTimer: ReturnType<typeof setInterval> | null = null
  const rooms = new Set<string>()
  // ack-ожидания: ключ `${ackType}:${room}` (subscribed/unsubscribed).
  const pending = new Map<string, PendingAck[]>()

  function setStatus(s: ConnectionStatus): void {
    status = s
    opts.onStatus?.(s)
  }

  function clearTimers(): void {
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (pingTimer !== null) {
      clearInterval(pingTimer)
      pingTimer = null
    }
  }

  function scheduleReconnect(): void {
    if (explicitClose || reconnectTimer !== null) return
    const delay = Math.min(
      reconnectBaseMs * 2 ** reconnectAttempt + Math.random() * 250,
      reconnectMaxMs,
    )
    reconnectAttempt += 1
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, delay)
  }

  function sendJson(msg: unknown): void {
    socket?.send(JSON.stringify(msg))
  }

  function awaitAck(ackType: string, room: string): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      const key = `${ackType}:${room}`
      // entry захватывается таймером, но таймер срабатывает позже
      // присваивания — безопасно.
      const entry: PendingAck = {
        resolve,
        reject,
        timer: setTimeout(() => {
          // Снимаем только своё ожидание; остальные (повторные subscribe)
          // продолжают ждать свой ack.
          const list = pending.get(key) ?? []
          pending.set(
            key,
            list.filter((p) => p !== entry),
          )
          reject(new Error(`realtime: ack timeout (${ackType} ${room})`))
        }, ackTimeoutMs),
      }
      const list = pending.get(key) ?? []
      list.push(entry)
      pending.set(key, list)
    })
  }

  function settlePending(ackType: string, room: string): void {
    const key = `${ackType}:${room}`
    const list = pending.get(key) ?? []
    pending.delete(key)
    for (const p of list) {
      clearTimeout(p.timer)
      p.resolve()
    }
  }

  function failAllPending(err: Error): void {
    for (const [, list] of pending) {
      for (const p of list) {
        clearTimeout(p.timer)
        p.reject(err)
      }
    }
    pending.clear()
  }

  function onOpen(): void {
    reconnectAttempt = 0
    setStatus('open')
    // Переподписка на все комнаты (S-132a: сервер забывает комнаты
    // отвалившегося клиента — unregister чистит rooms).
    for (const room of rooms) {
      sendJson({ type: 'subscribe', payload: { room } })
    }
    pingTimer = setInterval(() => {
      sendJson({ type: PING, payload: {} })
    }, pingIntervalMs)
  }

  function onClose(): void {
    if (pingTimer !== null) {
      clearInterval(pingTimer)
      pingTimer = null
    }
    socket = null
    failAllPending(new Error('realtime: connection closed'))
    if (explicitClose) {
      setStatus('closed')
      return
    }
    setStatus('closed')
    scheduleReconnect()
  }

  function onMessage(ev: { data: string }): void {
    let msg: { type?: string; payload?: unknown; time?: string }
    try {
      msg = JSON.parse(ev.data) as typeof msg
    } catch {
      return // мусор игнорируем
    }
    if (typeof msg.type !== 'string') return
    if (msg.type === 'pong') return
    if (msg.type === 'subscribed' || msg.type === 'unsubscribed') {
      const room = (msg.payload as { room?: unknown } | undefined)?.room
      if (typeof room === 'string') settlePending(msg.type, room)
      return
    }
    opts.onEvent({ type: msg.type, payload: msg.payload, time: msg.time })
  }

  function connect(): void {
    if (socket !== null) return
    // Явный connect (в т.ч. после disconnect) снимает флаг — иначе клиент
    // одноразовый. Авто-реконнект при explicitClose не стартует (см.
    // scheduleReconnect/onClose), так что семантика disconnect сохранена.
    explicitClose = false
    setStatus('connecting')
    const ws = createSocket(opts.buildUrl())
    socket = ws
    ws.onopen = () => onOpen()
    ws.onmessage = (ev) => onMessage(ev)
    ws.onerror = () => {
      // ошибку доводим до close-потока (браузер сам закроет сокет)
      try {
        ws.close()
      } catch {
        // ignore
      }
    }
    ws.onclose = () => {
      if (socket === ws) onClose()
    }
  }

  function disconnect(): void {
    explicitClose = true
    clearTimers()
    failAllPending(new Error('realtime: disconnected'))
    try {
      socket?.close()
    } catch {
      // ignore
    }
    socket = null
    setStatus('closed')
  }

  async function ensureOpen(): Promise<void> {
    if (status === 'open' && socket !== null) return
    connect()
    const started = Date.now()
    while (status !== 'open') {
      if (explicitClose) throw new Error('realtime: disconnected')
      if (Date.now() - started > ackTimeoutMs) {
        throw new Error('realtime: connect timeout')
      }
      await new Promise((r) => setTimeout(r, 25))
    }
  }

  // subscribe добавляет комнату и ждёт ack сервера (S-132a). Невалидную
  // комнату сервер проигнорирует → будет ack-timeout (ошибка наружу).
  async function subscribe(room: string): Promise<void> {
    rooms.add(room)
    await ensureOpen()
    sendJson({ type: 'subscribe', payload: { room } })
    await awaitAck('subscribed', room)
  }

  async function unsubscribe(room: string): Promise<void> {
    rooms.delete(room)
    if (status !== 'open' || socket === null) return
    sendJson({ type: 'unsubscribe', payload: { room } })
    await awaitAck('unsubscribed', room)
  }

  return {
    connect,
    disconnect,
    subscribe,
    unsubscribe,
    getStatus: () => status,
    isSubscribed: (room: string) => rooms.has(room),
    subscribedRooms: () => [...rooms],
  }
}

export type RealtimeClient = ReturnType<typeof createRealtimeClient>
