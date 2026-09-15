import { describe, expect, it } from 'vitest'
import {
  defaultConfig,
  defaultRates,
  flightFields,
  railingForSpiral,
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

  it('прямой марш — общие поля + перила + габариты помещения', () => {
    expect(flightFields.straight).toEqual([...common, 'comfortStepMM', 'railing', 'roomWidthMM', 'roomLengthMM', 'approachSpaceMM'])
    expect(flightFields.straight).not.toContain('direction')
    expect(flightFields.straight).not.toContain('spiralDirection')
  })

  it('L/U — общие + поля площадки + габариты помещения + перила по сегментам и поворот + подход', () => {
    const l = [...common, 'comfortStepMM', 'landingWidthMM', 'landingDepthMM',
      'roomWidthMM', 'roomLengthMM', 'lowerStepCountMM',
      'railingLower', 'railingLanding', 'railingUpper', 'direction', 'approachSpaceMM']
    expect(flightFields.l_shape).toEqual(l)
    const u = [...common, 'comfortStepMM', 'landingWidthMM', 'landingDepthMM',
      'roomWidthMM', 'roomLengthMM', 'lowerStepCountMM',
      'railingLower', 'railingLanding', 'railingUpper', 'direction', 'approachSpaceMM']
    expect(flightFields.u_shape).toEqual(u)
  })

  it('спираль — без шага комфорта, но с наружным радиусом, габаритами помещения, направлением и подходом', () => {
    expect(flightFields.spiral).toEqual([...common, 'outerRadiusMM', 'roomWidthMM', 'roomLengthMM', 'spiralDirection', 'approachSpaceMM'])
    expect(flightFields.spiral).not.toContain('comfortStepMM')
    expect(flightFields.spiral).not.toContain('landingWidthMM')
    expect(flightFields.spiral).not.toContain('lowerStepCountMM')
    expect(flightFields.spiral).not.toContain('railing')
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
        'landingWidthMM', 'landingDepthMM', 'roomWidthMM', 'roomLengthMM',
        'approachSpaceMM', 'lowerStepCountMM', 'outerRadiusMM',
        'railing', 'railingLower', 'railingLanding', 'railingUpper',
        'direction', 'spiralDirection',
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

  it('габариты помещения валидируются для прямого марша (опционально)', () => {
    const cfg = { ...defaultConfig, flight: 'straight' as const }
    // Пустые — без ошибок (проверка вписываемости отключена).
    expect(validateForm(cfg).roomWidthMM).toBeUndefined()
    expect(validateForm(cfg).roomLengthMM).toBeUndefined()
    // Вне диапазона — ошибка.
    expect(validateForm({ ...cfg, roomWidthMM: '9000' }).roomWidthMM).toBe('Не более 8000')
    expect(validateForm({ ...cfg, roomLengthMM: '-1' }).roomLengthMM).toBe('Не менее 0')
    // В диапазоне — без ошибок.
    expect(
      validateForm({ ...cfg, roomWidthMM: '3000', roomLengthMM: '4000' }).roomWidthMM,
    ).toBeUndefined()
  })

  it('свободное пространство перед маршем: обязательно для прямого, диапазон 1000–1200', () => {
    const cfg = { ...defaultConfig, flight: 'straight' as const }
    // Дефолт 1000 — без ошибок.
    expect(validateForm(cfg).approachSpaceMM).toBeUndefined()
    // Пустое (явно очищенное) для прямого — обязательная ошибка (расчёт блокируется).
    expect(validateForm({ ...cfg, approachSpaceMM: '' }).approachSpaceMM).toBe('Укажите значение')
    // В диапазоне — без ошибок.
    expect(validateForm({ ...cfg, approachSpaceMM: '1000' }).approachSpaceMM).toBeUndefined()
    expect(validateForm({ ...cfg, approachSpaceMM: '1200' }).approachSpaceMM).toBeUndefined()
    // Вне диапазона — ошибка.
    expect(validateForm({ ...cfg, approachSpaceMM: '999' }).approachSpaceMM).toBe('Не менее 1000')
    expect(validateForm({ ...cfg, approachSpaceMM: '1201' }).approachSpaceMM).toBe('Не более 1200')
    // Для всех типов (включая L/U и спираль) поле теперь обязательно и
    // валидируется в диапазоне (EDR-0023, approachSpace для всех типов).
    expect(
      validateForm({ ...defaultConfig, flight: 'l_shape' as const, approachSpaceMM: '999' })
        .approachSpaceMM,
    ).toBe('Не менее 1000')
  })

  it('пределы толщины ступени зависят от материала', () => {
    expect(validateForm({ ...defaultConfig, stepThicknessMM: '9' }).stepThicknessMM).toBe(
      'Не более 8',
    )
    expect(validateForm({ ...defaultConfig, stepThicknessMM: '2' }).stepThicknessMM).toBe(
      'Не менее 3',
    )
    const alum = { ...defaultConfig, material: 'ALUM-5083' as const }
    expect(validateForm({ ...alum, stepThicknessMM: '61' }).stepThicknessMM).toBe('Не более 60')
    expect(validateForm({ ...alum, stepThicknessMM: '1' }).stepThicknessMM).toBe('Не менее 2')
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
    expect(rulesFor('stepThicknessMM', 'STEEL-S235')).toEqual({ min: 3, max: 8 })
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

describe('railingForSpiral', () => {
  it('перила спирали автоматически от направления', () => {
    expect(railingForSpiral('cw')).toBe('right')
    expect(railingForSpiral('ccw')).toBe('left')
  })
})

describe('toRequest railing/direction', () => {
  it('прямой марш шлёт одну сторону перил', () => {
    const req = toRequest({ ...defaultConfig, railing: 'right' })
    expect(req.railing).toBe('right')
    expect(req.railing_lower).toBeUndefined()
    expect(req.direction).toBeUndefined()
  })

  it('L/U шлют перила по сегментам и поворот площадки', () => {
    const req = toRequest({
      ...defaultConfig,
      flight: 'l_shape',
      railingLower: 'left',
      railingLanding: 'both',
      railingUpper: 'right',
      direction: 'right',
    })
    expect(req.railing_lower).toBe('left')
    expect(req.railing_landing).toBe('both')
    expect(req.railing_upper).toBe('right')
    expect(req.direction).toBe('right')
    expect(req.railing).toBeUndefined()
  })

  it('спираль шлёт только направление; перила считает сервер', () => {
    const req = toRequest({ ...defaultConfig, flight: 'spiral', spiralDirection: 'cw' })
    expect(req.spiral_direction).toBe('cw')
    expect(req.railing).toBeUndefined()
    expect(req.direction).toBeUndefined()
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

  it('габариты помещения отправляются и для прямого марша', () => {
    const sr = toRequest({
      ...defaultConfig,
      flight: 'straight' as const,
      roomWidthMM: '3000',
      roomLengthMM: '4200',
    })
    expect(sr.room_width_mm).toBe(3000)
    expect(sr.room_length_mm).toBe(4200)
    // Пустые габариты — 0 (проверка вписываемости отключена).
    const empty = toRequest(defaultConfig)
    expect(empty.room_width_mm).toBe(0)
    expect(empty.room_length_mm).toBe(0)
  })

  it('свободное пространство перед маршем отправляется только для прямого', () => {
    const sr = toRequest({
      ...defaultConfig,
      flight: 'straight' as const,
      approachSpaceMM: '1100',
    })
    expect(sr.approach_space_mm).toBe(1100)
    // Пустое для прямого — дефолт 1000.
    expect(toRequest({ ...defaultConfig, flight: 'straight' as const }).approach_space_mm).toBe(1000)
    // L/U и спираль теперь тоже отправляют зону подхода (EDR-0023).
    expect(
      toRequest({ ...defaultConfig, flight: 'l_shape' as const, approachSpaceMM: '1100' })
        .approach_space_mm,
    ).toBe(1100)
    expect(
      toRequest({ ...defaultConfig, flight: 'u_shape' as const, approachSpaceMM: '1100' })
        .approach_space_mm,
    ).toBe(1100)
    expect(
      toRequest({ ...defaultConfig, flight: 'spiral' as const, approachSpaceMM: '1100' })
        .approach_space_mm,
    ).toBe(1100)
    // Пустое для L/U — дефолт 1000.
    expect(toRequest({ ...defaultConfig, flight: 'l_shape' as const }).approach_space_mm).toBe(1000)
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