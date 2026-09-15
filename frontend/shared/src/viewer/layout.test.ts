import { describe, expect, it } from 'vitest'
import { approachZoneCenterX, entranceFaceX, stairTopLineX } from './layout'

// Реальные данные прямого марша (geometry.Generate, H=2700, n=15, b=270,
// W=900, st=40, approach=1000, Run=4050): до зеркалирования bbox X∈[960,5050];
// mirrorX в GeometryViewer даёт локальный box X∈[−5050,−960].
const straightBox = {
  min: { x: -5050, y: 0, z: 0 },
  max: { x: -960, y: 2700, z: 900 },
} as const

describe('stairTopLineX', () => {
  it('прямой марш: линия верха на стороне box.min.x (верх марша после mirrorX)', () => {
    expect(stairTopLineX(straightBox, 'straight')).toBe(-5050)
  })
  it('прочие типы: линия верха на стороне box.max.x (без зеркалирования)', () => {
    expect(stairTopLineX(straightBox, 'l_shape')).toBe(-960)
    expect(stairTopLineX(straightBox, 'u_shape')).toBe(-960)
    expect(stairTopLineX(straightBox, 'spiral')).toBe(-960)
  })
})

describe('entranceFaceX', () => {
  it('прямой марш: грань входа на StepThickness ближе к маршу, чем box.max.x', () => {
    // box.max.x = −960 — хвост первой проступи; грань лицевой панели/входа — ещё на 40 мм вперёд.
    expect(entranceFaceX(straightBox, 'straight', 40)).toBe(-1000)
  })
  it('прочие типы: грань входа остаётся на box.max.x (поведение не меняется)', () => {
    expect(entranceFaceX(straightBox, 'l_shape', 40)).toBe(-960)
  })
})

describe('approachZoneCenterX', () => {
  // offset = −min.x = 5050 → мир: зона [box.max.x−st+offset, +approach] = [4050, 5050].
  it('прямой марш: зона примыкает к входу и ровно упирается в стену комнаты (roomWidth = Run+approach)', () => {
    const st = 40
    const ap = 1000
    const offsetX = -straightBox.min.x // computePlacement: прижим к стене П
    const start = entranceFaceX(straightBox, 'straight', st) + offsetX
    const end = start + ap
    expect(start).toBe(4050) // грань входа в мировых координатах
    expect(end).toBe(5050) // = roomWidth: зона упирается ровно в стену комнаты, ничего не вылезает
    expect(approachZoneCenterX(straightBox, 'straight', st, ap)).toBe(-500) // центр в локальных
    expect(approachZoneCenterX(straightBox, 'straight', st, ap) + offsetX).toBe(4550) // центр в мире
  })
})