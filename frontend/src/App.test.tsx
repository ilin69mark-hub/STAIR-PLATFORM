// App — корневой экран кабинета: приветственный экран с плитками → страница
// плитки → проект. Держим AuthProvider реальным, но мокаем API-слои.
//
// Проверяем ровно то, ради чего верхний уровень переделывался в плитки:
// плитка появляется только при наличии метрики внутри неё, раздел внутри
// плитки — по своей метрике, выход из системы есть на любом экране, а потеря
// права закрывает открытую плитку.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { AuthProvider } from './auth/AuthContext'
import { ApiError } from '@shared/types'
import type { User } from '@shared/types'
import { P } from '@shared/cabinetPermissions'

// Роль admin на бэкенде отдаёт все права (Role.Permissions()), поэтому фикстура
// перечисляет их явно: гейт вкладок читает permissions, а не role.
const adminUser: User = {
  id: 'u-admin',
  email: 'admin@example.com',
  name: 'Админ',
  role: 'admin',
  tenant_id: 't-1',
  permissions: Object.values(P),
}

const regularUser: User = {
  id: 'u-user',
  email: 'user@example.com',
  name: 'Пользователь',
  role: 'user',
  tenant_id: 't-1',
}

// Руководитель продаж: не админ платформы, но ведёт заказы и платежи.
const salesHead: User = {
  id: 'u-sales',
  email: 'sales@example.com',
  name: 'Руководитель продаж',
  role: 'sales',
  tenant_id: 't-1',
  permissions: [P.financeSales, P.financePayments, P.trafficFunnel, P.staffView, P.priceMaterials],
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

const storeMock = vi.hoisted(() => {
  // Полный документ настроек по DTO бэкенда: разделы рендерятся все сразу
  // (одним скроллом), поэтому фикстура обязана покрывать все группы.
  const settings = {
    contacts: { phone: '+7 (999) 123-45-67', email: 'sales@example.com', address: 'Москва', work_hours: 'Пн–Пт 9:00–18:00' },
    company: {
      name: 'ООО Лестница',
      legal_name: 'Общество с ограниченной ответственностью «Лестница»',
      inn: '7701234567',
      ogrn: '1157746000000',
      email: 'office@example.com',
      website: 'https://example.com',
    },
    social: { telegram: 'example', vk: 'example', whatsapp: 'example', youtube: 'example' },
    seo: { default_title: 'Лестницы', default_description: 'Расчёт лестниц', og_image: '' },
    counters: { yandex_metrika_id: '', ga4_measurement_id: '' },
    rates: {
      machine_per_hour_rub: 1200,
      labor_per_hour_rub: 800,
      overhead_percent: 10,
      margin_percent: 20,
      discount_percent: 5,
      tax_percent: 20,
    },
    updated_by: 'admin@admin.ru',
    updated_at: '2026-09-17T00:00:00Z',
  }
  return {
    settings,
    settingsFn: vi.fn(async () => settings),
    saveSettingsGroup: vi.fn(async (_group: string, _settings: unknown) => settings),
    setPrice: vi.fn(async () => undefined),
    resetPrice: vi.fn(async () => undefined),
    materials: vi.fn(async () => []),
    tiers: vi.fn(async () => []),
    payments: vi.fn(async () => []),
    refundPayment: vi.fn(async () => ({ id: 'pay-1' })),
  }
})

vi.mock('./api/store', () => ({
  storeApi: {
    settings: storeMock.settingsFn,
    saveSettingsGroup: storeMock.saveSettingsGroup,
    prices: vi.fn(async () => []),
    setPrice: storeMock.setPrice,
    resetPrice: storeMock.resetPrice,
    // Крепёж (MNT-0001): пустой прайс — блок «Крепление» покажет пустой
    // список, расчёт при этом не должен падать.
    fasteners: vi.fn(async () => []),
    setFastenerPrice: vi.fn(async () => ({})),
    resetFastenerPrice: vi.fn(async () => undefined),
    // Свои позиции крепежа (000048): пусто — блок «Свои позиции» покажет
    // подсказку о встроенном каталоге.
    fastenerPositions: vi.fn(async () => []),
    createFastenerPosition: vi.fn(async () => ({})),
    deleteFastenerPosition: vi.fn(async () => undefined),
    // Герметик (CONF-SEALANT): пустой прайс, как и у крепежа.
    sealants: vi.fn(async () => []),
    setSealantPrice: vi.fn(async () => ({})),
    resetSealantPrice: vi.fn(async () => undefined),
    // Закупочный прайс ограждения (CONF-PURCHASED): пустой, как и у крепежа.
    purchased: vi.fn(async () => []),
    setPurchasedPrice: vi.fn(async () => ({})),
    resetPurchasedPrice: vi.fn(async () => undefined),
    materials: storeMock.materials,
    tiers: storeMock.tiers,
    payments: storeMock.payments,
    refundPayment: storeMock.refundPayment,
  },
}))

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
  // Плитка «Проекты» — единственная без метрик, она открыта любому.
  const openProjects = async () => {
    fireEvent.click(await screen.findByRole('button', { name: /^Проекты/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Создать проект' })).toBeInTheDocument())
  }

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

  it('открывает приветственный экран с плитками, а не список проектов', async () => {
    authMock.state.user = adminUser
    renderApp()
    await waitFor(() => expect(screen.getByRole('button', { name: /^Проекты/ })).toBeInTheDocument())
    // Список проектов спрятан за плиткой: сразу после входа его нет.
    expect(screen.queryByText('Лестница на второй этаж')).not.toBeInTheDocument()
  })

  it('админ видит все плитки кабинета', async () => {
    authMock.state.user = adminUser
    renderApp()
    await screen.findByRole('button', { name: /^Проекты/ })
    for (const title of [
      'Проекты',
      'Сайт и витрина',
      'Цены и расчёт',
      'Продажи и деньги',
      'Производство',
      'Реклама и трафик',
      'Команда',
    ]) {
      expect(screen.getByRole('button', { name: new RegExp(`^${title}`) })).toBeInTheDocument()
    }
  })

  it('плитка «Проекты» открывает список и кнопку создания', async () => {
    authMock.state.user = adminUser
    renderApp()
    await openProjects()
    expect(screen.getByText('Лестница на второй этаж')).toBeInTheDocument()
  })

  it('показывает только плитки, в которых что-то выдано', async () => {
    // Руководитель продаж: заказы, платежи, воронка, сотрудники (чтение), цены.
    authMock.state.user = salesHead
    renderApp()
    await screen.findByRole('button', { name: /^Проекты/ })
    expect(screen.getByRole('button', { name: /^Продажи и деньги/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Реклама и трафик/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Команда/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Цены и расчёт/ })).toBeInTheDocument()
    // Ни контактов, ни реквизитов, ни себестоимости ему не выдавали.
    expect(screen.queryByRole('button', { name: /^Сайт и витрина/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Производство/ })).not.toBeInTheDocument()
  })

  it('плитка без выданных метрик не показывается', async () => {
    // Публичный каталог материалов — единственная причина, по которой
    // «Сайт и витрина» могла бы светиться каждому; её одной не хватает.
    authMock.state.user = { ...regularUser, permissions: [P.priceMaterials] }
    renderApp()
    await screen.findByRole('button', { name: /^Проекты/ })
    expect(screen.queryByRole('button', { name: /^Сайт и витрина/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Цены и расчёт/ })).toBeInTheDocument()
  })

  it('плитка скрывает разделы без нужного права', async () => {
    // Право только на заказы: платежей, себестоимости и выгрузки не видно.
    authMock.state.user = { ...salesHead, permissions: [P.financeSales] }
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: /^Продажи и деньги/ }))
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Заказы и продажи' })).toBeInTheDocument(),
    )
    expect(screen.queryByRole('heading', { name: 'Платежи' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Себестоимость и прибыль' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Выгрузка данных' })).not.toBeInTheDocument()
  })

  it('журнал аудита живёт внутри плитки «Команда»', async () => {
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: /^Команда/ }))
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Сотрудники' })).toBeInTheDocument(),
    )
    expect(screen.getByRole('heading', { name: 'Политика безопасности' })).toBeInTheDocument()
    // Отдельной вкладки «Аудит» больше нет: журнал — раздел в одном скролле.
    expect(screen.queryByRole('button', { name: 'Аудит' })).not.toBeInTheDocument()
  })

  it('настройки витрины разнесены по разделам своей плитки', async () => {
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: /^Сайт и витрина/ }))
    await waitFor(() => expect(screen.getByLabelText('Телефон')).toBeInTheDocument())
    expect(screen.getByLabelText('Юридическое лицо')).toBeInTheDocument()
    expect(screen.getByLabelText('Яндекс.Метрика (ID)')).toBeInTheDocument()
    // Ставки расчёта — не в витрине, а в ценах.
    expect(screen.queryByLabelText('Маржа, %')).not.toBeInTheDocument()
  })

  it('ставки расчёта сохраняются своей группой', async () => {
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: /^Цены и расчёт/ }))
    const margin = await screen.findByLabelText('Маржа, %')
    fireEvent.change(margin, { target: { value: '35' } })
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить ставки' }))
    await waitFor(() => expect(storeMock.saveSettingsGroup).toHaveBeenCalled())
    // Первый аргумент — группа: контакты не должны ехать вместе со ставками.
    expect(storeMock.saveSettingsGroup.mock.calls[0][0]).toBe('rates')
  })

  it('навигация плитки возвращает на приветственный экран', async () => {
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: /^Команда/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: '← Плитки' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: '← Плитки' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /^Проекты/ })).toBeInTheDocument())
  })

  it('открывает детали проекта и возвращается к плиткам', async () => {
    authMock.state.user = adminUser
    renderApp()
    await openProjects()
    fireEvent.click(screen.getByText('Лестница на второй этаж'))
    await waitFor(() => expect(screen.getByRole('button', { name: '← Проекты' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: '← Проекты' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /^Проекты/ })).toBeInTheDocument())
  })

  it('создание проекта обновляет список и открывает новый проект', async () => {
    authMock.state.user = regularUser
    projectsMock.create.mockResolvedValueOnce({ ...project, id: 'p-2', name: 'Лесенка 2' })
    renderApp()
    await openProjects()
    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Лесенка 2' } })
    fireEvent.click(screen.getByRole('button', { name: 'Создать проект' }))
    await waitFor(() =>
      expect(projectsMock.create).toHaveBeenCalledWith({ name: 'Лесенка 2', description: '' }),
    )
    // onCreated → void refresh() (перезагрузка списка) + переход на детали нового проекта
    await waitFor(() => expect(screen.getByRole('button', { name: '← Проекты' })).toBeInTheDocument())
    expect(projectsMock.list).toHaveBeenCalled()
  })

  it('выход из системы доступен с приветственного экрана', async () => {
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: 'Выйти' }))
    await waitFor(() => expect(authMock.authApi.logout).toHaveBeenCalled())
    await waitFor(() => expect(screen.getByLabelText('Email')).toBeInTheDocument())
  })

  it('выход из системы доступен и внутри плитки', async () => {
    // Раньше кнопка выхода была только на экране проектов: из админки, аудита
    // и магазина выйти было нельзя. Общий каркас кабинета это закрыл.
    authMock.state.user = adminUser
    renderApp()
    fireEvent.click(await screen.findByRole('button', { name: /^Команда/ }))
    fireEvent.click(await screen.findByRole('button', { name: 'Выйти' }))
    await waitFor(() => expect(authMock.authApi.logout).toHaveBeenCalled())
    await waitFor(() => expect(screen.getByLabelText('Email')).toBeInTheDocument())
  })

  it('показывает ошибку загрузки списка проектов', async () => {
    projectsMock.list.mockRejectedValueOnce(new ApiError(500, 'unknown', 'HTTP 500'))
    authMock.state.user = regularUser
    renderApp()
    await openProjects()
    await waitFor(() => expect(screen.getByText('HTTP 500')).toBeInTheDocument())
  })

  it('не-ApiError ошибка списка показывает общее сообщение', async () => {
    projectsMock.list.mockRejectedValueOnce(new Error('boom'))
    authMock.state.user = regularUser
    renderApp()
    await openProjects()
    await waitFor(() =>
      expect(screen.getByText('Не удалось загрузить проекты')).toBeInTheDocument(),
    )
  })
})
