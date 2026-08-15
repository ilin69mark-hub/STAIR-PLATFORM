// API аудита (Phase G, EDR-0013): журнал событий безопасности.

import { get } from './client'
import type { AuditEvent } from './types'

export const auditApi = {
  listProjectAudit: (projectId: string) => get<AuditEvent[]>(`/api/v1/projects/${projectId}/audit`),

  listTenantAudit: () => get<AuditEvent[]>('/api/v1/audit'),
}
