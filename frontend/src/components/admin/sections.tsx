// Реестр разделов кабинета: id раздела → панель со своей загрузкой.
//
// Зачем он: раньше AdminPanel грузил шесть ручек одним Promise.all и склеивал
// 12 секций в один скролл, а StorePanel поверх — вторым уровнем вкладок. В
// плиточной модели у каждого раздела своя загрузка: плитка «Команда» не
// должна дёргать ручку цен, а «Продажи и деньги» — ручку ключей.
//
// Список id обязан совпадать с реестром плиток (shared/src/cabinetTiles.ts):
// тест сверяет их, чтобы раздел не оказался «нарисованным, но не нарисованным».
import { useCallback, useEffect, useState, type ReactNode } from 'react'

import { hasCapability, P, type GrantsSubject } from '@shared/cabinetPermissions'
import type { ApiKey, AdminOverview, AdminPolicy, AdminUser, OrderDTO, TestimonialDTO } from '@shared/types'

import { adminApi } from '../../api/admin'
import { analyticsApi } from '../../api/analytics'
import { useAuth } from '../../auth/context'
import { useLoad } from '../../lib/useLoad'

import {
  CostAnalyticsPanel,
  FunnelAnalyticsPanel,
  ManufacturingPanel,
  ProjectsAnalyticsPanel,
  UsageAnalyticsPanel,
} from './AnalyticsPanel'
import { ApiKeysPanel } from './ApiKeysPanel'
import { ExportPanel } from './ExportPanel'
import { OrdersPanel } from './OrdersPanel'
import { OverviewPanel } from './OverviewPanel'
import { PolicyPanel } from './PolicyPanel'
import { TestimonialsPanel } from './TestimonialsPanel'
import { UsersPanel } from './UsersPanel'
import { AuditSection } from './AuditSection'
import { MaterialsSection, ServicesSection } from './store/Catalogs'
import { PaymentsSection } from './store/Payments'
import { FastenersSection } from './store/FastenersSection'
import { PricesSection } from './store/PricesSettings'
import { SealantsSection } from './store/SealantsSection'
import { RatesSection } from './store/RatesSection'
import { SiteSettingsSection, type SettingsGroupId } from './store/SiteSettingsSection'

/** Загрузка/ошибка для раздела: единое место, а не пять копий try/catch. */
export function SectionShell({
  loading,
  error,
  empty,
  children,
}: {
  loading: boolean
  error: string
  empty?: boolean
  children: ReactNode
}) {
  if (error) {
    return (
      <p className="alert alert--error" role="alert">
        {error}
      </p>
    )
  }
  if (loading) return <p className="muted">Загрузка…</p>
  if (empty) return <p className="muted">{children}</p>
  return <>{children}</>
}

function errText(e: unknown, fallback: string): string {
  return e instanceof Error && e.message ? e.message : fallback
}

/** Окно отчёта по умолчанию: последние 30 дней, как было в админке. */
function last30Days(): { from: string; to: string } {
  const now = new Date()
  const from = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
  return { from: from.toISOString().slice(0, 10), to: now.toISOString().slice(0, 10) }
}

/** Список + мутации: общая заготовка для сотрудников, ключей и отзывов. */
function useResource<T>(load: () => Promise<T[]>, errorText0: string) {
  const [items, setItems] = useState<T[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const reload = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setItems(await load())
    } catch (e) {
      setError(errText(e, errorText0))
    } finally {
      setLoading(false)
    }
  }, [load, errorText0])

  useEffect(() => {
    void reload()
  }, [reload])

  return { items, setItems, loading, error, setError, notice, setNotice, reload }
}

// ---- Команда ----

function StaffSummary() {
  const { data, error, loading } = useLoad(() => adminApi.overview(), [])
  return (
    <SectionShell loading={loading && !data} error={error}>
      {data && <OverviewPanel overview={data} />}
    </SectionShell>
  )
}

function StaffUsers() {
  const { user } = useAuth()
  const res = useResource(() => adminApi.listUsers(), 'Не удалось загрузить сотрудников')

  const update = async (id: string, body: Parameters<typeof adminApi.updateUser>[1]) => {
    res.setError('')
    res.setNotice('')
    try {
      await adminApi.updateUser(id, body)
      res.setNotice('Сотрудник обновлён')
      await res.reload()
    } catch (e) {
      res.setError(errText(e, 'Не удалось обновить сотрудника'))
    }
  }

  // Кнопка удаления появляется только при staff.delete: метрика действия, и
  // выдать её можно осознанно (по умолчанию есть лишь у владельца).
  const canDelete = hasCapability(user, 'staff.delete', 'action')

  const previewDelete = (id: string) => adminApi.userDeletionPreview(id)

  const remove = async (id: string, transferTo: string) => {
    res.setError('')
    res.setNotice('')
    try {
      await adminApi.deleteUser(id, transferTo)
      res.setNotice('Сотрудник удалён, проекты переданы')
      await res.reload()
    } catch (e) {
      res.setError(errText(e, 'Не удалось удалить сотрудника'))
    }
  }

  return (
    <>
      {res.notice && <p className="ok-text">{res.notice}</p>}
      <SectionShell loading={res.loading && res.items.length === 0} error={res.error} empty>
        {res.items.length === 0 ? (
          'Сотрудников пока нет.'
        ) : (
          <UsersPanel
            users={res.items}
            currentUserId={user?.id ?? ''}
            onUpdate={update}
            canDelete={canDelete}
            onPreviewDelete={previewDelete}
            onDelete={remove}
          />
        )}
      </SectionShell>
    </>
  )
}

function StaffPolicy() {
  const { data: policy, error, loading } = useLoad(() => adminApi.getSettings(), [])
  const [draft, setDraft] = useState<AdminPolicy | null>(null)
  const [notice, setNotice] = useState('')
  const [saveError, setSaveError] = useState('')

  useEffect(() => {
    if (policy) setDraft(policy)
  }, [policy])

  const save = async () => {
    if (!draft) return
    setSaveError('')
    setNotice('')
    try {
      await adminApi.updateSettings(draft)
      setNotice('Политика сохранена')
    } catch (e) {
      setSaveError(errText(e, 'Не удалось сохранить политику'))
    }
  }

  return (
    <>
      {notice && <p className="ok-text">{notice}</p>}
      {saveError && (
        <p className="alert alert--error" role="alert">
          {saveError}
        </p>
      )}
      <SectionShell loading={loading && !draft} error={error}>
        {draft && (
          <PolicyPanel
            policy={draft}
            onChange={(key, value) => setDraft((p) => (p ? { ...p, [key]: value } : p))}
            onSave={save}
          />
        )}
      </SectionShell>
    </>
  )
}

function StaffKeys() {
  const res = useResource(() => adminApi.listApiKeys(), 'Не удалось загрузить ключи')
  return (
    <>
      {res.notice && <p className="ok-text">{res.notice}</p>}
      <SectionShell loading={res.loading && res.items.length === 0} error={res.error}>
        <ApiKeysPanel
          keys={res.items}
          onRefresh={res.reload}
          onError={res.setError}
          onNotice={res.setNotice}
        />
      </SectionShell>
    </>
  )
}

function StaffAudit() {
  return <AuditSection />
}

// ---- Сайт и витрина ----

function SiteReviews() {
  const res = useResource(() => adminApi.listTestimonials(), 'Не удалось загрузить отзывы')
  return (
    <>
      {res.notice && <p className="ok-text">{res.notice}</p>}
      <SectionShell loading={res.loading && res.items.length === 0} error={res.error}>
        <TestimonialsPanel
          testimonials={res.items}
          onRefresh={res.reload}
          onError={res.setError}
          onNotice={res.setNotice}
        />
      </SectionShell>
    </>
  )
}

// ---- Продажи и деньги ----

function FinanceSales() {
  const { user } = useAuth()
  const res = useResource(() => adminApi.listOrders(), 'Не удалось загрузить заказы')

  const changeStatus = async (id: string, status: string) => {
    res.setError('')
    res.setNotice('')
    try {
      await adminApi.updateOrderStatus(id, status)
      res.setNotice('Статус заказа обновлён')
      await res.reload()
    } catch (e) {
      // Раньше этот обработчик отбрасывал промис: при 403 статус молча не
      // менялся, и человек не видел ничего. Теперь ошибка видна.
      res.setError(errText(e, 'Не удалось изменить статус заказа'))
    }
  }

  return (
    <>
      {res.notice && <p className="ok-text">{res.notice}</p>}
      <SectionShell loading={res.loading && res.items.length === 0} error={res.error}>
        <OrdersPanel
          orders={res.items}
          canManage={hasCapability(user as GrantsSubject | null, P.financeSales, 'write')}
          onStatusChange={changeStatus}
        />
      </SectionShell>
    </>
  )
}

function FinancePayments() {
  const { user } = useAuth()
  return (
    <PaymentsSection canRefund={hasCapability(user as GrantsSubject | null, P.financeRefunds, 'action')} />
  )
}

function FinanceExport() {
  return <ExportPanel onExport={(scope, format) => { window.location.href = adminApi.exportUrl(scope, format) }} />
}

// ---- Аналитика ----

/**
 * Загрузка отчёта по датам. Панели сами рисуют «Загрузка…» и пустые
 * состояния, поэтому контейнер только держит данные и ошибку.
 */
function useReport<T>(load: () => Promise<T>) {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const run = useCallback(() => {
    let alive = true
    setLoading(true)
    setError('')
    load()
      .then((res) => {
        if (alive) setData(res)
      })
      .catch((e: unknown) => {
        if (alive) setError(errText(e, 'Не удалось загрузить отчёт'))
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [load])

  useEffect(() => run(), [run])

  return { data, loading, error }
}

function ReportError({ error }: { error: string }) {
  if (!error) return null
  return (
    <p className="alert alert--error" role="alert">
      {error}
    </p>
  )
}

function TrafficUsage() {
  const { data, loading, error } = useReport(() => analyticsApi.usage(last30Days()))
  return (
    <>
      <ReportError error={error} />
      <UsageAnalyticsPanel
        usage={data}
        loading={loading}
        granularity="day"
        onChangeGranularity={() => {}}
      />
    </>
  )
}

function FinanceProjects() {
  const { data, loading, error } = useReport(() => analyticsApi.projects(last30Days()))
  return (
    <>
      <ReportError error={error} />
      <ProjectsAnalyticsPanel projects={data} loading={loading} />
    </>
  )
}

function ProductionReport() {
  const { data, loading, error } = useReport(() => analyticsApi.manufacturing(last30Days()))
  return (
    <>
      <ReportError error={error} />
      <ManufacturingPanel
        mfg={data}
        loading={loading}
        granularity="day"
        onChangeGranularity={() => {}}
      />
    </>
  )
}

function FinanceCost() {
  const { data, loading, error } = useReport(() => analyticsApi.cost(last30Days()))
  return (
    <>
      <ReportError error={error} />
      <CostAnalyticsPanel
        cost={data}
        loading={loading}
        granularity="day"
        onChangeGranularity={() => {}}
      />
    </>
  )
}

function TrafficFunnel() {
  const { data, loading, error } = useReport(() => analyticsApi.funnel(last30Days()))
  return (
    <>
      <ReportError error={error} />
      <FunnelAnalyticsPanel funnel={data} loading={loading} />
    </>
  )
}

// ---- Реестр ----

export interface SectionRenderProps {
  /** Кто смотрит: разделы сами решают, показывать ли кнопку изменения. */
  user: GrantsSubject | null
}

export type SectionRenderer = (props: SectionRenderProps) => ReactNode

/** SECTION_RENDERERS: id раздела → панель. Ключи сверяются с плитками. */
export const SECTION_RENDERERS: Record<string, SectionRenderer> = {
  // Команда
  'staff-summary': () => <StaffSummary />,
  'staff-users': () => <StaffUsers />,
  'staff-policy': () => <StaffPolicy />,
  'staff-keys': () => <StaffKeys />,
  'staff-audit': () => <StaffAudit />,
  // Сайт и витрина
  'site-contacts': () => <SiteSettingsSection group="contacts" />,
  'site-company': () => <SiteSettingsSection group="company" />,
  'site-social': () => <SiteSettingsSection group="social" />,
  'site-seo': () => <SiteSettingsSection group="seo" />,
  'site-reviews': () => <SiteReviews />,
  'site-materials': () => <MaterialsSection />,
  // Цены и расчёт
  'price-materials': () => <PricesSection />,
  'price-fasteners': () => <FastenersSection />,
  'price-sealant': () => <SealantsSection />,
  'price-rates': () => <RatesSection />,
  'price-services': () => <ServicesSection />,
  // Продажи и деньги
  'finance-sales': () => <FinanceSales />,
  'finance-payments': () => <FinancePayments />,
  'finance-cost': () => <FinanceCost />,
  'finance-export': () => <FinanceExport />,
  'finance-projects': () => <FinanceProjects />,
  // Производство
  'production-report': () => <ProductionReport />,
  // Реклама и трафик
  'traffic-funnel': () => <TrafficFunnel />,
  'traffic-usage': () => <TrafficUsage />,
}

export const SECTION_IDS = Object.keys(SECTION_RENDERERS)

/** Панель раздела; null — если раздел неизвестен (опечатка в реестре). */
export function renderSection(id: string, props: SectionRenderProps): ReactNode {
  const renderer = SECTION_RENDERERS[id]
  return renderer ? renderer(props) : null
}

export type { SettingsGroupId }
export type { AdminOverview, AdminUser, ApiKey, OrderDTO, TestimonialDTO }