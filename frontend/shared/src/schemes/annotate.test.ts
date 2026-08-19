import { describe, expect, it } from 'vitest'
import { overlap, placeLabel, rectAt, bboxRect, textWidth } from './annotate'

describe('annotate', () => {
  it('overlap детектирует пересечение прямоугольников', () => {
    expect(overlap({ x: 0, y: 0, w: 10, h: 10 }, { x: 5, y: 5, w: 10, h: 10 })).toBe(true)
    expect(overlap({ x: 0, y: 0, w: 10, h: 10 }, { x: 20, y: 20, w: 10, h: 10 })).toBe(false)
    expect(overlap({ x: 0, y: 0, w: 10, h: 10 }, { x: 10, y: 0, w: 10, h: 10 })).toBe(false)
  })

  it('bboxRect считает границы', () => {
    const b = bboxRect([{ x: 2, y: -4 }, { x: 10, y: 8 }, { x: 6, y: 0 }])
    expect(b).toEqual({ x: 2, y: -4, w: 8, h: 12 })
  })

  it('placeLabel ставит лейбл в свободной зоне мимо контура', () => {
    const obstacle = rectAt(0, 0, 200, 100)
    const p = placeLabel({ ax: 0, ay: 0, w: 80, h: 16, dx: 1, dy: 0, obstacles: [obstacle], step: 20 })
    // Первые шаги (20..) пересекают контур (x+10..x+90) → уезжает дальше.
    expect(rectAt(p.x, p.y, 80, 16).x).toBeGreaterThanOrEqual(obstacle.x + obstacle.w)
  })

  it('placeLabel избегает уже размещённые подписи', () => {
    const placed = [rectAt(0, 0, 40, 16)]
    const p1 = placeLabel({ ax: 0, ay: 0, w: 40, h: 16, dx: 1, dy: 0, placed, step: 20 })
    const p2 = placeLabel({ ax: 0, ay: 0, w: 40, h: 16, dx: 1, dy: 0, placed: [...placed, rectAt(p1.x, p1.y, 40, 16)], step: 20 })
    expect(overlap(rectAt(p1.x, p1.y, 40, 16), rectAt(p2.x, p2.y, 40, 16))).toBe(false)
  })

  it('textWidth оценивает ширину текста положительно', () => {
    expect(textWidth('1200 мм', 12)).toBeGreaterThan(0)
  })
})
