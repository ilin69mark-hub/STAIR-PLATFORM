import { describe, expect, it } from 'vitest'
import { rub, flightLabel } from './proposal'

describe('proposal pure helpers', () => {
  it('rub', () => {
    expect(rub(undefined)).toBe('—')
    expect(rub(10000, 2)).toContain('₽')
    expect(rub(12345, 2)).toContain('123')
  })
  it('flightLabel', () => {
    expect(flightLabel({ spiral: {} } as any)).toBe('Винтовая')
    expect(flightLabel({ ushape: {} } as any)).toBe('П-образная')
    expect(flightLabel({ lshape: {} } as any)).toBe('Г-образная')
    expect(flightLabel({} as any)).toBe('Прямой марш')
  })
})
