// Enterprise analytics API (Phase F, EDR-0028): метрики использования.
// Все эндпоинты требуют admin + право analytics.read.

import { get } from './client'
import type { CostReport, ManufacturingReport, ProjectReport, UsageReport } from './types'

export interface UsageQuery {
  from?: string
  to?: string
  granularity?: 'day' | 'week' | 'month'
}

export interface RangeQuery {
  from?: string
  to?: string
}

function qs(params: object): string {
  const url = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v) url.set(k, String(v))
  }
  const s = url.toString()
  return s ? `?${s}` : ''
}

export const analyticsApi = {
  usage: (query: UsageQuery = {}) =>
    get<UsageReport>(`/api/v1/admin/analytics/usage${qs(query)}`),

  projects: (query: RangeQuery = {}) =>
    get<ProjectReport>(`/api/v1/admin/analytics/projects${qs(query)}`),

  manufacturing: (query: UsageQuery = {}) =>
    get<ManufacturingReport>(`/api/v1/admin/analytics/manufacturing${qs(query)}`),

  cost: (query: UsageQuery = {}) =>
    get<CostReport>(`/api/v1/admin/analytics/cost${qs(query)}`),
}
