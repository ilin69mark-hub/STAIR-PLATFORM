import { describe, expect, it } from 'vitest'
import { rub } from './proposal'

describe('proposal pure helpers', () => {
  it('rub', () => {
    expect(rub(undefined)).toBe('—')
    expect(rub(10000, 2)).toContain('₽')
    expect(rub(12345, 2)).toContain('123')
  })
})
