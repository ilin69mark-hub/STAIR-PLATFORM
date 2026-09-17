import { describe, expect, it, vi, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { AuditPage } from './AuditPage'
import * as auditApi from '../api/audit'

describe('AuditPage', () => {
  afterEach(() => vi.restoreAllMocks())

  it('loading shows skeletons', async () => {
    vi.spyOn(auditApi.auditApi, 'listTenantAudit').mockReturnValue(new Promise(() => {}))
    render(<AuditPage onBack={vi.fn()} />)
    expect(document.querySelectorAll('.skeleton').length).toBe(3)
  })

  it('empty shows message', async () => {
    vi.spyOn(auditApi.auditApi, 'listTenantAudit').mockResolvedValue([])
    render(<AuditPage onBack={vi.fn()} />)
    expect(await screen.findByText('Событий аудита пока нет.')).toBeInTheDocument()
  })

  it('renders events and back', async () => {
    vi.spyOn(auditApi.auditApi, 'listTenantAudit').mockResolvedValue([
      { id: '1', action: 'auth.login', result: 'ok', created_at: '2026-01-01T10:00:00Z', actor_id: 'u1', project_id: 'p1', ip: '127.0.0.1' } as any,
      { id: '2', action: 'unknown.action', result: 'failed', created_at: '2026-01-02T10:00:00Z', detail: 'bad' } as any,
    ])
    const onBack = vi.fn()
    render(<AuditPage onBack={onBack} />)
    expect(await screen.findByText('Аудит действий')).toBeInTheDocument()
    // eventually events render
    expect(await screen.findByText(/u1/)).toBeInTheDocument()
    fireEvent.click(screen.getByText('Назад'))
    expect(onBack).toHaveBeenCalled()
  })

  it('error shows alert', async () => {
    vi.spyOn(auditApi.auditApi, 'listTenantAudit').mockRejectedValue(new Error('boom'))
    render(<AuditPage onBack={vi.fn()} />)
    expect(await screen.findByText('Не удалось загрузить аудит')).toBeInTheDocument()
  })
})
