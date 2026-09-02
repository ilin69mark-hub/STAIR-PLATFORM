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

  const load = useCallback(async () => {
    setLoading(true)
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
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
    void loadUsage('day')
    void loadProjects()
    void loadManufacturing('day')
    void loadCost('day')
  }, [load, loadUsage, loadProjects, loadManufacturing, loadCost])

  const handleUpdateUser = async (id: string, body: { role?: 'user' | 'admin'; status?: 'active' | 'disabled' }) => {
    setError(null)
    setNotice(null)
    try {
      await adminApi.updateUser(id, body)
      setNotice('Пользователь обновлён')
      void load()
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
      void load()
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
      void load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось обновить статус заказа')
    }
  }

  return (
    <div className="page">
      <header className="page__header">
        <button className="btn btn--ghost" onClick={onBack}>
          ← Проекты
        </button>
        <div>
          <h1 className="page__title">Администрирование</h1>
          <p className="page__subtitle">Enterprise Controls · EDR-0016</p>
        </div>
      </header>

      {error && <div className="alert alert--error">{error}</div>}
      {notice && <div className="alert alert--ok">{notice}</div>}

      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <>
          <OverviewPanel overview={overview} />
          <UsersPanel users={users} currentUserId={currentUserId} onUpdate={handleUpdateUser} />
          <PolicyPanel policy={policy} onChange={setPolicyField} onSave={handleSavePolicy} />
          <ExportPanel onExport={handleExport} />
          <UsageAnalyticsPanel
            usage={usage}
            loading={usageLoading}
            granularity={usageGranularity}
            onChangeGranularity={(g) => { setUsageGranularity(g); void loadUsage(g) }}
          />
          <ProjectsAnalyticsPanel projects={projects} loading={projectsLoading} />
          <OrdersPanel orders={orders} onStatusChange={handleOrderStatus} />
          <ManufacturingPanel
            mfg={mfg}
            loading={mfgLoading}
            granularity={mfgGranularity}
            onChangeGranularity={(g) => { setMfgGranularity(g); void loadManufacturing(g) }}
          />
          <CostAnalyticsPanel
            cost={cost}
            loading={costLoading}
            granularity={costGranularity}
            onChangeGranularity={(g) => { setCostGranularity(g); void loadCost(g) }}
          />
          <ApiKeysPanel keys={keys} onRefresh={() => void load()} onError={setError} onNotice={setNotice} />
          <TestimonialsPanel testimonials={testimonials} onRefresh={() => void load()} onError={setError} onNotice={setNotice} />
        </>
      )}
    </div>
  )
}
