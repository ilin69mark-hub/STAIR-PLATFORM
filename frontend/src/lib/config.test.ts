import { describe, expect, it } from 'vitest'
import { defaultConfig, defaultRates, toRatesRequest, toRequest, validateForm } from './config'

describe('validateForm', () => {
  it('дефолтная конфигурация без ошибок', () => {
    expect(validateForm(defaultConfig)).toEqual({})
  })

  it('значение меньше минимума', () => {
    expect(validateForm({ ...defaultConfig, widthMM: '250' }).widthMM).toBe('Не менее 300')
  })

  it('значение больше максимума', () => {
    expect(validateForm({ ...defaultConfig, heightMM: '7000' }).heightMM).toBe('Не более 6000')
  })

  it('нечисловое значение', () => {
    expect(validateForm({ ...defaultConfig, widthMM: 'abc' }).widthMM).toBe('Введите число')
  })

  it('пустое обязательное поле', () => {
    expect(validateForm({ ...defaultConfig, railingHeightMM: '' }).railingHeightMM).toBe(
      'Укажите значение',
    )
  })

  it('comfortStep необязателен, но проверяется диапазоном', () => {
    expect(validateForm(defaultConfig).comfortStepMM).toBeUndefined()
    expect(validateForm({ ...defaultConfig, comfortStepMM: '550' }).comfortStepMM).toBe(
      'Не менее 600',
    )
    expect(validateForm({ ...defaultConfig, comfortStepMM: '650' }).comfortStepMM).toBe(
      'Не более 640',
    )
  })
})

describe('toRequest', () => {
  it('маппит конфигурацию в snake_case', () => {
    const r = toRequest(defaultConfig)
    expect(r.width_mm).toBe(900)
    expect(r.height_mm).toBe(2700)
    expect(r.step_height_mm).toBe(180)
    expect(r.comfort_step_mm).toBeUndefined()
  })

  it('включает comfort_step_mm при заполнении', () => {
    const r = toRequest({ ...defaultConfig, comfortStepMM: '620' })
    expect(r.comfort_step_mm).toBe(620)
  })
})

describe('toRatesRequest', () => {
  it('undefined при полностью пустых полях', () => {
    expect(toRatesRequest(defaultRates)).toBeUndefined()
  })

  it('включает только заполненные ставки', () => {
    const r = toRatesRequest({ ...defaultRates, steel: '200', overheadPct: '12.5' })
    expect(r).toEqual({
      material_per_kg_rub: { 'STEEL-S235': 200 },
      overhead_percent: 12.5,
    })
  })

  it('все типы ставок маппятся корректно', () => {
    const r = toRatesRequest({
      ...defaultRates,
      alum: '300',
      wood: '150',
      machinePerHour: '1000',
      laborPerHour: '800',
      marginPct: '15',
      discountPct: '5',
      taxPct: '20',
    })
    expect(r).toEqual({
      material_per_kg_rub: { 'ALUM-5083': 300, 'WOOD-OAK': 150 },
      machine_per_hour_rub: 1000,
      labor_per_hour_rub: 800,
      margin_percent: 15,
      discount_percent: 5,
      tax_percent: 20,
    })
  })
})