import { describe, expect, it } from 'vitest'
import { acesToneMappedGrey, acesWhitePoint } from './postfx'

// Фон сцены — clear-цвет кадра, и он проходит по той же цепочке, что и
// геометрия. После появления пост-обработки белый 1.0 уезжал в серый:
// «белый циклорамный» фон перестал быть белым. Тесты фиксируют решение:
// цвет фона = значение, которое ПОСЛЕ ACES даёт ровно 1.0.
describe('белая точка фона при ACES', () => {
  it('обычный белый тонмаппится в серый — ровно та беда, которую чиним', () => {
    // Проверено против шейдера three: 1.0 при экспозиции 1.05 → ~0.80.
    expect(acesToneMappedGrey(1, 1.05)).toBeGreaterThan(0.75)
    expect(acesToneMappedGrey(1, 1.05)).toBeLessThan(0.85)
  })

  it('белая точка после тонмаппинга даёт ровно белый', () => {
    for (const exposure of [1.0, 1.05, 1.2, 0.8]) {
      const w = acesWhitePoint(exposure)
      expect(acesToneMappedGrey(w, exposure)).toBeCloseTo(1, 3)
      // Сандвич: точка правдоподобно больше единицы, но не абсурд.
      expect(w).toBeGreaterThan(1)
      expect(w).toBeLessThan(64)
    }
  })

  it('чем выше экспозиция, тем ниже нужная белая точка', () => {
    // Иначе при подъёме экспозиции фон «уезжал» в пересвет.
    expect(acesWhitePoint(1.4)).toBeLessThan(acesWhitePoint(0.9))
  })

  it('тёмные значения не трогаются: 0 остаётся 0', () => {
    expect(acesToneMappedGrey(0, 1.05)).toBe(0)
  })
})
