// Enterprise analytics API (Phase F, EDR-0028): метрики использования.
// Все эндпоинты требуют admin + право analytics.read.

import { get } from './client'
import type { UsageReport } from './types'

export interface UsageQuery {
  from?: string
  to?: string
  granularity?: 'day' | 'week' | 'month'
}

export const analyticsApi = {
  usage: (query: UsageQuery = {}) => {
    const params = new URLSearchParams()
    if (query.from) params.set('from', query.from)
    if (query.to) params.set('to', query.to)
    if (query.granularity) params.set('granularity', query.granularity)
    const qs = params.toString()
    return get<UsageReport>(`/api/v1/admin/analytics/usage${qs ? `?${qs}` : ''}`)
  },
}
