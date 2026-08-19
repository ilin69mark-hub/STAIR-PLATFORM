import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuditPanel } from './AuditPanel'
import { auditApi } from '../api/audit'
import { ApiError, type AuditEvent } from '@shared/types'

const makeEvent = (overrides: Partial<AuditEvent> = {}): AuditEvent => ({
  id: 'e-1',
  tenant_id: 't-1',
  project_id: 'p1',
  actor_id: 'u-1',
  action: 'project.created',
  result: 'ok',
  detail: 'Проект создан',
  ip: '127.0.0.1',
  created_at: '2026-08-15T10:00:00Z',
  ...overrides,
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('AuditPanel', () => {
  it('показывает события аудита проекта', async () => {
    vi.spyOn(auditApi, 'listProjectAudit').mockResolvedValue([
      makeEvent(),
      makeEvent({
        id: 'e-2',
        action: 'authz.denied',
        result: 'denied',
        detail: 'Права отсутствуют',
      }),
    ])
    render(<AuditPanel projectId="p1" />)
    expect(await screen.findByText('Создание проекта')).toBeInTheDocument()
    expect(screen.getByText('Отказ в доступе · denied')).toBeInTheDocument()
    expect(screen.getByText('Права отсутствуют')).toBeInTheDocument()
  })

  it('показывает пустое состояние', async () => {
    vi.spyOn(auditApi, 'listProjectAudit').mockResolvedValue([])
    render(<AuditPanel projectId="p1" />)
    expect(await screen.findByText('Событий аудита пока нет.')).toBeInTheDocument()
  })

  it('показывает ошибку API', async () => {
    vi.spyOn(auditApi, 'listProjectAudit').mockRejectedValue(
      new ApiError(403, 'forbidden', 'Нет доступа к аудиту'),
    )
    render(<AuditPanel projectId="p1" />)
    expect(await screen.findByText('Нет доступа к аудиту')).toBeInTheDocument()
  })
})
