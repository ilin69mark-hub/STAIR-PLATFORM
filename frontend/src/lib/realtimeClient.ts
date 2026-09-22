// Realtime-синглтон админки (S-132b): один WS на /ws/admin (S-116,
// cookie session_admin — браузер шлёт сам, S-110), fan-out событий по
// комнатам pipeline:<configId> (configId из payload, S-132a).

import {
  createRealtimeClient,
  type ConnectionStatus,
  type RealtimeClient,
  type RealtimeEvent,
} from '@shared/api/realtime'

// wsUrl — same-origin WS-URL (в dev — через vite-proxy /ws, в проде —
// через nginx location /ws; S-121).
function wsUrl(path: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}${path}`
}

export type RoomHandler = (e: RealtimeEvent) => void

const roomHandlers = new Map<string, Set<RoomHandler>>()
const statusHandlers = new Set<(s: ConnectionStatus) => void>()

function configIdOf(payload: unknown): string | null {
  if (typeof payload === 'object' && payload !== null && 'configId' in payload) {
    const v = (payload as { configId: unknown }).configId
    return typeof v === 'string' && v !== '' ? v : null
  }
  return null
}

let client: RealtimeClient | null = null

export function getRealtimeClient(): RealtimeClient {
  if (!client) {
    client = createRealtimeClient({
      buildUrl: () => wsUrl('/ws/admin'),
      onEvent: (e) => {
        const id = configIdOf(e.payload)
        if (id === null) return
        const handlers = roomHandlers.get(`pipeline:${id}`)
        if (!handlers) return
        for (const h of [...handlers]) {
          try {
            h(e)
          } catch {
            // один сбойный обработчик не роняет остальных (как session.ts)
          }
        }
      },
      onStatus: (s) => {
        for (const h of [...statusHandlers]) {
          try {
            h(s)
          } catch {
            // ignore
          }
        }
      },
    })
  }
  return client
}

// onRealtimeStatus — подписка на статус соединения (немедленно отдаёт
// текущий — без мигания «offline» при монтировании).
export function onRealtimeStatus(handler: (s: ConnectionStatus) => void): () => void {
  statusHandlers.add(handler)
  handler(getRealtimeClient().getStatus())
  return () => {
    statusHandlers.delete(handler)
  }
}

// subscribeRoom — подписка обработчика на комнату + ленивый connect.
// Возвращает отписку. Ошибка subscribe (невалидная комната/таймаут) —
// чистим только если обработчиков комнаты не осталось (комнату могут
// делить несколько хуков).
// Без WebSocket в окружении (SSR, jsdom без мока) — тихий no-op: UI
// остаётся синхронным, realtime просто недоступен.
export function subscribeRoom(room: string, handler: RoomHandler): () => void {
  if (typeof WebSocket === 'undefined') return () => {}
  let set = roomHandlers.get(room)
  if (!set) {
    set = new Set()
    roomHandlers.set(room, set)
  }
  set.add(handler)
  const c = getRealtimeClient()
  c.connect()
  void c.subscribe(room).catch(() => {
    if ((roomHandlers.get(room)?.size ?? 0) === 0) {
      void c.unsubscribe(room).catch(() => {})
    }
  })
  return () => {
    const s = roomHandlers.get(room)
    s?.delete(handler)
    if (s !== undefined && s.size === 0) {
      roomHandlers.delete(room)
      void getRealtimeClient()
        .unsubscribe(room)
        .catch(() => {})
    }
  }
}

// __resetRealtimeForTests — только для тестов (синглтон иначе переживает
// тест-кейсы).
export function __resetRealtimeForTests(): void {
  try {
    client?.disconnect()
  } catch {
    // ignore
  }
  client = null
  roomHandlers.clear()
  statusHandlers.clear()
}
