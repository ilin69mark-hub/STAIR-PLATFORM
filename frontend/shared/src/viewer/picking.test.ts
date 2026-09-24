import { describe, expect, it } from 'vitest'
import {
  dragAxisIsHorizontal,
  dragLandingMM,
  dragComfortStep,
  dragHeight,
  groupIndexForFace,
  groupsFromRanges,
  isDragDistance,
  isEditablePart,
  partLabel,
  pickPart,
  type PartGroup,
} from './picking'
import type { MeshPartRange } from '../types'

const ranges: MeshPartRange[] = [
  { Solid: 0, Role: 'tread', Start: 0, End: 12 },
  { Solid: 0, Role: 'stringer', Start: 12, End: 36 },
  { Solid: 1, Role: 'tread', Start: 36, End: 48 },
  { Solid: 1, Role: 'stringer', Start: 48, End: 72 },
]

describe('groupsFromRanges', () => {
  it('переводит PartRanges в группы с count', () => {
    expect(groupsFromRanges(ranges)).toEqual([
      { start: 0, count: 12, solid: 0, role: 'tread' },
      { start: 12, count: 24, solid: 0, role: 'stringer' },
      { start: 36, count: 12, solid: 1, role: 'tread' },
      { start: 48, count: 24, solid: 1, role: 'stringer' },
    ])
  })

  it('пустые диапазоны и отсутствие PartRanges дают пустой список', () => {
    expect(groupsFromRanges(undefined)).toEqual([])
    expect(groupsFromRanges([])).toEqual([])
    expect(groupsFromRanges([{ Solid: 0, Role: 'tread', Start: 5, End: 5 }])).toEqual([])
  })
})

describe('groupIndexForFace', () => {
  const groups = groupsFromRanges(ranges)

  it('находит группу по номеру треугольника', () => {
    expect(groupIndexForFace(groups, 0)).toBe(0)
    expect(groupIndexForFace(groups, 11)).toBe(0)
    expect(groupIndexForFace(groups, 12)).toBe(1)
    expect(groupIndexForFace(groups, 35)).toBe(1)
    expect(groupIndexForFace(groups, 36)).toBe(2)
    expect(groupIndexForFace(groups, 71)).toBe(3)
  })

  it('вне диапазона — минус один', () => {
    expect(groupIndexForFace(groups, -1)).toBe(-1)
    expect(groupIndexForFace(groups, 72)).toBe(-1)
    expect(groupIndexForFace([], 3)).toBe(-1)
  })
})

describe('pickPart', () => {
  it('возвращает solid, роль и индекс группы', () => {
    const p = pickPart(groupsFromRanges(ranges), 40)
    expect(p).toEqual({ solid: 1, role: 'tread', groupIndex: 2 })
  })

  it('промах мимо геометрии даёт null', () => {
    expect(pickPart(groupsFromRanges(ranges), 999)).toBeNull()
  })
})

describe('partLabel', () => {
  it('ступень нумеруется с единицы, служебные детали — без номера', () => {
    expect(partLabel(0, 'tread')).toBe('Ступень 1')
    expect(partLabel(6, 'tread')).toBe('Ступень 7')
    expect(partLabel(0, 'stringer')).toBe('Косоур')
    expect(partLabel(0, 'landing')).toBe('Площадка')
    expect(partLabel(0, 'railing_post')).toBe('Балясина')
  })

  it('неизвестная роль не ломает подпись', () => {
    expect(partLabel(3, 'weird_part')).toBe('Weird_part')
    expect(partLabel(3, '')).toBe('Деталь')
  })
})

describe('isEditablePart', () => {
  it('редактируются ступень, косоур и площадка', () => {
    const editable: PartGroup[] = [
      { start: 0, count: 1, solid: 0, role: 'tread' },
      { start: 0, count: 1, solid: 0, role: 'stringer' },
      { start: 0, count: 1, solid: 0, role: 'landing' },
    ]
    expect(editable.map((g) => isEditablePart(g.role))).toEqual([true, true, true])
  })

  it('ограждение не редактируется', () => {
    expect(isEditablePart('railing_post')).toBe(false)
  })
})

describe('dragHeight', () => {
  it('прибавляет смещение и округляет до 10 мм', () => {
    expect(dragHeight(2700, 123.4, { min: 1200, max: 6000 })).toBe(2820)
    expect(dragHeight(2700, -37, { min: 1200, max: 6000 })).toBe(2660)
    expect(dragHeight(2700, 0, { min: 1200, max: 6000 })).toBe(2700)
  })

  it('зажимает в допустимый диапазон материала', () => {
    expect(dragHeight(2700, 9999, { min: 1200, max: 6000 })).toBe(6000)
    expect(dragHeight(2700, -9999, { min: 1200, max: 4550 })).toBe(1200)
  })
})

describe('isDragDistance', () => {
  it('мелкое движение — это клик, большое — перетаскивание', () => {
    expect(isDragDistance(3)).toBe(false)
    expect(isDragDistance(5)).toBe(true)
    expect(isDragDistance(-5)).toBe(true)
  })
})

describe('dragComfortStep', () => {
  it('двигает шаг комфорта с шагом 5 мм', () => {
    expect(dragComfortStep(630, 23, { min: 600, max: 640 })).toBe(655 - 15)
    expect(dragComfortStep(630, -12, { min: 600, max: 640 })).toBe(620)
    expect(dragComfortStep(630, 7, { min: 600, max: 640 })).toBe(635)
  })

  it('зажимает в окно комфорта', () => {
    expect(dragComfortStep(630, 500, { min: 600, max: 640 })).toBe(640)
    expect(dragComfortStep(630, -500, { min: 600, max: 640 })).toBe(600)
  })
})

describe('dragAxisIsHorizontal', () => {
  it('по горизонтали правим проступь, по вертикали — высоту', () => {
    expect(dragAxisIsHorizontal(30, 5)).toBe(true)
    expect(dragAxisIsHorizontal(5, 30)).toBe(false)
    expect(dragAxisIsHorizontal(10, 10)).toBe(false)
  })
})

describe('dragLandingMM', () => {
  it('меняет габарит площадки с шагом 10 мм в допуске', () => {
    expect(dragLandingMM(1000, 47, { min: 600, max: 3000 })).toBe(1050)
    expect(dragLandingMM(1000, -23, { min: 600, max: 5000 })).toBe(980)
    expect(dragLandingMM(1000, 9000, { min: 600, max: 3000 })).toBe(3000)
    expect(dragLandingMM(1000, -9000, { min: 600, max: 5000 })).toBe(600)
  })
})
