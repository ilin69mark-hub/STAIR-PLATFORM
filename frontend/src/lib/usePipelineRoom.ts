// usePipelineRoom (S-132b): live-уведомления конвейера для ProjectDetail.
// Подписывается на pipeline:<configurationId> (протокол S-132a) и копит
// последние события (завершение конвейера/анализа, готовность документов).

import { useEffect, useState } from 'react'

import type { RealtimeEvent } from '@shared/api/realtime'

import { onRealtimeStatus, subscribeRoom } from './realtimeClient'

export type PipelineNoticeKind =
  | 'pipeline_status'
  | 'analysis_progress'
  | 'document_generated'
  | 'notification'

export interface PipelineNotice {
  kind: PipelineNoticeKind | string
  title: string
  detail?: string
  at: number
}

const MAX_NOTICES = 5

function payloadRecord(payload: unknown): Record<string, unknown> {
  if (typeof payload === 'object' && payload !== null) {
    return payload as Record<string, unknown>
  }
  return {}
}

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}

function num(v: unknown): number | null {
  return typeof v === 'number' ? v : null
}

// toNotice маппит серверное событие в человекочитаемое уведомление.
// Неизвестные типы не отбрасываем (сервер вправе добавить новые) —
// показываем сырой type как title.
export function toNotice(e: RealtimeEvent): PipelineNotice {
  const p = payloadRecord(e.payload)
  const at = Date.now()
  switch (e.type) {
    case 'pipeline_status': {
      const status = str(p['status'])
      if (status === 'completed') {
        return { kind: e.type, title: 'Конвейер завершён', at }
      }
      if (status === 'failed') {
        const err = str(p['error'])
        return {
          kind: e.type,
          title: 'Конвейер: ошибка',
          detail: err || undefined,
          at,
        }
      }
      const stage = str(p['stage'])
      const progress = num(p['progress'])
      return {
        kind: e.type,
        title: `Конвейер: ${stage || status || 'в работе'}`,
        detail: progress !== null ? `${progress}%` : undefined,
        at,
      }
    }
    case 'analysis_progress': {
      const stage = str(p['stage'])
      const progress = num(p['progress'])
      return {
        kind: e.type,
        title: `Анализ: ${stage || 'в работе'}`,
        detail: progress !== null ? `${progress}%` : undefined,
        at,
      }
    }
    case 'document_generated': {
      const t = str(p['type'])
      const format = str(p['format'])
      return {
        kind: e.type,
        title: 'Документ готов',
        detail: [t, format].filter(Boolean).join(' · ') || undefined,
        at,
      }
    }
    default:
      return { kind: e.type, title: `Событие: ${e.type}`, at }
  }
}

export interface PipelineRoomState {
  notices: PipelineNotice[]
  // live — сокет открыт (события доходят); иначе показываем «offline»
  // вместо тишины (иначе непонятно, почему нет уведомлений).
  live: boolean
}

export function usePipelineRoom(configurationId: string | null): PipelineRoomState {
  const [notices, setNotices] = useState<PipelineNotice[]>([])
  const [live, setLive] = useState(false)

  useEffect(() => {
    if (configurationId === null || configurationId === '') {
      setNotices([])
      return
    }
    const room = `pipeline:${configurationId}`
    const off = subscribeRoom(room, (e) => {
      setNotices((prev) => [toNotice(e), ...prev].slice(0, MAX_NOTICES))
    })
    return off
  }, [configurationId])

  useEffect(() => onRealtimeStatus((s) => setLive(s === 'open')), [])

  return { notices, live }
}
