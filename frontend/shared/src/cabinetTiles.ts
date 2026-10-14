/**
 * Плитки приветственного экрана кабинета.
 *
 * Плитка — это блок бизнеса («Сайт и витрина», «Продажи и деньги»), внутри
 * которого живут разделы. Раньше кабинет делился на вкладки
 * «Администрирование / Аудит / Магазин», и блоки бизнеса попадали внутрь
 * вкладок по техническим причинам: производство и себестоимость оказывались
 * рядом с сотрудниками и политикой безопасности.
 *
 * Что меняется для человека: он видит не то, как устроена система, а за что
 * он отвечает. Кнопки «выдать права» или «сменить статус заказа» больше не
 * нужны, чтобы добраться до отчёта по производству.
 *
 * Видимость плитки считается по наличию ХОТЯ БЫ ОДНОЙ метрики внутри неё
 * (allowedTiles), а не по отдельному праву на плитку: заводить 37-е право
 * «открыть плитку Производство» только для того, чтобы показать её, —
 * дублирование того же факта, который уже выражен метриками разделов.
 *
 * Реестр зеркалит блоки бэкенда (internal/application/auth/capability.go,
 * BlockSite и соседние, CapabilityBlocks): порядок здесь и порядок плиток
 * на экране один, иначе матрица прав и главный экран разъедутся.
 */
import { P, type Capability, hasPermission, type GrantsSubject } from './cabinetPermissions'

/**
 * Каталоги материалов и услуг бьют в публичные эндпоинты, поэтому метрики на
 * них нет. Показывать их можно всем — но тогда плитка «Сайт и витрина»
 * светилась бы у каждого (см. allowedTiles), а метрики site.materials и
 * price.services остались бы в реестре без дома. Поэтому разделы-каталоги
 * закрыты своими метриками: сервер отдаёт данные и без прав, но владелец
 * решает, показывать ли их на экране сотрудника.
 *
 * Отложенные метрики: объявлены в реестре (бэкенд их знает и может выдать),
 * но экрана пока нет. Перечислены явно, чтобы тест «каждая метрика открывает
 * раздел» ловил новые неучтённые, а не молчал.
 *
 *  - staff.create / staff.status / staff.role / staff.grants / staff.delete /
 *    staff.delete_hard / staff.project_transfer — матрица прав сотрудника и
 *    перевешивание проектов (следующие задачи).
 *  - security.keys_issue — кнопка внутри раздела «API-ключи», не свой раздел.
 *  - integration.endpoints_issue, storage.delete — экран интеграций.
 *  - production.plan — задания в производство: бэкенда нет.
 */
export const DEFERRED_CAPABILITIES: ReadonlySet<string> = new Set([
  P.staffCreate,
  P.staffStatus,
  P.staffRole,
  P.staffGrants,
  P.staffDelete,
  P.staffDeleteHard,
  P.staffProjectTransfer,
  P.securityKeysIssue,
  P.integrationEndpointsIssue,
  P.storageDelete,
  // Интеграции (ERP/MES): эндпоинты и хранилище живут, экран не нарисован.
  P.integrationEndpoints,
  P.productionPlan,
  // Кнопка внутри раздела «Платежи»: возврат виден только с правом и не
  // выдаётся отдельным разделом.
  P.financeRefunds,
])

export interface CabinetSection {
  id: string
  title: string
  /** Метрика, без которой раздел не показывается. null — доступен всем. */
  needs: Capability | null
}

export interface CabinetTile {
  id: string
  title: string
  /** Короткая подпись под названием: чем этот блок занимается. */
  hint: string
  sections: CabinetSection[]
}

/**
 * Плитки в порядке показа. «Проекты» идёт первой и без прав: это рабочее
 * место человека, а не администрирование. Владелец видит только свои
 * проекты (backend отдаёт проекты, где вызывающий — участник), обзор по
 * всей компании живёт в разделе «Сводка по проектам» плитки «Продажи и деньги».
 */
export const CABINET_TILES: CabinetTile[] = [
  {
    id: 'projects',
    title: 'Проекты',
    hint: 'Расчёты лестниц, чертежи, комментарии',
    sections: [{ id: 'projects-list', title: 'Мои проекты', needs: null }],
  },
  {
    id: 'site',
    title: 'Сайт и витрина',
    hint: 'Что видит покупатель: контакты, реквизиты, тексты, отзывы',
    sections: [
      { id: 'site-contacts', title: 'Контакты и адрес', needs: P.siteContacts },
      { id: 'site-company', title: 'Реквизиты компании', needs: P.siteCompany },
      { id: 'site-social', title: 'Соцсети', needs: P.siteSocial },
      { id: 'site-seo', title: 'SEO и счётчики', needs: P.siteSEO },
      { id: 'site-reviews', title: 'Отзывы', needs: P.siteReviews },
      { id: 'site-materials', title: 'Каталог материалов', needs: P.siteMaterials },
    ],
  },
  {
    id: 'pricing',
    title: 'Цены и расчёт',
    hint: 'Сколько стоит материал и по каким правилам считаем',
    sections: [
      { id: 'price-materials', title: 'Цены материалов', needs: P.priceMaterials },
      // Крепёж (MNT-0001): своя плитка, потому что единица другая (₽/шт) и
      // позиций около сорока. В «Ценах материалов» он был бы 45-й строкой
      // с неясной графой «цена».
      { id: 'price-fasteners', title: 'Крепёж', needs: P.priceMaterials },
      // Герметик (CONF-SEALANT) — та же логика, но единица закупки ₽/картридж.
      { id: 'price-sealant', title: 'Герметик', needs: P.priceMaterials },
      // Ограждение (CONF-PURCHASED): закупочный прайс покупных деталей,
      // ₽/шт по ВИДУ детали. Своя плитка потому, что до этого прайса смета
      // стоила без ограждения — 29 позиций с нулевой суммой.
      { id: 'price-purchased', title: 'Ограждение', needs: P.priceMaterials },
      { id: 'price-rates', title: 'Ставки расчёта', needs: P.priceRates },
      { id: 'price-services', title: 'Услуги и тарифы', needs: P.priceServices },
    ],
  },
  {
    id: 'finance',
    title: 'Продажи и деньги',
    hint: 'Заказы, платежи, себестоимость и прибыль',
    sections: [
      { id: 'finance-sales', title: 'Заказы и продажи', needs: P.financeSales },
      { id: 'finance-payments', title: 'Платежи', needs: P.financePayments },
      { id: 'finance-cost', title: 'Себестоимость и прибыль', needs: P.financeCost },
      { id: 'finance-export', title: 'Выгрузка данных', needs: P.dataExport },
      { id: 'finance-projects', title: 'Сводка по проектам', needs: P.projectReport },
    ],
  },
  {
    id: 'production',
    title: 'Производство',
    hint: 'Раскрой, утилизация, отходы',
    sections: [
      { id: 'production-report', title: 'Раскрой, утилизация, отходы', needs: P.productionReport },
    ],
  },
  {
    id: 'traffic',
    title: 'Реклама и трафик',
    hint: 'Откуда приходят и где отваливаются',
    sections: [
      { id: 'traffic-funnel', title: 'Воронка', needs: P.trafficFunnel },
      { id: 'traffic-usage', title: 'Активность пользователей', needs: P.trafficUsage },
    ],
  },
  {
    id: 'staff',
    title: 'Команда',
    hint: 'Сотрудники, доступы, безопасность, журнал действий',
    sections: [
      { id: 'staff-summary', title: 'Сводка', needs: P.staffView },
      { id: 'staff-users', title: 'Сотрудники', needs: P.staffView },
      { id: 'staff-policy', title: 'Политика безопасности', needs: P.securityPolicy },
      { id: 'staff-keys', title: 'API-ключи', needs: P.securityKeys },
      { id: 'staff-audit', title: 'Журнал аудита', needs: P.auditView },
    ],
  },
]

/**
 * Плитки, доступные пользователю: есть хоть одна метрика из разделов.
 * Если метрика есть, но не у того раздела, плитка всё равно появится, а
 * раздел внутри неё — нет: это осознанно, иначе пришлось бы заводить право
 * на плитку отдельно от прав на её содержимое.
 */
export function allowedTiles(user: GrantsSubject | null): CabinetTile[] {
  if (!user) return []
  return CABINET_TILES.filter((tile) => {
    // Плитка без метрик (Проекты) открыта любому, кто вошёл.
    if (tile.sections.every((s) => s.needs === null)) return true
    // Остальным нужен хотя бы один раздел ЗА МЕТРИКОЙ. Иначе «Сайт и
    // витрина» светилась бы каждому из-за публичного каталога материалов,
    // хотя ни контактов, ни реквизитов, ни отзывов ему не выдавали.
    return allowedTileSections(user, tile).some((s) => s.needs !== null)
  })
}

/** Разделы плитки, доступные пользователю. */
export function allowedTileSections(user: GrantsSubject | null, tile: CabinetTile): CabinetSection[] {
  return tile.sections.filter((s) => hasPermission(user, s.needs))
}

/** Плитка по id; null — если такой плитки нет или она недоступна. */
export function findTile(user: GrantsSubject | null, id: string): CabinetTile | null {
  return allowedTiles(user).find((t) => t.id === id) ?? null
}

/** Первая доступная плитка — куда попасть, если выбранная закрыта. */
export function firstTileId(user: GrantsSubject | null): string | null {
  return allowedTiles(user)[0]?.id ?? null
}
