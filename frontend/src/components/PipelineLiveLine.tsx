// PipelineLiveLine (S-132b): строка live-статуса конвейера в ProjectDetail.
// Показывает точку соединения и последнее событие комнаты
// pipeline:<configurationId>. Пусто (null), когда сокет открыт, а событий
// ещё нет — не шумим в UI до первого события.

import type { PipelineNotice } from '../lib/usePipelineRoom'

interface Props {
  live: boolean
  notices: PipelineNotice[]
}

export function PipelineLiveLine({ live, notices }: Props) {
  const latest = notices[0]
  if (live && !latest) return null
  return (
    <p className="muted" data-testid="pipeline-live">
      <span aria-hidden="true">{live ? '●' : '○'}</span>{' '}
      {live ? 'live' : 'realtime offline'}
      {latest !== undefined && (
        <>
          {' — '}
          {latest.title}
          {latest.detail !== undefined && latest.detail !== '' && (
            <span> · {latest.detail}</span>
          )}
        </>
      )}
    </p>
  )
}
