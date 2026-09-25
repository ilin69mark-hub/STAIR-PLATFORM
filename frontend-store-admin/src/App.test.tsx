import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { api, ApiError } from './api/client'
import { loginErrorMessage } from './api/loginError'
import type { AdminOrder, MaterialPrice, StoreSettings } from './api/store'

const settings: StoreSettings = {
  contacts: { phone: '+7 900 000-00-00', email: '', address: '', work_hours: '' },
  company: { name: 'Лестницы', legal_name: '', inn: '', ogrn: '', email: '', website: '' },
  social: { telegram: '', vk: '', whatsapp: '', youtube: '' },
  seo: { default_title: 'Лестницы на заказ', default_description: 'Описание', og_image: '' },
  counters: { yandex_metrika_id: '', ga4_measurement_id: '' },
  rates: {
    machine_per_hour_rub: 0,
    labor_per_hour_rub: 0,
    overhead_percent: 20,
    margin_percent: 30,
    discount_percent: 5,
    tax_percent: 20,
  },
  updated_at: '2026-09-25T10:00:00Z',
}

const prices: MaterialPrice[] = [
  { code: 'WOOD-OAK', price_per_kg_rub: 4200, overridden: true },
  { code: 'STEEL-S235', price_per_kg_rub: 100, overridden: false },
]

const order: AdminOrder = {
  id: 'order-1',
  kind: 'order',
  status: 'new',
  contact: { name: 'Иван', email: 'ivan@example.com', phone: '+7 900 000-00-00' },
  config: { width_mm: 900 },
  price: { total_rub: 10000 },
  created_at: '2026-09-25T10:00:00Z',
  updated_at: '2026-09-25T10:00:00Z',
}

function jsonResponse(payload: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(payload), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

const user = { id: 'u-1', email: 'admin@example.com', name: 'Админ', role: 'admin' }

function routeFetch(path: string, payload: unknown) {
  return vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
    void init
    if (!String(_url).endsWith(path)) {
      return jsonResponse({ error: { code: 'not_found', message: 'нет маршрута в тесте' } }, 404)
    }
    return jsonResponse(payload)
  })
}

// fetch панели: /auth/me отдаёт вошедшего админа, остальные пути — 404.
function panelFetch() {
  return vi.fn((url: RequestInfo | URL) => {
    const path = String(url)
    if (path.endsWith('/api/v1/auth/me')) return jsonResponse(user)
    if (path.includes('/admin/orders')) return jsonResponse([])
    if (path.includes('/admin/payments')) return jsonResponse([])
    if (path.includes('/public/materials')) return jsonResponse([])
    if (path.includes('/payment-tiers')) return jsonResponse([])
    return jsonResponse({ error: { code: 'not_found' } }, 404)
  })
}

// withSession — оборачивает мок так, чтобы /auth/me отдавал админа.
function withSession(
  mock: (url: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
) {
  return vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
    if (String(url).endsWith('/api/v1/auth/me')) return jsonResponse(user)
    return mock(url, init)
  })
}

beforeEach(() => {
  vi.restoreAllMocks()
})

describe('App', () => {
  it('показывает все разделы панели и открывает обзор по умолчанию', async () => {
    vi.stubGlobal('fetch', panelFetch())

    render(<App />)
    for (const title of [
      'Обзор',
      'Цены',
      'Настройки',
      'Материалы',
      'Услуги',
      'Заказы',
      'Контент',
      'Медиа',
      'Метрики',
      'Письма',
      'Платежи',
    ]) {
      expect(await screen.findByRole('button', { name: title })).toBeInTheDocument()
    }
    expect(screen.getByRole('heading', { name: 'Обзор' })).toBeInTheDocument()
  })

  it('открывает готовый раздел «Платежи»', async () => {
    vi.stubGlobal('fetch', panelFetch())
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Платежи' }))

    expect(await screen.findByRole('heading', { name: 'Платежи' })).toBeInTheDocument()
    expect(await screen.findByText('Платежей пока нет.')).toBeInTheDocument()
    expect(screen.queryByText(/Платежи и возвраты — волна 1/)).not.toBeInTheDocument()
  })

  it('переключает раздел «Цены» и показывает прайс материалов', async () => {
    vi.stubGlobal('fetch', withSession(routeFetch('/api/v1/admin/store/prices', prices)))
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Цены' }))
    expect(await screen.findByRole('heading', { name: 'Цены материалов' })).toBeInTheDocument()
    expect(screen.getByLabelText('Цена WOOD-OAK')).toHaveValue(4200)
    expect(screen.getByText('задана магазином')).toBeInTheDocument()
    expect(screen.getByText('встроенная ставка')).toBeInTheDocument()
  })

  it('сохраняет цену материала через admin API с заголовком origin', async () => {
    const fetchMock = withSession(routeFetch('/api/v1/admin/store/prices', prices))
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Цены' }))
    const input = await screen.findByLabelText('Цена WOOD-OAK')
    await user.clear(input)
    await user.type(input, '5000')
    await user.click(screen.getAllByRole('button', { name: 'Сохранить' })[0])

    await waitFor(() => {
      const putCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'PUT')
      expect(putCall).toBeDefined()
      const init = putCall?.[1] as RequestInit
      expect(String(init.body)).toContain('"price_per_kg_rub":5000')
      expect((init.headers as Record<string, string>)['X-App-Origin']).toBe('admin')
    })
  })

  it('редактирует и сохраняет настройки магазина', async () => {
    const fetchMock = withSession(routeFetch('/api/v1/admin/store/settings', settings))
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Настройки' }))
    const phone = await screen.findByLabelText('Телефон')
    await user.clear(phone)
    await user.type(phone, '+7 999 111-22-33')
    await user.click(screen.getByRole('button', { name: 'Сохранить настройки' }))

    await waitFor(() => {
      const putCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'PUT')
      expect(putCall).toBeDefined()
      expect(String(putCall?.[1]?.body)).toContain('+7 999 111-22-33')
    })
  })

  it('показывает ошибку бэкенда без падения панели', async () => {
    const fetchMock = withSession(routeFetch('/api/v1/admin/orders', []))
    fetchMock.mockImplementation((url: RequestInfo | URL, _init?: RequestInit) => {
      const path = String(url)
      if (path.endsWith('/api/v1/auth/me')) return jsonResponse(user)
      return jsonResponse(
        { error: { code: 'forbidden', message: 'Требуются права администратора магазина' } },
        403,
      )
    })
    vi.stubGlobal('fetch', fetchMock)
    render(<App />)
    expect(await screen.findByText('Требуются права администратора магазина')).toBeInTheDocument()
  })

  it('без сессии показывает форму входа, а разделы недоступны', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((url: RequestInfo | URL) =>
        String(url).endsWith('/api/v1/auth/me')
          ? jsonResponse({ error: { code: 'unauthorized' } }, 401)
          : jsonResponse([]),
      ),
    )
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Вход в панель магазина' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Цены' })).not.toBeInTheDocument()
  })

  it('вход через форму открывает панель и несёт admin-origin', async () => {
    const fetchMock = vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      const path = String(url)
      void init
      if (path.endsWith('/api/v1/auth/login')) return jsonResponse({ user, token: 't' })
      // До входа сессии нет: /auth/me отвечает 401.
      if (path.endsWith('/api/v1/auth/me')) return jsonResponse({ error: { code: 'unauthorized' } }, 401)
      return jsonResponse([])
    })
    vi.stubGlobal('fetch', fetchMock)
    const user_ = userEvent.setup()
    render(<App />)

    await user_.type(await screen.findByRole('textbox', { name: 'Email' }), 'admin@example.com')
    await user_.type(screen.getByLabelText('Пароль'), 'secret123')
    await user_.click(screen.getByRole('button', { name: 'Войти' }))

    expect(await screen.findByRole('heading', { name: 'Обзор' })).toBeInTheDocument()
    const loginCall = fetchMock.mock.calls.find(([url]) => String(url).endsWith('/api/v1/auth/login'))
    const headers = loginCall?.[1]?.headers as Record<string, string> | undefined
    expect(headers?.['X-App-Origin']).toBe('admin')
  })

  it('неверный пароль показывает понятную ошибку', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((url: RequestInfo | URL) =>
        String(url).endsWith('/api/v1/auth/login')
          ? jsonResponse({ error: { code: 'invalid_credentials', message: 'nope' } }, 401)
          : jsonResponse({ error: { code: 'unauthorized' } }, 401),
      ),
    )
    const user_ = userEvent.setup()
    render(<App />)

    await user_.type(await screen.findByRole('textbox', { name: 'Email' }), 'admin@example.com')
    await user_.type(screen.getByLabelText('Пароль'), 'wrong')
    await user_.click(screen.getByRole('button', { name: 'Войти' }))
    expect(await screen.findByText('Неверный email или пароль.')).toBeInTheDocument()
  })

  it('заглушка неготового раздела объясняет план', async () => {
    vi.stubGlobal('fetch', panelFetch())
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Метрики' }))
    expect(screen.getByText(/волна 2/i)).toBeInTheDocument()
  })

  it('показывает каталог материалов и услуг', async () => {
    const materials = [
      {
        code: 'STEEL-S235',
        name: 'Steel',
        name_ru: 'Сталь',
        category: 'metal',
        density_kg_m3: 7850,
        min_thickness_mm: 3,
        max_thickness_mm: 8,
        max_width_mm: 1000,
        max_height_mm: 3000,
        price_per_kg_rub: 100,
        swatch_url: '/steel.jpg',
      },
    ]
    const tiers = [
      {
        id: 'design',
        title: 'Проект',
        description: 'Чертежи',
        amount_minor: 500000,
        amount_rub: 5000,
        currency: 'RUB',
      },
    ]
    const fetchMock = withSession(
      vi.fn((url: RequestInfo | URL) => {
        const path = String(url)
        if (path.endsWith('/public/materials')) return jsonResponse(materials)
        if (path.endsWith('/public/payment-tiers')) return jsonResponse(tiers)
        return jsonResponse([])
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Материалы' }))
    expect(await screen.findByText('Сталь')).toBeInTheDocument()
    expect(screen.getByText('3–8')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Услуги' }))
    expect(await screen.findByText('Проект')).toBeInTheDocument()
    expect(screen.getByText('5000 RUB')).toBeInTheDocument()
  })

  it('меняет статус заказа через API с admin-origin и CSRF', async () => {
    document.cookie = 'csrf_admin=test-token'
    const fetchMock = withSession(
      vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
        const path = String(url)
        if (path.includes('/admin/orders/') && init?.method === 'PATCH') {
          return jsonResponse({ ...order, status: 'priced' })
        }
        if (path.includes('/admin/orders')) return jsonResponse([order])
        return jsonResponse([])
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Заказы' }))
    expect(await screen.findByText('Заказ')).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('Статус'), 'priced')

    await waitFor(() => {
      const call = fetchMock.mock.calls.find(([, init]) => init?.method === 'PATCH')
      expect(call).toBeDefined()
      const headers = call?.[1]?.headers as Record<string, string>
      expect(headers['X-App-Origin']).toBe('admin')
      expect(headers['X-CSRF-Token']).toBe('test-token')
      expect(String(call?.[1]?.body)).toContain('"status":"priced"')
    })
  })

  it('не сохраняет пустую цену и сбрасывает переопределение', async () => {
    document.cookie = 'csrf_admin=test-token'
    const fetchMock = withSession(
      vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
        if (init?.method === 'DELETE') return Promise.resolve(new Response(null, { status: 204 }))
        if (String(url).endsWith('/api/v1/admin/store/prices')) return jsonResponse(prices)
        return jsonResponse([])
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Цены' }))
    const input = await screen.findByLabelText('Цена WOOD-OAK')
    await user.clear(input)
    await user.click(screen.getAllByRole('button', { name: 'Сохранить' })[0])
    expect(await screen.findByText('Укажите цену материала')).toBeInTheDocument()

    await user.type(input, '5000')
    await user.click(screen.getAllByRole('button', { name: 'Сохранить' })[0])
    await waitFor(() => expect(screen.getByText('Цена WOOD-OAK сохранена')).toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: 'Сбросить' }))
    expect(await screen.findByText(/возвращена к встроенной ставке/)).toBeInTheDocument()
    const resetCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'DELETE')
    expect(resetCall).toBeDefined()
  })

  it('преобразует ошибки API и обрабатывает пустой ответ', async () => {
    const fetchMock = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === 'DELETE') return Promise.resolve(new Response(null, { status: 204 }))
      return jsonResponse({ error: { code: 'invalid_input', message: 'Некорректное значение' } }, 422)
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(api.get('/bad')).rejects.toMatchObject({
      status: 422,
      code: 'invalid_input',
      message: 'Некорректное значение',
    })
    await expect(api.delete('/empty')).resolves.toBeUndefined()
  })

  it('выходит из панели и показывает форму входа', async () => {
    const fetchMock = vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      const path = String(url)
      if (path.endsWith('/api/v1/auth/logout') && init?.method === 'POST') {
        return Promise.resolve(new Response(null, { status: 204 }))
      }
      if (path.endsWith('/api/v1/auth/me')) return jsonResponse(user)
      if (path.includes('/admin/orders')) return jsonResponse([])
      if (path.includes('/public/materials')) return jsonResponse([])
      if (path.includes('/payment-tiers')) return jsonResponse([])
      return jsonResponse([])
    })
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Выйти' }))
    expect(await screen.findByRole('heading', { name: 'Вход в панель магазина' })).toBeInTheDocument()
  })

  it('показывает ошибку загрузки настроек', async () => {
    const fetchMock = withSession(
      vi.fn((url: RequestInfo | URL) => {
        const path = String(url)
        if (path.endsWith('/api/v1/admin/store/settings')) {
          return jsonResponse({ error: { code: 'internal', message: 'Настройки недоступны' } }, 500)
        }
        return jsonResponse([])
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Настройки' }))
    expect(await screen.findByText('Настройки недоступны')).toBeInTheDocument()
  })

  it('различает ошибки входа и сетевой сбой', () => {
    expect(loginErrorMessage(new ApiError(403, 'forbidden', 'forbidden'))).toContain('нет прав')
    expect(loginErrorMessage(new Error('network'))).toContain('соединение')
  })
})
