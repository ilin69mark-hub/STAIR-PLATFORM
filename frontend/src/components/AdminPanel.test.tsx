import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AdminPanel } from './AdminPanel'
import { adminApi } from '../api/admin'
import { analyticsApi } from '../api/analytics'
import {
  ApiError,
  type AdminOverview,
  type AdminPolicy,
  type AdminUser,
  type ApiKey,
  type CostReport,
  type ManufacturingReport,
  type OrderDTO,
  type ProjectReport,
  type TestimonialDTO,
  type UsageReport,
} from '@shared/types'

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

const usage: UsageReport = {
  from: '2026-07-16',
  to: '2026-08-15',
  granularity: 'day',
  totals: {
    users: 2,
    active_users: 1,
    projects: 3,
    calculations: 4,
    logins: 5,
    exports: 1,
    payments: 2,
  },
  series: [
    {
      bucket: '2026-08-15',
      logins: 2,
      active_users: 1,
      projects_created: 1,
      calculations: 2,
      exports: 1,
      payments: 1,
    },
  ],
}

const projects: ProjectReport = {
  from: '2026-07-16',
  to: '2026-08-15',
  totals: {
    projects: 2,
    projects_created: 1,
    by_status: { draft: 1, approved: 1 },
    projects_with_calculation: 1,
    valid_projects: 1,
    configurations: 2,
    calculations: 1,
    comments: 3,
  },
  projects: [
    {
      id: 'p-1',
      name: 'Approved project',
      status: 'approved',
      owner_email: 'owner@example.com',
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-10T00:00:00Z',
      configurations: 2,
      calculations: 1,
      latest_calculation_valid: true,
      comments: 3,
      members: 2,
    },
  ],
}

const manufacturing: ManufacturingReport = {
  from: '2026-07-16',
  to: '2026-08-15',
  granularity: 'day',
  totals: {
    calculations: 2,
    parts: 10,
    bom_lines: 4,
    cut_items: 2,
    sheets: 1,
    part_area: 1000,
    sheet_area: 2000,
    waste_area: 1000,
    utilization: 0.5,
    materials: { 'STEEL-S235': 10 },
  },
  series: [
    {
      bucket: '2026-08-15',
      calculations: 2,
      parts: 10,
      sheets: 1,
      utilization: 0.5,
    },
  ],
}

const cost: CostReport = {
  from: '2026-07-16',
  to: '2026-08-15',
  granularity: 'day',
  totals: {
    calculations: 2,
    material: 200,
    machine: 40,
    labor: 60,
    overhead: 20,
    production_cost: 320,
    margin: 80,
    discount: 0,
    pre_tax: 400,
    tax: 40,
    final_price: 440,
    avg_final_price: 220,
    currency: 'RUB',
  },
  series: [
    {
      bucket: '2026-08-15',
      calculations: 2,
      final_price: 440,
      avg_final_price: 220,
    },
  ],
}

const orders: OrderDTO[] = [
  {
    id: 'order-1',
    kind: 'order',
    status: 'new',
    contact: { name: 'Иван Клиент', email: 'buyer@example.com', phone: '+7 900 000-00-00' },
    config: { width_mm: 900, height_mm: 2700 },
    price: { final_price_rub: 180000 },
    created_at: '2026-08-17T10:00:00Z',
    updated_at: '2026-08-17T10:00:00Z',
  },
  {
    id: 'order-2',
    kind: 'consultation',
    status: 'new',
    contact: { name: 'Анна Клиент', email: 'anna@example.com', phone: '' },
    config: { question: 'Какая минимальная ширина лестницы?' },
    created_at: '2026-08-17T11:00:00Z',
    updated_at: '2026-08-17T11:00:00Z',
  },
]

const testimonials: TestimonialDTO[] = [
  {
    id: 't-1',
    author: 'Мария',
    text: 'Отличная лестница, очень довольны!',
    rating: 5,
    published: true,
    created_at: '2026-08-17T10:00:00Z',
  },
]

function mockApi() {
  vi.spyOn(adminApi, 'overview').mockResolvedValue(overview)
  vi.spyOn(adminApi, 'listUsers').mockResolvedValue(users)
  vi.spyOn(adminApi, 'getSettings').mockResolvedValue(policy)
  vi.spyOn(adminApi, 'listApiKeys').mockResolvedValue(keys)
  vi.spyOn(adminApi, 'listOrders').mockResolvedValue(orders)
  vi.spyOn(adminApi, 'listTestimonials').mockResolvedValue(testimonials)
  vi.spyOn(analyticsApi, 'usage').mockResolvedValue(usage)
  vi.spyOn(analyticsApi, 'projects').mockResolvedValue(projects)
  vi.spyOn(analyticsApi, 'manufacturing').mockResolvedValue(manufacturing)
  vi.spyOn(analyticsApi, 'cost').mockResolvedValue(cost)
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
    // Обзор: счётчик проектов = 3 (встречается и в аналитике).
    expect(screen.getAllByText('3').length).toBeGreaterThan(0)
    expect(screen.getByText('Активных API-ключей')).toBeInTheDocument()
  })

  it('показывает ошибку API', async () => {
    vi.spyOn(adminApi, 'overview').mockRejectedValue(new ApiError(403, 'forbidden', 'Нет доступа'))
    vi.spyOn(analyticsApi, 'usage').mockResolvedValue(usage)
    vi.spyOn(analyticsApi, 'projects').mockResolvedValue(projects)
    vi.spyOn(analyticsApi, 'manufacturing').mockResolvedValue(manufacturing)
    vi.spyOn(analyticsApi, 'cost').mockResolvedValue(cost)
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

    await screen.findByRole('heading', { name: 'API-ключи' })
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

  it('показывает аналитику использования и меняет гранулярность', async () => {
    mockApi()
    const usageMock = vi.spyOn(analyticsApi, 'usage').mockResolvedValue(usage)
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    expect(await screen.findByText('Аналитика использования')).toBeInTheDocument()
    // Автозагрузка с гранулярностью по умолчанию.
    await waitFor(() =>
      expect(usageMock).toHaveBeenCalledWith(expect.objectContaining({ granularity: 'day' })),
    )
    // Итоги отчёта: пользователи/активные = 2 / 1.
    expect(await screen.findByText('Пользователи / активные')).toBeInTheDocument()
    expect(screen.getByText('2 / 1')).toBeInTheDocument()
    expect(screen.getByText('4')).toBeInTheDocument()
    // Серия: бакет 2026-08-15 (встречается и в таблице производства).
    expect(screen.getAllByText('2026-08-15').length).toBeGreaterThan(0)

    // Переключение гранулярности (две секции имеют кнопку «Месяц»).
    fireEvent.click(screen.getAllByRole('button', { name: 'Месяц' })[0])
    await waitFor(() =>
      expect(usageMock).toHaveBeenCalledWith(expect.objectContaining({ granularity: 'month' })),
    )
  })

  it('показывает ошибку аналитики', async () => {
    vi.spyOn(adminApi, 'overview').mockResolvedValue(overview)
    vi.spyOn(adminApi, 'listUsers').mockResolvedValue(users)
    vi.spyOn(adminApi, 'getSettings').mockResolvedValue(policy)
    vi.spyOn(adminApi, 'listApiKeys').mockResolvedValue(keys)
    vi.spyOn(adminApi, 'listOrders').mockResolvedValue([])
    vi.spyOn(adminApi, 'listTestimonials').mockResolvedValue([])
    vi.spyOn(analyticsApi, 'usage').mockRejectedValue(
      new ApiError(403, 'forbidden', 'Нет права analytics.read'),
    )
    vi.spyOn(analyticsApi, 'projects').mockResolvedValue(projects)
    vi.spyOn(analyticsApi, 'manufacturing').mockResolvedValue(manufacturing)
    vi.spyOn(analyticsApi, 'cost').mockResolvedValue(cost)
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    expect(await screen.findByText('Нет права analytics.read')).toBeInTheDocument()
  })
  it('показывает сводку по проектам', async () => {
    mockApi()
    const projectsMock = vi.spyOn(analyticsApi, 'projects').mockResolvedValue(projects)
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    expect(await screen.findByText('Проекты (в окне)')).toBeInTheDocument()
    // Агрегаты: 2 проекта, 1 черновик, 1 утверждён.
    expect(screen.getByText('2 (1)')).toBeInTheDocument()
    expect(screen.getAllByText('1 / 1').length).toBeGreaterThan(0)
    // Таблица: проект Approved project, последний расчёт валиден.
    expect(screen.getByText('Approved project')).toBeInTheDocument()
    expect(screen.getByText('валиден')).toBeInTheDocument()
    // Окно отчёта: последние 30 дней от текущей даты (логика компонента).
    const now = new Date()
    const from = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
    await waitFor(() =>
      expect(projectsMock).toHaveBeenCalledWith(
        expect.objectContaining({
          from: from.toISOString().slice(0, 10),
          to: now.toISOString().slice(0, 10),
        }),
      ),
    )
  })

  it('показывает аналитику производства и меняет гранулярность', async () => {
    mockApi()
    const mfgMock = vi.spyOn(analyticsApi, 'manufacturing').mockResolvedValue(manufacturing)
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    expect(await screen.findByRole('heading', { name: 'Производство' })).toBeInTheDocument()
    // Автозагрузка с гранулярностью по умолчанию.
    await waitFor(() =>
      expect(mfgMock).toHaveBeenCalledWith(expect.objectContaining({ granularity: 'day' })),
    )
    // Агрегаты: 2 / 10 расчётов/деталей, утилизация 50.0%.
    expect(screen.getByText('2 / 10')).toBeInTheDocument()
    expect(screen.getAllByText('50.0%').length).toBeGreaterThan(0)
    // Серия: бакет 2026-08-15.
    expect(screen.getAllByText('2026-08-15').length).toBeGreaterThan(0)
    // Материал.
    expect(screen.getByText('STEEL-S235: 10')).toBeInTheDocument()

    fireEvent.click(screen.getAllByRole('button', { name: 'Неделя' })[1])
    await waitFor(() =>
      expect(mfgMock).toHaveBeenCalledWith(expect.objectContaining({ granularity: 'week' })),
    )
  })

  it('показывает аналитику стоимости и меняет гранулярность', async () => {
    mockApi()
    const costMock = vi.spyOn(analyticsApi, 'cost').mockResolvedValue(cost)
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    expect(await screen.findByRole('heading', { name: 'Стоимость' })).toBeInTheDocument()
    await waitFor(() =>
      expect(costMock).toHaveBeenCalledWith(expect.objectContaining({ granularity: 'day' })),
    )
    // Агрегаты: итоговая цена 440 / средняя 220 RUB.
    expect(screen.getByText('440 / 220.00 RUB')).toBeInTheDocument()
    // Серия: бакет 2026-08-15.
    expect(screen.getAllByText('2026-08-15').length).toBeGreaterThan(0)

    fireEvent.click(screen.getAllByRole('button', { name: 'Месяц' })[2])
    await waitFor(() =>
      expect(costMock).toHaveBeenCalledWith(expect.objectContaining({ granularity: 'month' })),
    )
  })

  it('показывает заказы store и меняет статус', async () => {
    mockApi()
    const statusMock = vi.spyOn(adminApi, 'updateOrderStatus').mockResolvedValue({
      ...orders[0],
      status: 'confirmed',
    })
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    expect(await screen.findByText('Заказы (store)')).toBeInTheDocument()
    expect(screen.getByText('Иван Клиент')).toBeInTheDocument()
    expect(screen.getByText(/buyer@example.com/)).toBeInTheDocument()
    expect(screen.getByText(/900 000-00-00/)).toBeInTheDocument()
    expect(screen.getByText(/900 × 2700 мм/)).toBeInTheDocument()
    // Консультация: метка + вопрос вместо размеров/цены.
    expect(screen.getByText('Консультация')).toBeInTheDocument()
    expect(screen.getByText('Какая минимальная ширина лестницы?')).toBeInTheDocument()

    // Смена статуса через select заказа.
    fireEvent.change(screen.getByLabelText('Статус заказа order-1'), {
      target: { value: 'confirmed' },
    })
    await waitFor(() =>
      expect(statusMock).toHaveBeenCalledWith('order-1', 'confirmed'),
    )
    expect(screen.getByText('Статус заказа обновлён')).toBeInTheDocument()
  })

  it('показывает отзывы, публикует/скрывает и удаляет', async () => {
    mockApi()
    const publish = vi.spyOn(adminApi, 'updateTestimonial').mockResolvedValue({
      ...testimonials[0],
      published: false,
    })
    const remove = vi.spyOn(adminApi, 'deleteTestimonial').mockResolvedValue(undefined)
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    expect(await screen.findByText('Отзывы (store)')).toBeInTheDocument()
    expect(screen.getByText(/Мария/)).toBeInTheDocument()
    expect(screen.getByText('Отличная лестница, очень довольны!')).toBeInTheDocument()
    expect(screen.getByText('Опубликован')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Скрыть' }))
    await waitFor(() =>
      expect(publish).toHaveBeenCalledWith('t-1', {
        author: 'Мария',
        text: 'Отличная лестница, очень довольны!',
        rating: 5,
        published: false,
      }),
    )

    fireEvent.click(screen.getByRole('button', { name: 'Удалить' }))
    await waitFor(() => expect(remove).toHaveBeenCalledWith('t-1'))
  })

  it('добавляет отзыв через форму', async () => {
    mockApi()
    const create = vi.spyOn(adminApi, 'createTestimonial').mockResolvedValue(testimonials[0])
    render(<AdminPanel currentUserId="u-admin" onBack={vi.fn()} />)

    await screen.findByText('Отзывы (store)')
    fireEvent.change(screen.getByPlaceholderText('Автор (имя клиента)'), {
      target: { value: 'Пётр' },
    })
    fireEvent.change(screen.getByPlaceholderText('Текст отзыва'), {
      target: { value: 'Всё понравилось' },
    })
    fireEvent.change(screen.getByLabelText('Оценка отзыва'), {
      target: { value: '4' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Добавить отзыв' }))

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({
        author: 'Пётр',
        text: 'Всё понравилось',
        rating: 4,
      }),
    )
  })
})
