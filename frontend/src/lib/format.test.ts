import { describe, expect, it } from 'vitest'
import { fmt } from './format'

describe('fmt.mm', () => {
  it('форматирует значение с разделителем и единицей', () => {
    expect(fmt.mm(900)).toBe('900 мм')
    expect(fmt.mm(12345)).toBe('12\u00A0345 мм')
  })

  it('undefined → длинное тире', () => {
    expect(fmt.mm(undefined)).toBe('—')
  })
})

describe('fmt.mm2 / fmt.mm3', () => {
  it('переводит в млн и подписывает степень', () => {
    expect(fmt.mm2(1_000_000)).toBe('1×10⁶ мм²')
    expect(fmt.mm2(4_100_000)).toBe('4,1×10⁶ мм²')
    expect(fmt.mm3(1_000_000)).toBe('1×10⁶ мм³')
  })

  it('undefined → длинное тире', () => {
    expect(fmt.mm2(undefined)).toBe('—')
    expect(fmt.mm3(undefined)).toBe('—')
  })
})

describe('fmt.deg', () => {
  it('переводит радианы в градусы', () => {
    expect(fmt.deg(Math.PI / 2)).toBe('90.0°')
    expect(fmt.deg(0)).toBe('0.0°')
  })

  it('undefined → длинное тире', () => {
    expect(fmt.deg(undefined)).toBe('—')
  })
})

describe('fmt.rub', () => {
  it('делит минорные единицы на decimals', () => {
    expect(fmt.rub(100)).toBe('1,00 ₽')
    expect(fmt.rub(286828274)).toBe('2\u00A0868\u00A0282,74 ₽')
  })

  it('учитывает decimals валюты', () => {
    expect(fmt.rub(12345, 3)).toBe('12,35 ₽')
  })

  it('undefined → длинное тире', () => {
    expect(fmt.rub(undefined)).toBe('—')
  })
})

describe('fmt.pct', () => {
  it('переводит 0..1 в проценты', () => {
    expect(fmt.pct(0.5)).toBe('50%')
    expect(fmt.pct(0.16)).toBe('16%')
  })

  it('undefined → длинное тире', () => {
    expect(fmt.pct(undefined)).toBe('—')
  })
})