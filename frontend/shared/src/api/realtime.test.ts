// Тесты realtime-клиента (S-132b): фейковый сокет вместо WebSocket.

import { describe, expect, it, vi } from 'vitest'

import {
  createRealtimeClient,
  type RealtimeEvent,
  type WebSocketLike,
} from './realtime'

const ROOM = 'pipeline:123e4567-e89b-12d3-a456-426614174000'

class FakeSocket implements WebSocketLike {
  sent: string[] = []
  onopen: ((ev: unknown) => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onclose: ((ev: unknown) => void) | null = null
  onerror: ((ev: unknown) => void) | null = null
  closed = false

  send(data: string): void {
    this.sent.push(data)
  }

  close(): void {
    if (this.closed) return
    this.closed = true
    queueMicrotask(() => this.onclose?.({}))
  }

  // Хелперы теста: серверная сторона.
  open(): void {
    this.onopen?.({})
  }

  receive(msg: unknown): void {
    this.onmessage?.({ data: JSON.stringify(msg) })
  }

  receiveRaw(data: string): void {
    this.onmessage?.({ data })
  }

  drop(): void {
    this.onclose?.({})
  }
}

function setup(opts?: { ackTimeoutMs?: number; reconnectBaseMs?: number }) {
  const sockets: FakeSocket[] = []
  const events: RealtimeEvent[] = []
  const statuses: string[] = []
  const client = createRealtimeClient({
    buildUrl: () => 'ws://localhost:5173/ws/admin',
    onEvent: (e) => events.push(e),
    onStatus: (s) => statuses.push(s),
    createSocket: () => {
      const s = new FakeSocket()
      sockets.push(s)
      return s
    },
    pingIntervalMs: 20,
    ackTimeoutMs: opts?.ackTimeoutMs ?? 200,
    reconnectBaseMs: opts?.reconnectBaseMs ?? 5,
    reconnectMaxMs: 20,
  })
  return { client, sockets, events, statuses }
}

// connectFirst — детерминированное открытие: сначала connect (комнат нет —
// никаких resub-гонок, см. S-132b), затем subscribe.
async function connectFirst(
  client: ReturnType<typeof createRealtimeClient>,
  sockets: FakeSocket[],
): Promise<void> {
  client.connect()
  sockets[0].open()
  await vi.waitFor(() => expect(client.getStatus()).toBe('open'))
}

describe('realtime client', () => {
  it('строит URL приложения и открывается', () => {
    const { client, sockets, statuses } = setup()
    client.connect()
    expect(sockets).toHaveLength(1)
    expect(statuses).toEqual(['connecting'])
    sockets[0].open()
    expect(statuses).toEqual(['connecting', 'open'])
    expect(client.getStatus()).toBe('open')
    client.disconnect()
  })

  it('subscribe отправляет комнату и резолвится по ack', async () => {
    const { client, sockets } = setup()
    await connectFirst(client, sockets)
    const p = client.subscribe(ROOM)
    let sub: Record<string, unknown> | undefined
    await vi.waitFor(() => {
      sub = sockets[0].sent
        .map((s) => JSON.parse(s) as Record<string, unknown>)
        .find((m) => m['type'] === 'subscribe')
      expect(sub).toBeDefined()
    })
    expect(sub).toMatchObject({ type: 'subscribe', payload: { room: ROOM } })
    sockets[0].receive({ type: 'subscribed', payload: { room: ROOM } })
    await expect(p).resolves.toBeUndefined()
    expect(client.isSubscribed(ROOM)).toBe(true)
    client.disconnect()
  })

  it('subscribe без ack — ошибка timeout, комната остаётся', async () => {
    const { client, sockets } = setup({ ackTimeoutMs: 30 })
    await connectFirst(client, sockets)
    const p = client.subscribe(ROOM)
    await expect(p).rejects.toThrow(/ack timeout/)
    // комнату не снимаем — сервер мог подписать (ack потерян): следующий
    // реконнект переподпишет, дубликаты сервер схлопывает в Set.
    expect(client.isSubscribed(ROOM)).toBe(true)
    client.disconnect()
  })

  it('серверные события уходят в onEvent, pong и мусор — игнор', async () => {
    const { client, sockets, events } = setup()
    client.connect()
    sockets[0].open()
    sockets[0].receive({
      type: 'pipeline_status',
      payload: { configId: 'abc', status: 'completed' },
      time: '2026-09-22T00:00:00Z',
    })
    sockets[0].receive({ type: 'pong', payload: {} })
    sockets[0].receiveRaw('not-json{{{')
    sockets[0].receive({ payload: {} })
    await vi.waitFor(() => expect(events).toHaveLength(1))
    expect(events[0]).toMatchObject({
      type: 'pipeline_status',
      payload: { configId: 'abc', status: 'completed' },
    })
    client.disconnect()
  })

  it('разрыв → реконнект с переподпиской комнат', async () => {
    const { client, sockets } = setup({ reconnectBaseMs: 5 })
    await connectFirst(client, sockets)
    const p = client.subscribe(ROOM)
    await vi.waitFor(() => {
      const found = sockets[0].sent
        .map((s) => JSON.parse(s) as { type: string })
        .some((m) => m.type === 'subscribe')
      expect(found).toBe(true)
    })
    sockets[0].receive({ type: 'subscribed', payload: { room: ROOM } })
    await p
    sockets[0].drop()
    await vi.waitFor(() => expect(sockets.length).toBe(2), { timeout: 1000 })
    sockets[1].open()
    // Переподписка — синхронно в onOpen: первый send нового сокета.
    expect(sockets[1].sent).toHaveLength(1)
    const first = JSON.parse(sockets[1].sent[0]) as { type: string }
    expect(first.type).toBe('subscribe')
    expect(client.subscribedRooms()).toEqual([ROOM])
    client.disconnect()
  })

  it('ping шлётся по интервалу', async () => {
    const { client, sockets } = setup()
    client.connect()
    sockets[0].open()
    await vi.waitFor(
      () => {
        const types = sockets[0].sent.map((s) => (JSON.parse(s) as { type: string }).type)
        expect(types).toContain('ping')
      },
      { timeout: 1000 },
    )
    client.disconnect()
  })

  it('unsubscribe отписывается и ждёт ack; без соединения — no-op', async () => {
    const { client, sockets } = setup()
    await client.unsubscribe(ROOM) // не подключены — тихо
    await connectFirst(client, sockets)
    const p = client.subscribe(ROOM)
    await vi.waitFor(() => {
      const found = sockets[0].sent
        .map((s) => JSON.parse(s) as { type: string })
        .some((m) => m.type === 'subscribe')
      expect(found).toBe(true)
    })
    sockets[0].receive({ type: 'subscribed', payload: { room: ROOM } })
    await p
    const up = client.unsubscribe(ROOM)
    await vi.waitFor(() => {
      const types = sockets[0].sent.map((s) => (JSON.parse(s) as { type: string }).type)
      expect(types).toContain('unsubscribe')
    })
    sockets[0].receive({ type: 'unsubscribed', payload: { room: ROOM } })
    await expect(up).resolves.toBeUndefined()
    expect(client.isSubscribed(ROOM)).toBe(false)
    client.disconnect()
  })

  it('disconnect останавливает реконнекты', async () => {
    const { client, sockets } = setup({ reconnectBaseMs: 5 })
    client.connect()
    sockets[0].open()
    client.disconnect()
    expect(client.getStatus()).toBe('closed')
    await new Promise((r) => setTimeout(r, 60))
    expect(sockets).toHaveLength(1)
  })
})
