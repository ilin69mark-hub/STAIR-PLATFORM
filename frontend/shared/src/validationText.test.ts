import { describe, expect, it } from 'vitest'
import { elementLabel, isValidColumnLabel, isCriticalSeverity, severityLabel } from './validationText'

describe('validationText', () => {
  it('переводит severity в русские подписи', () => {
    expect(severityLabel('error')).toBe('Ошибка')
    expect(severityLabel('warning')).toBe('Предупреждение')
    expect(severityLabel('info')).toBe('Информация')
  })

  it('фолбэк для пустого/неизвестного severity', () => {
    expect(severityLabel(null)).toBe('—')
    expect(severityLabel(undefined)).toBe('—')
    expect(severityLabel('EXTRA')).toBe('EXTRA')
  })

  it('переводит element в русские подписи', () => {
    expect(elementLabel('room')).toBe('Помещение')
    expect(elementLabel('tread_depth')).toBe('Проступь')
    expect(elementLabel('stringer_thickness')).toBe('Толщина косоура')
  })

  it('фолбэк для неизвестного element', () => {
    expect(elementLabel('weird_key')).toBe('weird_key')
    expect(elementLabel(null)).toBe('—')
  })

  it('isValidColumnLabel является алиасом elementLabel', () => {
    expect(isValidColumnLabel('clearance')).toBe('Просвет')
  })

  it('isCriticalSeverity выделяет только блокирующие уровни', () => {
    expect(isCriticalSeverity('error')).toBe(true)
    expect(isCriticalSeverity('critical')).toBe(true)
    expect(isCriticalSeverity('warning')).toBe(false)
    expect(isCriticalSeverity(null)).toBe(false)
  })
})