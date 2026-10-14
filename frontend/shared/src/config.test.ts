import { describe, expect, it } from 'vitest'
import {
  defaultConfig,
  defaultRates,
  flightFields,
  rulesFor,
  isPlateFrame,
  fixedTreadThicknessMM,
  materialThicknessMM,
  fitThicknessMM,
  toRatesRequest,
  toRequest,
  validateForm,
  purposeOptions,
  purposeOptionsFor,
  purposeLabel,
  purposeScopeNote,
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
    // Материал ступеней — общее поле: ступени есть у любого типа марша.
    'treadMaterial',
  ]


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


})

describe('validateForm', () => {
  it('дефолтная конфигурация без ошибок', () => {
    expect(validateForm(defaultConfig)).toEqual({})
  })

  it('значение меньше минимума', () => {
    // Нижняя граница ширины 600 мм — граница изделия (S-169).
    expect(validateForm({ ...defaultConfig, widthMM: '250' }).widthMM).toBe('Не менее 600')
  })

  it('значение больше максимума', () => {
    // Верхняя граница высоты 5000 мм — граница изделия (S-169).
    expect(validateForm({ ...defaultConfig, heightMM: '7000' }).heightMM).toBe('Не более 5000')
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
    // Для всех типов (включая L/U) поле теперь обязательно и
    // валидируется в диапазоне (EDR-0023, approachSpace для всех типов).
    expect(
      validateForm({ ...defaultConfig, flight: 'l_shape' as const, approachSpaceMM: '999' })
        .approachSpaceMM,
    ).toBe('Не менее 1000')
  })

  // Разделение материалов: пределы толщины ступени задаёт МАТЕРИАЛ СТУПЕНЕЙ,
  // а не каркаса. Иначе металлокаркас не собрать: стальной косоур бывает
  // 6–10 мм, а дубовая проступь — от 20 мм, и одно поле материала не могло
  // описать оба случая сразу.
  it('толщина ступени ограничена материалом СТУПЕНЕЙ, а не каркаса', () => {
    const steelFrameWoodTread = {
      ...defaultConfig,
      material: 'STEEL-C245' as const,
      treadMaterial: 'WOOD-OAK',
      stepThicknessMM: '40',
    }
    expect(validateForm(steelFrameWoodTread)).toEqual({})

    // Та же сталь 40 мм не годится: косоур 6 мм + стальная ступень 40 мм —
    // превышение стального выпуска.
    const steelBoth = { ...defaultConfig, treadMaterial: '', stepThicknessMM: '40' }
    expect(validateForm(steelBoth).stepThicknessMM).toBe('Не более 8')

    // Дуб требует минимум 40 мм на ступень, даже если каркас стальной
    // (решение владельца 2026-10-05: под скругление R10 нужна полка).
    const thinWood = { ...steelFrameWoodTread, stepThicknessMM: '10' }
    expect(validateForm(thinWood).stepThicknessMM).toBe('Не менее 40')
  })

  it('толщина подступенка равна толщине ступени: подступенок идёт по материалу ступеней', () => {
    // Стальной каркас + дубовая ступень: подступенок ДЕРЕВЯННЫЙ (бэк режет
    // PartRiser из cfg.TreadMaterial), поэтому его толщина обязана быть
    // допустимой для дуба, то есть 40 мм, а не 6 мм стали.
    const base = { ...defaultConfig, stepThicknessMM: '40' }
    const mixed = { ...base, material: 'STEEL-C245' as const, treadMaterial: 'WOOD-OAK' }
    expect(toRequest(mixed).riser_thickness_mm).toBe(40)

    // Металлические ступени: подступенок стальной, толщина ступени та же.
    const allSteel = { ...base, material: 'STEEL-C245' as const, treadMaterial: 'STEEL-C245', stepThicknessMM: '6' }
    expect(toRequest(allSteel).riser_thickness_mm).toBe(6)

    // Деревянная лестница: подступенок и проступь из одного материала.
    const wood = { ...base, material: 'WOOD-OAK' as const, treadMaterial: '', stepThicknessMM: '40' }
    expect(toRequest(wood).riser_thickness_mm).toBe(40)
  })

  it('toRequest отправляет tread_material только когда он отличается', () => {
    const base = { ...defaultConfig, stepThicknessMM: '40' }
    expect(toRequest({ ...base, treadMaterial: '' })).not.toHaveProperty('tread_material')
    expect(toRequest({ ...base, treadMaterial: 'STEEL-C245' })).not.toHaveProperty('tread_material')
    expect(toRequest({ ...base, treadMaterial: 'WOOD-OAK' }).tread_material).toBe('WOOD-OAK')
  })

  // CONF-FIXING (2026-10-08): основания крепления и доп. крепёж по объекту.
  it('toRequest всегда отправляет основания крепления (дефолт — бетон)', () => {
    // Явная передача, даже когда выбор не меняли: в сохранённой ревизии должно
    // быть видно, что крепление выбирали, а не оставили на волю движку.
    expect(toRequest(defaultConfig).floor_fixing).toBe('concrete')
    expect(toRequest(defaultConfig).ceiling_fixing).toBe('concrete')
    const wood = { ...defaultConfig, floorFixing: 'wood' as const, ceilingFixing: 'wood' as const }
    expect(toRequest(wood).floor_fixing).toBe('wood')
    expect(toRequest(wood).ceiling_fixing).toBe('wood')
  })

  it('toRequest отправляет доп. крепёж только когда список непуст', () => {
    // Пустой массив в запросе не меняет расчёт, но засоряет его и логи.
    expect(toRequest(defaultConfig)).not.toHaveProperty('extra_fasteners')
    const withExtra = { ...defaultConfig, extraFasteners: ['SCREW-8x80@12', 'PLUG-8x80'] }
    expect(toRequest(withExtra).extra_fasteners).toEqual(['SCREW-8x80@12', 'PLUG-8x80'])
  })

  it('пределы толщины ступени зависят от материала', () => {
    expect(validateForm({ ...defaultConfig, stepThicknessMM: '9' }).stepThicknessMM).toBe(
      'Не более 8',
    )
    expect(validateForm({ ...defaultConfig, stepThicknessMM: '2' }).stepThicknessMM).toBe(
      'Не менее 3',
    )
    const wood = { ...defaultConfig, material: 'WOOD-OAK' as const }
    expect(validateForm({ ...wood, stepThicknessMM: '15' }).stepThicknessMM).toBe('Не менее 40')
    expect(validateForm({ ...wood, stepThicknessMM: '39' }).stepThicknessMM).toBe('Не менее 40')
    expect(validateForm({ ...wood, stepThicknessMM: '61' }).stepThicknessMM).toBe('Не более 60')
    expect(validateForm({ ...wood, stepThicknessMM: '40' }).stepThicknessMM).toBeUndefined()
  })

  it('максимальная высота подъёма зависит от материала', () => {
    // У стали предел совпадает с границей изделия (5000 мм, S-169).
    expect(validateForm({ ...defaultConfig, heightMM: '5100' }).heightMM).toBe('Не более 5000')
    // Предел ниже общей границы остался только у древесины:
    // крупнейшая плита покрывает подъём до 4550 мм.
    const wood = { ...defaultConfig, material: 'WOOD-OAK' as const }
    expect(validateForm({ ...wood, heightMM: '5000' }).heightMM).toBe('Не более 4550')
    expect(validateForm({ ...wood, heightMM: '4550' }).heightMM).toBeUndefined()
  })

  it('максимальная ширина марша ограничена листом материала', () => {
    expect(validateForm({ ...defaultConfig, widthMM: '3500' }).widthMM).toBe('Не более 3000')
    expect(validateForm({ ...defaultConfig, widthMM: '3000' }).widthMM).toBeUndefined()
  })

  it('косоур из стали ограничен выпуском листа 3–12 мм', () => {
    // Сталь — лазерный раскрой. Верхняя граница 12 мм, а не 8: норматив на
    // листовой косоур (решение владельца 2026-10-05) описывает строки
    // 8 / 10 / 12 мм, и форма обязана дать ввести предписанную толщину.
    // Раньше предел косоура был и 8, и 60 мм — стальной косоур 50 мм проходил
    // форму, но отвергался сервером (MFG-MATERIAL).
    expect(validateForm({ ...defaultConfig, stringerThicknessMM: '13' }).stringerThicknessMM).toBe(
      'Не более 12',
    )
    // 10 и 12 мм — строки норматива, форма их принимает.
    expect(
      validateForm({ ...defaultConfig, stringerThicknessMM: '10' }).stringerThicknessMM,
    ).toBeUndefined()
    expect(
      validateForm({ ...defaultConfig, stringerThicknessMM: '12' }).stringerThicknessMM,
    ).toBeUndefined()
    expect(
      validateForm({ ...defaultConfig, stringerThicknessMM: '8' }).stringerThicknessMM,
    ).toBeUndefined()
  })
})

describe('rulesFor', () => {
  it('базовая норма без материал-зависимых переопределений', () => {
    expect(rulesFor('stepHeightMM', 'STEEL-C245')).toEqual({ min: 150, max: 200 })
    expect(rulesFor('clearanceMM', 'WOOD-OAK').min).toBe(0)
  })

  it('толщина ступени и высота берутся из предела материала', () => {
    expect(rulesFor('stepThicknessMM', 'STEEL-C245')).toEqual({ min: 3, max: 8 })
    // Предел СТУПЕНИ 40 мм отдельно от общего минимума детали 20 мм:
    // косоур той же породы остаётся от 20 мм.
    expect(rulesFor('stepThicknessMM', 'WOOD-OAK')).toEqual({ min: 40, max: 60 })
    // Косоур из дуба остаётся от 30 мм (базовый норматив пиломатериала):
    // правило 40 мм подняло ТОЛЬКО ступень.
    expect(rulesFor('stringerThicknessMM', 'WOOD-OAK').min).toBe(30)
    expect(rulesFor('heightMM', 'STEEL-C245').max).toBe(5000)
    expect(rulesFor('heightMM', 'WOOD-OAK').max).toBe(4550)
    // Удалённые из каталога коды не должны попадать в пределы: правила
    // приходят из materialLimits, и код вне каталога обязан давать пусто.
    expect(rulesFor('stepThicknessMM', 'НЕТ-ТАКОГО' as never)).toEqual({})
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

})

// DOM-001 (2026-09-26): тип поворота и число поворотных ступеней были
// недостижимы из интерфейса — бэкенд принимал turn_kind/winder_count, но форма
// не могла их задать, поэтому поворотные ступени (winder) нельзя было выбрать.
describe('toRequest turn_kind/winder_count (DOM-001)', () => {

  it('L/U с площадкой шлют turn_kind=platform без winder_count', () => {
    const req = toRequest({ ...defaultConfig, flight: 'l_shape', turnKind: 'platform' })
    expect(req.turn_kind).toBe('platform')
    // winder_count несовместим с площадкой (CHECK-ограничение в БД).
    expect(req.winder_count).toBeUndefined()
  })

  it('L/U с поворотными ступенями шлют turn_kind и winder_count', () => {
    const req = toRequest({
      ...defaultConfig,
      flight: 'u_shape',
      turnKind: 'winder',
      winderCountMM: '5',
    })
    expect(req.turn_kind).toBe('winder')
    expect(req.winder_count).toBe(5)
  })

  it('пустое число поворотных ступеней не отправляется (0 = не задано)', () => {
    const req = toRequest({ ...defaultConfig, flight: 'l_shape', turnKind: 'winder' })
    expect(req.turn_kind).toBe('winder')
    expect(req.winder_count).toBeUndefined()
  })
})

describe('validateForm turn_kind/winder_count (DOM-001)', () => {
  it('площадка: число поворотных ступеней не обязательно', () => {
    const errors = validateForm({ ...defaultConfig, flight: 'l_shape', turnKind: 'platform' })
    expect(errors.winderCountMM).toBeUndefined()
    expect(errors.turnKind).toBeUndefined()
  })

  it('поворотные ступени: число обязательно', () => {
    const errors = validateForm({ ...defaultConfig, flight: 'u_shape', turnKind: 'winder' })
    expect(errors.winderCountMM).toBe('Укажите значение')
  })

  it('поворотные ступени: минимум 3', () => {
    const errors = validateForm({
      ...defaultConfig,
      flight: 'u_shape',
      turnKind: 'winder',
      winderCountMM: '2',
    })
    expect(errors.winderCountMM).toBe('Не менее 3')
  })

  it('поле не проверяется для прямого марша', () => {
    const errors = validateForm({
      ...defaultConfig,
      flight: 'straight',
      turnKind: 'winder',
      winderCountMM: '1',
    })
    expect(errors.winderCountMM).toBeUndefined()
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
    expect(toRequest(defaultConfig).material).toBe('STEEL-C245')
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

})

describe('toRatesRequest', () => {
  it('undefined при полностью пустых полях', () => {
    expect(toRatesRequest(defaultRates)).toBeUndefined()
  })

  it('включает только заполненные ставки', () => {
    const r = toRatesRequest({ ...defaultRates, steel: '200', overheadPct: '12.5' })
    expect(r).toEqual({
      material_per_kg_rub: { 'STEEL-C245': 200 },
      overhead_percent: 12.5,
    })
  })

  it('все типы ставок маппятся корректно', () => {
    const r = toRatesRequest({
      ...defaultRates,
      wood: '150',
      machinePerHour: '1000',
      laborPerHour: '800',
      marginPct: '15',
      discountPct: '5',
      taxPct: '20',
    })
    expect(r).toEqual({
      material_per_kg_rub: { 'WOOD-OAK': 150 },
      machine_per_hour_rub: 1000,
      labor_per_hour_rub: 800,
      margin_percent: 15,
      discount_percent: 5,
      tax_percent: 20,
    })
  })
})
// ---- Норматив на деревянный косоур-балку (решение владельца 2026-10-05) ----
//
// Зеркало internal/domain/engineering/timber_stringer.go: минимум толщины
// зависит от длины марша ПО ПРОЕКЦИИ и ширины марша, и сверх того несёт
// информацию о заготовке и глубине тела.
describe('норматив на деревянный косоур-балку', () => {
  // Проекция = n·b, где n = H/h, b = S − 2h. При H=2700, h=180, S=630:
  // n=15, b=270, проекция 4050 мм → полоса «более 4.0 м» → 80 мм.
  const wood = (over: Partial<ConfigForm> = {}): ConfigForm => ({
    ...defaultConfig,
    material: 'WOOD-OAK',
    treadMaterial: 'WOOD-OAK',
    heightMM: '2700',
    stepHeightMM: '180',
    widthMM: '900',
    stringerThicknessMM: '50',
    ...over,
  })





  it('норматив НЕ ругается в форме: ошибки только по пределам выпуска', () => {
    // Норматив рекомендателен, и сервер выдаёт его как SeverityWarning. Если
    // сделать его ошибкой формы, калькулятор начнёт отвергать СВОЮ
    // конфигурацию по умолчанию: подъём 2700 мм даёт проекцию 4050 мм, а
    // норматив требует 80 мм вместо стоящих в форме 50.
    expect(validateForm(wood({ stringerThicknessMM: '50' })).stringerThicknessMM).toBeUndefined()
    expect(validateForm(wood({ heightMM: '1800', stringerThicknessMM: '40' })).stringerThicknessMM).toBeUndefined()
    // Но предел ВЫПУСКА материала по-прежнему жёсткий.
    expect(validateForm(wood({ stringerThicknessMM: '101' })).stringerThicknessMM).toBe('Не более 100')
    // Стальной каркас: норматив для балки не применяется вовсе.
    expect(
      validateForm(wood({ material: 'STEEL-C245', stringerThicknessMM: '8' })).stringerThicknessMM,
    ).toBeUndefined()
  })

  it('верхняя граница толщики — 100 мм, иначе норму не ввести', () => {
    expect(validateForm(wood({ stringerThicknessMM: '101' })).stringerThicknessMM).toBe('Не более 100')
  })
})

// ---- Норматив на листовой (сварной) косоур (решение владельца 2026-10-05) ----
//
// Зеркало internal/domain/engineering/plate_stringer.go. В отличие от норматива
// на дерево, здесь ключ таблицы — ТОЛЩИНА ЛИСТА, а длина марша, ширина,
// глубина тела и высота заготовки — её пределы.
describe('норматив на листовой (сварный) косоур', () => {
  const steel = (over: Partial<ConfigForm> = {}): ConfigForm => ({
    ...defaultConfig,
    material: 'STEEL-C245',
    treadMaterial: 'STEEL-C245',
    heightMM: '2700',
    stepHeightMM: '180',
    widthMM: '900',
    stringerThicknessMM: '8',
    ...over,
  })

  // Толщина листа — НЕ выбор пользователя: её выводит норматив по длине и
  // ширине марша (решение владельца 2026-10-06). Поэтому строки проверяются
  // не «прислали 8/10/12», а «марш такой длины даёт такую толщину».




  it('у стали поле толщины косоура скрыто, у дерева остаётся выбор', () => {
    // Скрытие задано в форме (Constructor.visible): у листового каркаса
    // толщина не входной параметр, у балки из пиломатериала — выбор.
    expect(isPlateFrame('STEEL-C245')).toBe(true)
    expect(isPlateFrame('WOOD-OAK')).toBe(false)
    expect(isPlateFrame('METAL-AISI304')).toBe(true)
  })


})

// ---- Толщина металлической ступени: всегда 5 мм, без выбора ----
//
// Решение владельца 2026-10-05. Поле в калькуляторе у металла скрыто, но
// значение всё равно должно быть верным: отправленное 6 мм сервер привёл бы
// к 5 мм, и расчёт показал бы одно, а 3D — другое.
describe('толщина металлической ступени', () => {
  it('сталь: фиксированные 5 мм', () => {
    expect(fixedTreadThicknessMM('STEEL-C245')).toBe(5)
    expect(materialThicknessMM('STEEL-C245')).toBe(5)
  })

  it('дерево: толщина выбирается, пресета 40 мм', () => {
    expect(fixedTreadThicknessMM('WOOD-OAK')).toBe(0)
    expect(fixedTreadThicknessMM('WOOD-ASH')).toBe(0)
    expect(materialThicknessMM('WOOD-OAK')).toBe(40)
  })

  it('fitThicknessMM приводит любое значение к 5 мм у металла', () => {
    for (const cur of ['3', '5', '6', '8', '40', '']) {
      expect(fitThicknessMM('STEEL-C245', cur)).toBe('5')
    }
    // У дерева текущее значение сохраняется, если оно в допуске.
    expect(fitThicknessMM('WOOD-OAK', '40')).toBe('40')
    expect(fitThicknessMM('WOOD-OAK', '20')).toBe('40') // ниже минимума ступени
  })

  it('дефолт калькулятора — стальная лестница с 5 мм ступени', () => {
    expect(defaultConfig.material).toBe('STEEL-C245')
    expect(defaultConfig.stepThicknessMM).toBe('5')
  })

  it('границы выпуска листа на месте, даже при фиксированной толщине', () => {
    // Диапазон нужен серверу для проверки, а не пользователю для выбора.
    expect(rulesFor('stepThicknessMM', 'STEEL-C245')).toEqual({ min: 3, max: 8 })
  })
})

// Область применения назначения здания (решение владельца 2026-10-14):
// пока только жилые здания. Тест держит два свойства сразу — что
// нежилые назначения НЕЛЬЗЯ выбрать, и что сохранённый заказ с таким
// назначением всё равно виден и назван. Второе важнее первого: молча
// исчезнувшее значение выглядит как поломка данных, а не как решение.
describe('назначение здания: область применения', () => {
  it('в переключателе только жилые', () => {
    const values = purposeOptionsFor('house').map((o) => o.value)
    expect(values).toEqual(['house', 'flat'])
    for (const o of purposeOptionsFor('house')) {
      expect(o.disabled).toBeUndefined()
    }
  })

  it('нежилое назначение нельзя выбрать', () => {
    for (const p of ['public', 'industrial']) {
      const opts = purposeOptionsFor('house')
      const opt = opts.find((o) => o.value === p)
      expect(opt).toBeUndefined()
      // и при этом текущем значении оно не появляется как доступное
      const asCurrent = purposeOptionsFor(p)
      expect(asCurrent.filter((o) => o.disabled !== true)).toEqual(
        expect.arrayContaining([
          { value: 'house', label: expect.any(String) },
          { value: 'flat', label: expect.any(String) },
        ]),
      )
    }
  })

  it('сохранённое нежилое назначение показано, но заблокировано', () => {
    const opts = purposeOptionsFor('public')
    const kept = opts.find((o) => o.value === 'public')
    expect(kept).toBeDefined()
    expect(kept?.disabled).toBe(true)
    // Подпись остаётся нормативной: заказ обязан называть свой класс
    // здания, а не «что-то вне области».
    expect(kept?.label).toBe('Общественное здание (СП 118)')
    expect(kept?.hint).toContain('Вне текущей области')
    // И рядом остаются доступные жилые — из него можно уйти.
    expect(opts.filter((o) => o.disabled !== true).map((o) => o.value)).toEqual([
      'house',
      'flat',
    ])
  })

  it('мусорное и пустое значение не добавляет кнопку', () => {
    for (const v of ['', 'мусор', 'HOUSE']) {
      expect(purposeOptionsFor(v).map((o) => o.value)).toEqual(['house', 'flat'])
    }
  })

  it('пустой дефолт остаётся домом', () => {
    // Назначение по умолчанию — дом (СП 55); пустое поле не должно
    // превращаться в «ничего не выбрано» после сужения списка.
    expect(purposeOptionsFor(defaultConfig.purpose)[0].value).toBe('house')
  })

  it('полный нормативный список не пострадал', () => {
    // purposeOptions нужен для типа Purpose и purposeLabel: сужение
    // относится к ВЫБОРУ, а не к тому, что система знает о нормах.
    expect(purposeOptions.map((p) => p.value)).toEqual([
      'house',
      'flat',
      'public',
      'industrial',
    ])
    expect(purposeLabel('public')).toBe('Общественное здание (СП 118)')
    expect(purposeLabel('мусор')).toBe('мусор')
  })

  it('причина ограничения объявлена для показа', () => {
    expect(purposeScopeNote).toContain('жилых зданиях')
  })
})
