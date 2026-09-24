// Выбор детали марша в 3D (этап 2 «конструктор»).
//
// Меш приходит с PartRanges: каждой детали (ступень, косоур, площадка,
// элемент ограждения) соответствует диапазон треугольников [Start, End).
// Raycaster отдаёт faceIndex = индекс треугольника, поэтому деталь
// восстанавливается обратным поиском по диапазонам — без догадок и без
// отдельной геометрии на каждую ступень.

import type { MeshPartRange } from '../types'

export interface PartGroup {
  /** Начальный треугольник (включительно). */
  start: number
  /** Число треугольников группы. */
  count: number
  /** Номер тела (ступени) в модели. */
  solid: number
  /** Роль детали: tread, stringer, landing, railing_* … */
  role: string
}

export interface PickedPart {
  solid: number
  role: string
  /** Индекс группы в PartGroups — по нему подсвечивается материал. */
  groupIndex: number
}

const ROLE_LABELS: Record<string, string> = {
  tread: 'Ступень',
  riser: 'Подступенок',
  stringer: 'Косоур',
  landing: 'Площадка',
  railing_post: 'Балясина',
  railing_rail: 'Поручень',
  railing_baluster: 'Балясина',
  railing_newel: 'Столб',
}

/** Человекочитаемое имя детали: «Ступень 7», «Косоур», «Площадка». */
export function partLabel(solid: number, role: string): string {
  const base = ROLE_LABELS[role] ?? roleLabelFallback(role)
  if (role === 'tread' || role === 'riser') return `${base} ${solid + 1}`
  return base
}

function roleLabelFallback(role: string): string {
  if (role.startsWith('railing')) return 'Ограждение'
  return role ? role[0].toUpperCase() + role.slice(1) : 'Деталь'
}

/** PartRanges → группы с вычисленным count. */
export function groupsFromRanges(ranges: MeshPartRange[] | undefined | null): PartGroup[] {
  if (!ranges?.length) return []
  return ranges
    .map((r) => ({
      start: Math.max(0, r.Start),
      count: Math.max(0, r.End - r.Start),
      solid: r.Solid,
      role: r.Role || 'stair',
    }))
    .filter((g) => g.count > 0)
}

/**
 * Индекс группы по номеру треугольника. Группы отсортированы по start
 * (порядок PartRanges совпадает с порядком геометрии), поэтому бинарный поиск.
 */
export function groupIndexForFace(groups: PartGroup[], faceIndex: number): number {
  let lo = 0
  let hi = groups.length - 1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    const g = groups[mid]
    if (faceIndex < g.start) {
      hi = mid - 1
    } else if (faceIndex >= g.start + g.count) {
      lo = mid + 1
    } else {
      return mid
    }
  }
  return -1
}

/** Деталь под лучом: null, если луч не попал ни в одну группу. */
export function pickPart(groups: PartGroup[], faceIndex: number): PickedPart | null {
  const groupIndex = groupIndexForFace(groups, faceIndex)
  if (groupIndex < 0) return null
  const g = groups[groupIndex]
  return { solid: g.solid, role: g.role, groupIndex }
}

/** Можно ли редактировать выбранную деталь (редактируются ступени марша). */
export function isEditablePart(role: string): boolean {
  return role === 'tread' || role === 'stringer' || role === 'landing'
}
