import { describe, expect, it } from 'vitest'
import { toThreePositions } from './projection'

describe('toThreePositions', () => {
  it('переставляет высоту (Z) в ось вверх three.js', () => {
    const out = toThreePositions([
      { X: 100, Y: 800, Z: 0 },
      { X: 200, Y: 900, Z: 540 },
    ])
    // three-координаты: x = X (подъём), y = Z (высота), z = Y (ширина).
    expect(out).toEqual([100, 0, 800, 200, 540, 900])
  })

  it('пустой вход даёт пустой массив', () => {
    expect(toThreePositions([])).toEqual([])
  })

  it('не меняет число компонент относительно входных вершин', () => {
    const out = toThreePositions([
      { X: 1, Y: 2, Z: 3 },
      { X: 4, Y: 5, Z: 6 },
      { X: 7, Y: 8, Z: 9 },
    ])
    expect(out).toHaveLength(9)
  })
})