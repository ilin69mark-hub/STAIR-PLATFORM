import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AdminPanel } from './AdminPanel'
import { adminApi } from '../api/admin'
import { ApiError, type AdminOverview, type AdminPolicy, type AdminUser, type ApiKey } from '../api/types'

const overview: AdminOverview = {
  tenant_id: 't-1',
  users: 2,
  active_users: 2,
  disabled_users: 0,
  admins: 1,
  projects: 3,
  active_api_keys: 1,
  total_api_keys: 1,
}

const users: AdminUser[] = [
  { id: 'u-admin', email: 'admin@example.com', name: 'Админ', role: 'admin', tenant_id: 't-1', status: 'active' },
  { id: 'u-2', email: 'user@example.com', name: 'Пользователь', role: 'user', tenant_id: 't-1', status: 'active' },
]

const policy: AdminPolicy = {
  min_password_length: 8,
  require_number: false,
  require_upper: false,
  session_ttl_seconds: 86400,
  login_rate_limit_per_min: 10,
}

const keys: ApiKey[] = [
  { id: 'key-1', name: 'CI', scopes: ['users.list'], created_at: '2026-08-15T10:00:00Z' },
]

function mockApi() {
  vi.spyOn(adminApi, 'overview').mockResolvedValue(overview)
  vi.spyOn(adminApi, 'listUsers').mockResolvedValue(users)
  vi.spyOn(adminApi, 'getSettings').mockResolvedValue(policy)
  vi.spyOn(adminApi, 'listApiKeys').mockResolvedValue(keys)
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('AdminPanel', () => {
  it('показывает обзор, пользователей, политику и API-ключи', async () => {
    mockApi()
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    expect(await screen.findByText('Администрирование')).toBeInTheDocument()
    expect(screen.getByText('Политика безопасности')).toBeInTheDocument()
    expect(screen.getByText('Экспорт данных')).toBeInTheDocument()
    expect(screen.getByLabelText('Роль admin@example.com')).toBeInTheDocument()
    expect(screen.getByLabelText('Роль user@example.com')).toBeInTheDocument()
    expect(screen.getByText((_, el) => el?.textContent === 'CI · users.list')).toBeInTheDocument()
    // Обзор: счётчик проектов = 3, активных ключей = 1.
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('API-ключи (активных)')).toBeInTheDocument()
  })

  it('показывает ошибку API', async () => {
    vi.spyOn(adminApi, 'overview').mockRejectedValue(new ApiError(403, 'forbidden', 'Нет доступа'))
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)
    expect(await screen.findByText('Нет доступа')).toBeInTheDocument()
  })

  it('смена статуса пользователя вызывает updateUser', async () => {
    mockApi()
    const update = vi.spyOn(adminApi, 'updateUser').mockResolvedValue({ status: 'ok' })
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    const statusSelect = await screen.findByLabelText('Статус user@example.com')
    fireEvent.change(statusSelect, { target: { value: 'disabled' } })

    await waitFor(() =>
      expect(update).toHaveBeenCalledWith('u-2', { status: 'disabled' }),
    )
  })

  it('сохранение политики вызывает updateSettings', async () => {
    mockApi()
    const update = vi.spyOn(adminApi, 'updateSettings').mockResolvedValue(policy)
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    await screen.findByText('Политика безопасности')
    fireEvent.change(screen.getByLabelText('Минимальная длина пароля'), {
      target: { value: '12' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить политику' }))

    await waitFor(() =>
      expect(update).toHaveBeenCalledWith(expect.objectContaining({ min_password_length: 12 })),
    )
  })

  it('создание API-ключа показывает токен один раз', async () => {
    mockApi()
    const create = vi
      .spyOn(adminApi, 'createApiKey')
      .mockResolvedValue({ ...keys[0], token: 'abc123' })
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    await screen.findByText('API-ключи')
    fireEvent.change(screen.getByPlaceholderText('Имя ключа (например, CI)'), {
      target: { value: 'CI' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Создать ключ' }))

    expect(await screen.findByText(/Сохраните токен сейчас/)).toBeInTheDocument()
    expect(create).toHaveBeenCalledWith({ name: 'CI', scopes: ['users.list'] })
  })

  it('отзыв API-ключа вызывает revokeApiKey', async () => {
    mockApi()
    const revoke = vi.spyOn(adminApi, 'revokeApiKey').mockResolvedValue(undefined)
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    const revokeBtn = await screen.findByRole('button', { name: 'Отозвать' })
    fireEvent.click(revokeBtn)

    await waitFor(() => expect(revoke).toHaveBeenCalledWith('key-1'))
  })

  it('собственный профиль нельзя менять (селекты отключены)', async () => {
    mockApi()
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)
    expect(await screen.findByLabelText('Роль admin@example.com')).toBeDisabled()
    expect(
      screen.getAllByText((_, el) => el?.textContent?.includes('это вы') === true).length,
    ).toBeGreaterThan(0)
  })

  it('экспорт открывает URL с scope и форматом', async () => {
    mockApi()
    const hrefs: string[] = []
    const loc = {
      get href() {
        return hrefs[hrefs.length - 1] ?? ''
      },
      set href(v: string) {
        hrefs.push(v)
      },
    } as Location
    vi.spyOn(window, 'location', 'get').mockReturnValue(loc)
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    await screen.findByText('Экспорт данных')
    fireEvent.click(screen.getByRole('button', { name: 'users · CSV' }))

    await waitFor(() =>
      expect(hrefs).toContain('/api/v1/admin/export?scope=users&format=csv'),
    )
  })
})
