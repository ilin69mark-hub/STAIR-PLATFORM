/**
 * Права и разделы кабинета — одна карта вместо проверки роли по месту.
 *
 * Раздел показывается по наличию метрики из `User.grants` (приходит с
 * бэкенда, `GET /auth/me`). Реестр метрик живёт в
 * `internal/application/auth/capability.go` и приезжает готовым: клиент не
 * дублирует матрицу ролей, он только спрашивает «хватает ли уровня».
 *
 * Правило fail-closed: нет метрики — блок не рендерится вообще, а не
 * «прячется кнопкой». Пустой `grants` означает «владелец ничего не выдал»,
 * и это законное состояние, а не ошибка.
 */

/**
 * Уровни метрики. Повторяют серверную модель:
 *  - `read` — видеть;
 *  - `write` — видеть и менять;
 *  - `action` — выполнить действие (возврат, выдача ключа). Отдельный
 *    уровень, потому что это поступок, а не правка данных: его нельзя
 *    получить вместе с чтением.
 */
export type Level = 'read' | 'write' | 'action'

/**
 * Метрики, которыми закрываются разделы. Строки совпадают с Capability в Go
 * (internal/application/auth/capability.go).
 *
 * Смысл разделения на «видеть» и «сделать» — в том, что переключатель у
 * кнопки появляется только при своём уровне: показать «Оформить возврат»
 * человеку без finance.refunds значило бы предложить действие, которое
 * сервер отобьёт.
 */
export const P = {
  // Команда
  staffView: 'staff.view',
  staffCreate: 'staff.create',
  staffStatus: 'staff.status',
  staffRole: 'staff.role',
  staffGrants: 'staff.grants',
  staffDelete: 'staff.delete',
  staffDeleteHard: 'staff.delete_hard',
  staffProjectTransfer: 'staff.project_transfer',
  securityPolicy: 'security.policy',
  securityKeys: 'security.keys',
  securityKeysIssue: 'security.keys_issue',
  auditView: 'audit.view',
  // Финансы
  financeSales: 'finance.sales',
  financePayments: 'finance.payments',
  financeRefunds: 'finance.refunds',
  financeCost: 'finance.cost',
  dataExport: 'data.export',
  // Цены
  priceMaterials: 'price.materials',
  priceRates: 'price.rates',
  priceServices: 'price.services',
  // Сайт
  siteContacts: 'site.contacts',
  siteCompany: 'site.company',
  siteSocial: 'site.social',
  siteSEO: 'site.seo',
  siteReviews: 'site.reviews',
  siteMaterials: 'site.materials',
  // Производство
  productionReport: 'production.report',
  productionPlan: 'production.plan',
  // Трафик
  trafficFunnel: 'traffic.funnel',
  trafficUsage: 'traffic.usage',
  // Проекты. Доступ к проектам персональный (project_members), поэтому
  // отдельной метрики «видеть проекты» нет — нечего ограничивать.
  projectReport: 'project.report',
  // Интеграции
  integrationEndpoints: 'integration.endpoints',
  integrationEndpointsIssue: 'integration.endpoints_issue',
  storageDelete: 'storage.delete',
} as const

export type Capability = (typeof P)[keyof typeof P]

/** Пользователь в том виде, в каком приходит с бэкенда. */
export interface GrantsSubject {
  grants?: { capability: string; level: Level }[]
  permissions?: string[]
}

/**
 * hasCapability — хватает ли пользователю метрики на этом уровне.
 *
 * `write` подразумевает `read` (так же, как на сервере). Если метрика
 * есть, а уровень не action, сравниваем как с записью в grants.
 */
export function hasCapability(user: GrantsSubject | null, cap: Capability, level: Level = 'read'): boolean {
  if (!user) return false
  if (!cap) return false

  const grants = user.grants
  if (!grants) {
    // Старая сессия без grants: падаем на список метрик. Уровень там не
    // различался, поэтому подходит любой запрос кроме action — тот
    // требовал бы подтверждения, а его сервер всё равно проверит.
    const has = (user.permissions ?? []).includes(cap)
    if (!has) return false
    return level !== 'action'
  }

  const found = grants.find((g) => g.capability === cap)
  if (!found) return false
  if (found.level === 'write') return level === 'read' || level === 'write'
  return found.level === level
}

/**
 * hasPermission — есть ли метрика вообще, в любом уровне. Нужна для
 * показа блока: «видно, что метрика есть» достаточно, чтобы нарисовать
 * раздел; уровень проверяется на кнопке.
 */
export function hasPermission(user: GrantsSubject | null, needs: Capability | null): boolean {
  if (needs === null) return true
  if (!user) return false
  if (user.grants) return user.grants.some((g) => g.capability === needs)
  return (user.permissions ?? []).includes(needs)
}
