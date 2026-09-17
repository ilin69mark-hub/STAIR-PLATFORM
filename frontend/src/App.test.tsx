// App — корневой роутер (список → детали → админ/аудит/логин).
// Держим AuthProvider реальным, но мокаем API-слои: проверяем ветки
// рендера App.tsx (loading, AuthPage, список, admin, audit, детали).
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { AuthProvider } from './auth/AuthContext'
import { ApiError } from '@shared/types'
import type { User } from '@shared/types'

const adminUser: User = {
  id: 'u-admin',
  email: 'admin@example.com',
  name: 'Админ',
  role: 'admin',
  tenant_id: 't-1',
}

const regularUser: User = {
  id: 'u-user',
  email: 'user@example.com',
  name: 'Пользователь',
  role: 'user',
  tenant_id: 't-1',
}

const project = {
  id: 'p-1',
  name: 'Лестница на второй этаж',
  description: 'Заказ клиента',
  status: 'draft',
  owner_id: adminUser.id,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-17T00:00:00Z',
}

// ---- API-моки ----
const authMock = vi.hoisted(() => {
  const state: { user: User | null; sessionError: boolean } = { user: null, sessionError: false }
  return {
    state,
    authApi: {
      me: vi.fn(async () => {
        if (state.sessionError) throw new ApiError(401, 'unauthorized', '')
        return state.user
      }),
      login: vi.fn(async () => null as User | null),
      register: vi.fn(async () => null as User | null),
      logout: vi.fn(async () => undefined),
      ssoConfig: vi.fn(async () => ({ enabled: false, provider: 'keycloak' })),
      ssoUrl: vi.fn((redirect: string) => `/api/v1/auth/sso?redirect=${encodeURIComponent(redirect)}`),
    },
  }
})

vi.mock('./api/auth', () => ({ authApi: authMock.authApi }))

const projectsMock = vi.hoisted(() => ({
  list: vi.fn(async () => [project]),
  create: vi.fn(),
  get: vi.fn(async (id: string) => (id === project.id ? project : { ...project, id })),
  calculate: vi.fn(),
  preview: vi.fn(),
  optimize: vi.fn(),
  exportUrl: vi.fn((id: string) => `/api/v1/projects/${id}/export`),
  listMembers: vi.fn(async () => []),
  addMember: vi.fn(),
  updateMemberRole: vi.fn(),
  removeMember: vi.fn(),
  listComments: vi.fn(async () => []),
  addComment: vi.fn(),
  deleteComment: vi.fn(),
  requestReview: vi.fn(),
  signOffReview: vi.fn(),
  requestChanges: vi.fn(),
  listReviews: vi.fn(async () => []),
  approveConfiguration: vi.fn(),
  getConfigurationApproval: vi.fn(),
  listApprovals: vi.fn(async () => []),
  listConfigurations: vi.fn(async () => []),
  getConfiguration: vi.fn(),
  restoreConfiguration: vi.fn(),
  assistant: vi.fn(),
}))

vi.mock('./api/projects', () => ({ projectsApi: projectsMock }))

const adminMock = vi.hoisted(() => ({
  overview: vi.fn(async () => ({
    tenant_id: 't-1',
    users: 1,
    active_users: 1,
    disabled_users: 0,
    admins: 1,
    projects: 1,
    active_api_keys: 0,
    total_api_keys: 0,
  })),
  listUsers: vi.fn(async () => [adminUser]),
  updateUser: vi.fn(async () => ({ status: 'ok' })),
  getSettings: vi.fn(async () => ({
    min_password_length: 8,
    require_number: false,
    require_upper: false,
    session_ttl_seconds: 86400,
    login_rate_limit_per_min: 10,
  })),
  updateSettings: vi.fn(),
  exportUrl: vi.fn((scope: string, format: string) => `/api/v1/admin/export?scope=${scope}&format=${format}`),
  listApiKeys: vi.fn(async () => []),
  createApiKey: vi.fn(),
  revokeApiKey: vi.fn(),
  listOrders: vi.fn(async () => []),
  updateOrderStatus: vi.fn(),
  listTestimonials: vi.fn(async () => []),
  createTestimonial: vi.fn(),
  updateTestimonial: vi.fn(),
  deleteTestimonial: vi.fn(),
}))

vi.mock('./api/admin', () => ({ adminApi: adminMock }))

const analyticsMock = vi.hoisted(() => ({
  usage: vi.fn(async () => ({
    from: '2026-09-01',
    to: '2026-09-17',
    granularity: 'day',
    totals: { users: 1, active_users: 1, projects: 1, calculations: 0, logins: 0, exports: 0, payments: 0 },
    series: [],
  })),
  projects: vi.fn(async () => ({
    from: '',
    to: '',
    totals: {
      projects: 1,
      projects_created: 0,
      by_status: {},
      projects_with_calculation: 0,
      valid_projects: 0,
      configurations: 0,
      calculations: 0,
      comments: 0,
    },
    projects: [],
  })),
  manufacturing: vi.fn(async () => ({
    from: '',
    to: '',
    granularity: 'day',
    totals: {
      calculations: 0,
      parts: 0,
      bom_lines: 0,
      cut_items: 0,
      sheets: 0,
      part_area: 0,
      sheet_area: 0,
      waste_area: 0,
      utilization: 0,
      materials: {},
    },
    series: [],
  })),
  cost: vi.fn(async () => ({
    from: '',
    to: '',
    granularity: 'day',
    totals: {
      calculations: 0,
      material: 0,
      machine: 0,
      labor: 0,
      overhead: 0,
      production_cost: 0,
      margin: 0,
      discount: 0,
      pre_tax: 0,
      tax: 0,
      final_price: 0,
      avg_final_price: 0,
      currency: 'RUB',
    },
    series: [],
  })),
}))

vi.mock('./api/analytics', () => ({ analyticsApi: analyticsMock }))

vi.mock('./api/audit', () => ({
  auditApi: {
    listProjectAudit: vi.fn(async () => []),
    listTenantAudit: vi.fn(async () => []),
  },
  actionLabels: {},
}))

vi.mock('@shared/api/audit', () => ({ logAction: vi.fn() }))

function renderApp() {
  return render(
    <AuthProvider>
      <App />
    </AuthProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  authMock.state.user = null
  authMock.state.sessionError = false
})

describe('App routing', () => {
  it('показывает «Загрузка…» пока сессия проверяется', async () => {
    authMock.state.sessionError = true
    renderApp()
    await waitFor(() => expect(screen.getByText('Загрузка…')).toBeInTheDocument())
  })

  it('без пользователя рендерит страницу входа', async () => {
    renderApp()
    await waitFor(() => expect(screen.getByRole('heading', { name: 'STAIR PLATFORM' })).toBeInTheDocument())
    expect(screen.getByLabelText('Email')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument()
  })

  it('рендерит список проектов для авторизованного пользователя', async () => {
    authMock.state.user = regularUser
    renderApp()
    await waitFor(() => expect(screen.getByText('Лестница на второй этаж')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Создать проект' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Администрирование' })).not.toBeInTheDocument()
  })

  it('админ видит кнопки «Администрирование» и «Аудит действий»', async () => {
    authMock.state.user = adminUser
    renderApp()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Создать проект' })).toBeInTheDocument(),
    )
    expect(screen.getByRole('button', { name: 'Администрирование' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Аудит действий' })).toBeInTheDocument()
  })

  it('открывает AdminPanel и возвращается назад', async () => {
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: 'Администрирование' }))
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Администрирование' })).toBeInTheDocument(),
    )
    fireEvent.click(screen.getByRole('button', { name: '← Проекты' }))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Создать проект' })).toBeInTheDocument(),
    )
  })

  it('открывает AuditPage и возвращается назад', async () => {
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: 'Аудит действий' }))
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Аудит действий' })).toBeInTheDocument(),
    )
    fireEvent.click(screen.getByRole('button', { name: 'Назад' }))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Создать проект' })).toBeInTheDocument(),
    )
  })

  it('открывает детали проекта и возвращается к списку', async () => {
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByText('Лестница на второй этаж'))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '← Проекты' })).toBeInTheDocument(),
    )
    fireEvent.click(screen.getByRole('button', { name: '← Проекты' }))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Создать проект' })).toBeInTheDocument(),
    )
  })

  it('выход из системы вызывает logout и возвращает на страницу входа', async () => {
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: 'Выйти' }))
    await waitFor(() => expect(authMock.authApi.logout).toHaveBeenCalled())
    await waitFor(() => expect(screen.getByLabelText('Email')).toBeInTheDocument())
  })

  it('показывает ошибку загрузки списка проектов', async () => {
    projectsMock.list.mockRejectedValueOnce(new ApiError(500, 'unknown', 'HTTP 500'))
    authMock.state.user = regularUser
    renderApp()
    await waitFor(() => expect(screen.getByText('HTTP 500')).toBeInTheDocument())
  })
})