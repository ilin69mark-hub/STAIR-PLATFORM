import { describe, expect, it } from 'vitest'
import {
  defaultConfig,
  defaultRates,
  flightFields,
  rulesFor,
  toRatesRequest,
  toRequest,
  validateForm,
} from './config'

describe('flightFields', () => {
  const common = [
    'widthMM',
    'heightMM',
    'stepHeightMM',
    'stringerThicknessMM',
    'stepThicknessMM',
    'clearanceMM',
    'railingHeightMM',
  ]

  it('прямой марш — только общие поля', () => {
    expect(flightFields.straight).toEqual([...common, 'comfortStepMM'])
  })

  it('L/U — общие + поля площадки', () => {
    expect(flightFields.l_shape).toEqual([...common, 'comfortStepMM', 'landingWidthMM', 'lowerStepCountMM'])
    expect(flightFields.u_shape).toEqual([...common, 'comfortStepMM', 'landingWidthMM', 'lowerStepCountMM'])
  })

  it('спираль — без шага комфорта, но с наружным радиусом', () => {
    expect(flightFields.spiral).toEqual([...common, 'outerRadiusMM'])
    expect(flightFields.spiral).not.toContain('comfortStepMM')
    expect(flightFields.spiral).not.toContain('landingWidthMM')
    expect(flightFields.spiral).not.toContain('lowerStepCountMM')
  })

  it('SPA занимает все ключи ConfigForm', () => {
    const keys = new Set(Object.values(flightFields).flat())
    for (const variant of Object.values(flightFields)) {
      expect(new Set(variant).size).toBe(variant.length)
    }
    expect(keys).toEqual(
      new Set([
        'widthMM', 'heightMM', 'stepHeightMM', 'stringerThicknessMM',
        'stepThicknessMM', 'clearanceMM', 'railingHeightMM', 'comfortStepMM',
        'landingWidthMM', 'lowerStepCountMM', 'outerRadiusMM',
      ]),
    )
  })
})

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

  it('L-поля не валидируются для прямого марша', () => {
    expect(validateForm({ ...defaultConfig, landingWidthMM: '', lowerStepCountMM: '' })).toEqual({})
  })

  it('L-поля обязательны для l_shape', () => {
    const cfg = { ...defaultConfig, flight: 'l_shape' as const }
    expect(validateForm({ ...cfg, landingWidthMM: '' }).landingWidthMM).toBe('Укажите значение')
    expect(validateForm({ ...cfg, lowerStepCountMM: '' }).lowerStepCountMM).toBe('Укажите значение')
  })

  it('поля площадки обязательны и для u_shape', () => {
    const cfg = { ...defaultConfig, flight: 'u_shape' as const }
    expect(validateForm({ ...cfg, landingWidthMM: '' }).landingWidthMM).toBe('Укажите значение')
    expect(validateForm({ ...cfg, lowerStepCountMM: '' }).lowerStepCountMM).toBe('Укажите значение')
    expect(validateForm(cfg).landingWidthMM).toBeUndefined()
  })

  it('Wp в диапазоне формы допускается, даже если меньше ширины марша', () => {
    // Проверка Wp ≥ W выполняется на сервере (EDR-0005 §7); форма лишь
    // удерживает Wp в диапазоне 600–3000.
    const cfg = { ...defaultConfig, flight: 'l_shape' as const, landingWidthMM: '700' }
    expect(validateForm(cfg).landingWidthMM).toBeUndefined()
  })

  it('пределы толщины ступени зависят от материала', () => {
    expect(validateForm({ ...defaultConfig, stepThicknessMM: '61' }).stepThicknessMM).toBe(
      'Не более 60',
    )
    expect(validateForm({ ...defaultConfig, stepThicknessMM: '1' }).stepThicknessMM).toBe(
      'Не менее 2',
    )
    const wood = { ...defaultConfig, material: 'WOOD-OAK' as const }
    expect(validateForm({ ...wood, stepThicknessMM: '15' }).stepThicknessMM).toBe('Не менее 20')
    expect(validateForm({ ...wood, stepThicknessMM: '61' }).stepThicknessMM).toBe('Не более 60')
    expect(validateForm({ ...wood, stepThicknessMM: '30' }).stepThicknessMM).toBeUndefined()
  })

  it('максимальная высота подъёма зависит от материала', () => {
    expect(validateForm({ ...defaultConfig, heightMM: '6100' }).heightMM).toBe('Не более 6000')
    const alum = { ...defaultConfig, material: 'ALUM-5083' as const }
    expect(validateForm({ ...alum, heightMM: '5000' }).heightMM).toBe('Не более 4550')
    expect(validateForm({ ...alum, heightMM: '4550' }).heightMM).toBeUndefined()
  })

  it('максимальная ширина марша ограничена листом материала', () => {
    expect(validateForm({ ...defaultConfig, widthMM: '3500' }).widthMM).toBe('Не более 3000')
    expect(validateForm({ ...defaultConfig, widthMM: '3000' }).widthMM).toBeUndefined()
  })

  it('косоур не бывает толще лимита материала', () => {
    expect(validateForm({ ...defaultConfig, stringerThicknessMM: '61' }).stringerThicknessMM).toBe(
      'Не более 60',
    )
    expect(
      validateForm({ ...defaultConfig, stringerThicknessMM: '50' }).stringerThicknessMM,
    ).toBeUndefined()
  })
})

describe('rulesFor', () => {
  it('базовая норма без материал-зависимых переопределений', () => {
    expect(rulesFor('stepHeightMM', 'STEEL-S235')).toEqual({ min: 150, max: 200 })
    expect(rulesFor('clearanceMM', 'WOOD-OAK').min).toBe(0)
  })

  it('толщина ступени и высота берутся из предела материала', () => {
    expect(rulesFor('stepThicknessMM', 'STEEL-S235')).toEqual({ min: 2, max: 60 })
    expect(rulesFor('stepThicknessMM', 'ALUM-5083')).toEqual({ min: 2, max: 60 })
    expect(rulesFor('stepThicknessMM', 'WOOD-OAK')).toEqual({ min: 20, max: 60 })
    expect(rulesFor('heightMM', 'STEEL-S235').max).toBe(6000)
    expect(rulesFor('heightMM', 'ALUM-5083').max).toBe(4550)
    expect(rulesFor('heightMM', 'WOOD-OAK').max).toBe(4550)
  })

  it('косоур: норматив не менее 30 мм, лимит материала как максимум', () => {
    expect(rulesFor('stringerThicknessMM', 'WOOD-OAK')).toEqual({ min: 30, max: 60 })
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

  it('включает выбранный материал', () => {
    expect(toRequest(defaultConfig).material).toBe('STEEL-S235')
    const rw = toRequest({ ...defaultConfig, material: 'WOOD-OAK' })
    expect(rw.material).toBe('WOOD-OAK')
  })

  it('включает comfort_step_mm при заполнении', () => {
    const r = toRequest({ ...defaultConfig, comfortStepMM: '620' })
    expect(r.comfort_step_mm).toBe(620)
  })

  it('L-параметры отправляются только для l_shape', () => {
    const lr = toRequest({ ...defaultConfig, flight: 'l_shape' as const })
    expect(lr.landing_width_mm).toBe(1000)
    expect(lr.lower_step_count).toBe(6)

    const sr = toRequest(defaultConfig)
    expect(sr.landing_width_mm).toBeUndefined()
    expect(sr.lower_step_count).toBeUndefined()
  })

  it('параметры площадки отправляются и для u_shape', () => {
    const ur = toRequest({ ...defaultConfig, flight: 'u_shape' as const })
    expect(ur.landing_width_mm).toBe(1000)
    expect(ur.lower_step_count).toBe(6)
  })

  it('спираль не отправляет шаг комфорта', () => {
    const sr = toRequest({ ...defaultConfig, flight: 'spiral' as const, comfortStepMM: '620' })
    expect(sr.comfort_step_mm).toBeUndefined()
    expect(sr.outer_radius_mm).toBe(800)
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