import { useCallback, useEffect, useState } from 'react'
import { adminApi } from '../api/admin'
import { analyticsApi } from '../api/analytics'
import type {
  AdminOverview,
  AdminPolicy,
  AdminUser,
  ApiKey,
  CostReport,
  ManufacturingReport,
  OrderDTO,
  ProjectReport,
  TestimonialDTO,
  UsageGranularity,
  UsageReport,
} from '@shared/types'
import { ApiError } from '@shared/types'

import { OverviewPanel } from './admin/OverviewPanel'
import { UsersPanel } from './admin/UsersPanel'
import { PolicyPanel } from './admin/PolicyPanel'
import { ExportPanel } from './admin/ExportPanel'
import { UsageAnalyticsPanel, ProjectsAnalyticsPanel, ManufacturingPanel, CostAnalyticsPanel } from './admin/AnalyticsPanel'
import { OrdersPanel } from './admin/OrdersPanel'
import { ApiKeysPanel } from './admin/ApiKeysPanel'
import { TestimonialsPanel } from './admin/TestimonialsPanel'
import { useScrollSpy } from '../lib/useScrollSpy'
import { LazySection } from './admin/LazySection'

// ADMIN_SECTIONS — оглавление админ-панели (sticky TOC).
const ADMIN_SECTIONS = [
  { id: 'overview', label: 'Обзор' },
  { id: 'users', label: 'Пользователи' },
  { id: 'policy', label: 'Политика' },
  { id: 'export', label: 'Экспорт' },
  { id: 'usage', label: 'Использование' },
  { id: 'projects', label: 'Проекты' },
  { id: 'orders', label: 'Заказы' },
  { id: 'manufacturing', label: 'Производство' },
  { id: 'cost', label: 'Стоимость' },
  { id: 'api-keys', label: 'API-ключи' },
  { id: 'testimonials', label: 'Отзывы' },
]

const ADMIN_SECTION_IDS = ADMIN_SECTIONS.map((s) => s.id)

interface Props {
  currentUserId: string
  onBack: () => void
}

const emptyPolicy: AdminPolicy = {
  min_password_length: 8,
  require_number: false,
  require_upper: false,
  session_ttl_seconds: 86400,
  login_rate_limit_per_min: 10,
}

export function AdminPanel({ currentUserId, onBack }: Props) {
  const [overview, setOverview] = useState<AdminOverview | null>(null)
  const [users, setUsers] = useState<AdminUser[]>([])
  const [policy, setPolicy] = useState<AdminPolicy>(emptyPolicy)
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [orders, setOrders] = useState<OrderDTO[]>([])
  const [testimonials, setTestimonials] = useState<TestimonialDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const [usage, setUsage] = useState<UsageReport | null>(null)
  const [usageGranularity, setUsageGranularity] = useState<UsageGranularity>('day')
  const [usageLoading, setUsageLoading] = useState(false)

  const [projects, setProjects] = useState<ProjectReport | null>(null)
  const [projectsLoading, setProjectsLoading] = useState(false)

  const [mfg, setMfg] = useState<ManufacturingReport | null>(null)
  const [mfgGranularity, setMfgGranularity] = useState<UsageGranularity>('day')
  const [mfgLoading, setMfgLoading] = useState(false)

  const [cost, setCost] = useState<CostReport | null>(null)
  const [costGranularity, setCostGranularity] = useState<UsageGranularity>('day')
  const [costLoading, setCostLoading] = useState(false)

  const loadUsage = useCallback(async (granularity: UsageGranularity) => {
    setUsageLoading(true)
    try {
      const now = new Date()
      const from = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
      const rep = await analyticsApi.usage({
        from: from.toISOString().slice(0, 10),
        to: now.toISOString().slice(0, 10),
        granularity,
      })
      setUsage(rep)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить аналитику')
    } finally {
      setUsageLoading(false)
    }
  }, [])

  const loadProjects = useCallback(async () => {
    setProjectsLoading(true)
    try {
      const now = new Date()
      const from = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
      const rep = await analyticsApi.projects({
        from: from.toISOString().slice(0, 10),
        to: now.toISOString().slice(0, 10),
      })
      setProjects(rep)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить аналитику проектов')
    } finally {
      setProjectsLoading(false)
    }
  }, [])

  const loadManufacturing = useCallback(async (granularity: UsageGranularity) => {
    setMfgLoading(true)
    try {
      const now = new Date()
      const from = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
      const rep = await analyticsApi.manufacturing({
        from: from.toISOString().slice(0, 10),
        to: now.toISOString().slice(0, 10),
        granularity,
      })
      setMfg(rep)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить аналитику производства')
    } finally {
      setMfgLoading(false)
    }
  }, [])

  const loadCost = useCallback(async (granularity: UsageGranularity) => {
    setCostLoading(true)
    try {
      const now = new Date()
      const from = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
      const rep = await analyticsApi.cost({
        from: from.toISOString().slice(0, 10),
        to: now.toISOString().slice(0, 10),
        granularity,
      })
      setCost(rep)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить аналитику стоимости')
    } finally {
      setCostLoading(false)
    }
  }, [])

  const load = useCallback(async (silent = false) => {
    // silent=true — фоновое обновление данных после изменений: не меняем
    // панель на скелетон, чтобы не терять состояние (например, одноразовый
    // токен API-ключа) и не мигать интерфейсом.
    if (!silent) setLoading(true)
    setError(null)
    try {
      const [ov, us, pl, ks, ordersResp, tms] = await Promise.all([
        adminApi.overview(),
        adminApi.listUsers(),
        adminApi.getSettings(),
        adminApi.listApiKeys(),
        adminApi.listOrders(),
        adminApi.listTestimonials(),
      ])
      setOverview(ov)
      setUsers(us)
      setPolicy(pl)
      setKeys(ks)
      setOrders(ordersResp)
      setTestimonials(tms)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить панель администратора')
    } finally {
      if (!silent) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const activeSection = useScrollSpy(ADMIN_SECTION_IDS, 'overview', [loading])

  const handleUpdateUser = async (id: string, body: { role?: 'user' | 'admin'; status?: 'active' | 'disabled' }) => {
    setError(null)
    setNotice(null)
    try {
      await adminApi.updateUser(id, body)
      setNotice('Пользователь обновлён')
      void load(true)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось обновить пользователя')
    }
  }

  const handleSavePolicy = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setNotice(null)
    try {
      await adminApi.updateSettings(policy)
      setNotice('Политика безопасности сохранена')
      void load(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сохранить политику')
    }
  }

  const setPolicyField = (key: keyof AdminPolicy, value: string | boolean | number) =>
    setPolicy((p) => ({ ...p, [key]: value as never }))

  const handleExport = (scope: string, format: 'json' | 'csv') => {
    window.location.href = adminApi.exportUrl(scope, format)
  }

  const handleOrderStatus = async (id: string, status: string) => {
    setError(null)
    setNotice(null)
    try {
      await adminApi.updateOrderStatus(id, status)
      setNotice('Статус заказа обновлён')
      void load(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось обновить статус заказа')
    }
  }

  return (
    <div className="page">
      <header className="page__header page__header--stacked">
        <div className="page__header-title">
          <button className="btn btn--ghost" onClick={onBack}>
            ← Проекты
          </button>
          <h1 className="page__title">Администрирование</h1>
          <p className="page__subtitle">Enterprise Controls · EDR-0016</p>
        </div>
      </header>

      {error && <div className="alert alert--error">{error}</div>}
      {notice && <div className="alert alert--ok">{notice}</div>}

      {loading ? (
        <div className="stack">
          <div className="skeleton skeleton--card" />
          <div className="skeleton skeleton--card" />
          <div className="skeleton skeleton--card" />
        </div>
      ) : (
        <div className="page__content">
          <div className="page__main stack">
            <section id="overview" className="section">
              <OverviewPanel overview={overview} />
            </section>
            <section id="users" className="section">
              <UsersPanel users={users} currentUserId={currentUserId} onUpdate={handleUpdateUser} />
            </section>
            <section id="policy" className="section">
              <PolicyPanel policy={policy} onChange={setPolicyField} onSave={handleSavePolicy} />
            </section>
            <section id="export" className="section">
              <ExportPanel onExport={handleExport} />
            </section>
            <section id="usage" className="section">
              <LazySection onLoad={() => void loadUsage(usageGranularity)}>
                <UsageAnalyticsPanel
                  usage={usage}
                  loading={usageLoading}
                  granularity={usageGranularity}
                  onChangeGranularity={(g) => { setUsageGranularity(g); void loadUsage(g) }}
                />
              </LazySection>
            </section>
            <section id="projects" className="section">
              <LazySection onLoad={() => void loadProjects()}>
                <ProjectsAnalyticsPanel projects={projects} loading={projectsLoading} />
              </LazySection>
            </section>
            <section id="orders" className="section">
              <OrdersPanel orders={orders} onStatusChange={handleOrderStatus} />
            </section>
            <section id="manufacturing" className="section">
              <LazySection onLoad={() => void loadManufacturing(mfgGranularity)}>
                <ManufacturingPanel
                  mfg={mfg}
                  loading={mfgLoading}
                  granularity={mfgGranularity}
                  onChangeGranularity={(g) => { setMfgGranularity(g); void loadManufacturing(g) }}
                />
              </LazySection>
            </section>
            <section id="cost" className="section">
              <LazySection onLoad={() => void loadCost(costGranularity)}>
                <CostAnalyticsPanel
                  cost={cost}
                  loading={costLoading}
                  granularity={costGranularity}
                  onChangeGranularity={(g) => { setCostGranularity(g); void loadCost(g) }}
                />
              </LazySection>
            </section>
            <section id="api-keys" className="section">
              <ApiKeysPanel keys={keys} onRefresh={() => void load(true)} onError={setError} onNotice={setNotice} />
            </section>
            <section id="testimonials" className="section">
              <TestimonialsPanel testimonials={testimonials} onRefresh={() => void load(true)} onError={setError} onNotice={setNotice} />
            </section>
          </div>

          <aside className="page__sidebar">
            <nav className="toc" aria-label="Разделы администратора">
              <div className="toc__title">Навигация</div>
              <ul className="toc__list">
                {ADMIN_SECTIONS.map((s) => (
                  <li key={s.id}>
                    <a
                      className={`toc__link${activeSection === s.id ? ' toc__link--active' : ''}`}
                      href={`#${s.id}`}
                    >
                      {s.label}
                    </a>
                  </li>
                ))}
              </ul>
            </nav>
          </aside>
        </div>
      )}
    </div>
  )
}
