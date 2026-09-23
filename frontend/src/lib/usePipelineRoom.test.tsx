// Тесты usePipelineRoom (S-132b): realtimeClient замокан, проверяем
// подписку/маппинг/очистку хука.

import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { RealtimeEvent } from '@shared/api/realtime'

import { toNotice, usePipelineRoom } from './usePipelineRoom'

const { handlers, statusHandlers, subscribeMock, unsubscribeMock } = vi.hoisted(() => ({
  handlers: new Map<string, Set<(e: RealtimeEvent) => void>>(),
  statusHandlers: new Set<(s: string) => void>(),
  subscribeMock: vi.fn(),
  unsubscribeMock: vi.fn(),
}))

vi.mock('./realtimeClient', () => ({
  subscribeRoom: (room: string, h: (e: RealtimeEvent) => void) => {
    subscribeMock(room)
    let set = handlers.get(room)
    if (!set) {
      set = new Set()
      handlers.set(room, set)
    }
    set.add(h)
    return () => {
      unsubscribeMock(room)
      handlers.get(room)?.delete(h)
    }
  },
  onRealtimeStatus: (h: (s: string) => void) => {
    statusHandlers.add(h)
    h('closed')
    return () => {
      statusHandlers.delete(h)
    }
  },
}))

function emit(room: string, e: RealtimeEvent): void {
  for (const h of handlers.get(room) ?? []) h(e)
}

function setStatus(s: string): void {
  for (const h of [...statusHandlers]) h(s)
}

const CONFIG_ID = '123e4567-e89b-12d3-a456-426614174000'

describe('toNotice', () => {
  it('маппит известные события', () => {
    expect(
      toNotice({ type: 'pipeline_status', payload: { status: 'completed' } }),
    ).toMatchObject({ title: 'Конвейер завершён' })
    expect(
      toNotice({
        type: 'pipeline_status',
        payload: { status: 'failed', error: 'boom' },
      }),
    ).toMatchObject({ title: 'Конвейер: ошибка', detail: 'boom' })
    expect(
      toNotice({
        type: 'analysis_progress',
        payload: { stage: 'geometry', progress: 42 },
      }),
    ).toMatchObject({ title: 'Анализ: geometry', detail: '42%' })
    expect(
      toNotice({
        type: 'document_generated',
        payload: { type: 'spec', format: 'pdf' },
      }),
    ).toMatchObject({ title: 'Документ готов', detail: 'spec · pdf' })
  })

  it('неизвестный тип показывает сырым', () => {
    expect(toNotice({ type: 'something_new', payload: {} })).toMatchObject({
      title: 'Событие: something_new',
    })
  })
})

describe('usePipelineRoom', () => {
  beforeEach(() => {
    handlers.clear()
    statusHandlers.clear()
    subscribeMock.mockClear()
    unsubscribeMock.mockClear()
  })

  it('подписывается на pipeline:<id> и копит уведомления', () => {
    const { result } = renderHook(() => usePipelineRoom(CONFIG_ID))
    // мок subscribeRoom(room, handler) форвардит в subscribeMock только room;
    // наличие handler проверяем косвенно — emit доходит до notices.
    expect(subscribeMock).toHaveBeenCalledWith(`pipeline:${CONFIG_ID}`)
    expect(result.current.live).toBe(false)

    act(() => {
      setStatus('open')
      emit(`pipeline:${CONFIG_ID}`, {
        type: 'document_generated',
        payload: { type: 'spec', format: 'pdf' },
      })
    })
    expect(result.current.live).toBe(true)
    expect(result.current.notices).toHaveLength(1)
    expect(result.current.notices[0]).toMatchObject({ title: 'Документ готов' })
  })

  it('отписывается при размонтировании и чистит при смене id', () => {
    const { result, rerender, unmount } = renderHook(
      ({ id }: { id: string | null }) => usePipelineRoom(id),
      { initialProps: { id: CONFIG_ID as string | null } },
    )
    expect(result.current.notices).toEqual([])
    rerender({ id: null })
    expect(unsubscribeMock).toHaveBeenCalledWith(`pipeline:${CONFIG_ID}`)
    expect(result.current.notices).toEqual([])

    rerender({ id: 'other-id' })
    expect(subscribeMock).toHaveBeenCalledWith('pipeline:other-id')
    unmount()
    expect(unsubscribeMock).toHaveBeenCalledWith('pipeline:other-id')
  })
})
