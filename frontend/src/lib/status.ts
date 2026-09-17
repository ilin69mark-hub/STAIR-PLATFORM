import type { ProjectStatus } from '@shared/types'

interface StatusMeta {
  label: string
  badge: string
  color: string
  next: string
}

// statusMeta — единый источник правды для статусов проекта: русская
// подпись, CSS-класс бейджа, цвет левой полосы карточки и подсказка
// «следующее действие» для status-card в сайдбаре.
const STATUS_META: Record<ProjectStatus, StatusMeta> = {
  draft: {
    label: 'Черновик',
    badge: 'badge--draft',
    color: 'var(--status-draft)',
    next: 'Заполните параметры и выполните расчёт.',
  },
  in_review: {
    label: 'На ревью',
    badge: 'badge--review',
    color: 'var(--status-review)',
    next: 'Ожидается решение владельца проекта.',
  },
  changes_requested: {
    label: 'Доработка',
    badge: 'badge--changes',
    color: 'var(--status-changes)',
    next: 'Внесите правки и запросите ревью повторно.',
  },
  approved: {
    label: 'Подписано',
    badge: 'badge--approved',
    color: 'var(--status-approved)',
    next: 'Конфигурация утверждена и готова к производству.',
  },
}

const FALLBACK: StatusMeta = {
  label: 'Черновик',
  badge: 'badge--draft',
  color: 'var(--status-draft)',
  next: 'Заполните параметры и выполните расчёт.',
}

export function statusMeta(status: string): StatusMeta {
  return STATUS_META[status as ProjectStatus] ?? FALLBACK
}