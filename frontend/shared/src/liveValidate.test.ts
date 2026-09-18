import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defaultConfig } from './config'
import {
  VALIDATE_DEBOUNCE_MS,
  LiveValidator,
  adaptValidation,
  applySuggestion,
  applyVariation,
  fieldKeysFor,
} from './liveValidate'

const storeIssue = (o: Record<string, unknown>) => ({
  code: 'GEO-ANGLE',
  severity: 'error',
  element: 'angle',
  message: 'Угол наклона вне допустимого диапазона',
  guide: 'Уменьшите шаг ступени: угол станет положе.',
  fix: 'Увеличьте высоту ступени',
  param: undefined,
  suggestions: [
    { step_count: 17, step_height_mm: 176.5, tread_depth_mm: 290, angle_deg: 31.3 },
  ],
  ...o,
})

describe('adaptValidation (store snake_case)', () => {
  it('нормирует issues и маппит element→поле формы', () => {
    const v = adaptValidation({
      valid: false,
      blocking: true,
      issues: [storeIssue({})],
    })
    expect(v.valid).toBe(false)
    expect(v.blocking).toBe(true)
    expect(v.issues[0].code).toBe('GEO-ANGLE')
    expect(v.issues[0].suggestions?.[0].stepHeightMm).toBe(176.5)
    expect(v.fieldErrors.stepHeightMM).toBe(v.issues[0].guide)
  })

  it('spiral: param «Радиус спирали» маппится в outerRadiusMM', () => {
    const v = adaptValidation({
      valid: false,
      blocking: true,
      issues: [
        {
          code: 'GEO-SPIRAL-RADIUS',
          severity: 'error',
          element: 'configuration',
          message: 'Радиус спирали не превышает ширину марша',
          param: 'Радиус спирали',
          guide: 'Наружный радиус спирали должен быть больше ширины марша.',
        },
      ],
    })
    expect(v.fieldErrors.outerRadiusMM).toBeTruthy()
    expect(fieldKeysFor(v.issues[0])).toEqual(['outerRadiusMM'])
  })

  it('room_fit подсвечивает оба габарита помещения', () => {
    const v = adaptValidation({
      valid: false,
      blocking: true,
      issues: [
        {
          code: 'ROOM-FIT',
          severity: 'error',
          element: 'room',
          message: 'Лестница не помещается в помещение',
        },
      ],
    })
    expect(v.fieldErrors.roomWidthMM).toBeTruthy()
    expect(v.fieldErrors.roomLengthMM).toBeTruthy()
  })
})

describe('adaptValidation (admin PascalCase)', () => {
  it('читает Code/Element/Param/Guide/Suggestions', () => {
    const v = adaptValidation({
      Valid: false,
      Blocking: true,
      Issues: [
        {
          Code: 'GEO-CLEARANCE',
          Severity: 'error',
          Element: 'clearance',
          Message: 'Просвет меньше нормы',
          Guide: 'Вертикальный просвет должен быть ≥ 2000 мм.',
          Param: 'Просвет',
          Suggestions: [
            { StepCount: 16, StepHeightMm: 168.75, TreadDepthMm: 293, AngleDeg: 30 },
          ],
        },
      ],
    } as never)
    expect(v.blocking).toBe(true)
    expect(v.issues[0].param).toBe('Просвет')
    expect(v.fieldErrors.clearanceMM).toBeTruthy()
    expect(v.issues[0].suggestions?.[0].stepCount).toBe(16)
  })
})

describe('fieldKeysFor', () => {
  it('производные шаг/проступь/угол правятся через stepHeightMM', () => {
    for (const element of ['step_height', 'tread_depth', 'angle']) {
      expect(fieldKeysFor({ element } as never)).toEqual(['stepHeightMM'])
    }
  })
  it('неизвестный element без param не подсвечивает поле', () => {
    expect(fieldKeysFor({ element: 'configuration' } as never)).toEqual([])
  })
})

describe('applySuggestion / applyVariation', () => {
  it('прямой марш: подставляет шаг ступени', () => {
    const next = applySuggestion({ ...defaultConfig, flight: 'straight' }, {
      stepCount: 17,
      stepHeightMm: 176.5,
      treadDepthMm: 290,
      angleDeg: 31.3,
    })
    expect(next.stepHeightMM).toBe('176.5')
  })

  it('спираль: подставляет ширину и радиус', () => {
    const next = applySuggestion({ ...defaultConfig, flight: 'spiral' }, {
      stepCount: 20,
      stepHeightMm: 135,
      treadDepthMm: 210,
      angleDeg: 31,
      outerRadiusMm: 1050,
      widthMm: 900,
    })
    expect(next.outerRadiusMM).toBe('1050')
    expect(next.widthMM).toBe('900')
  })

  it('L-марш: подставляет нижний марш', () => {
    const next = applySuggestion(
      { ...defaultConfig, flight: 'l_shape' },
      { stepCount: 18, lowerStepCount: 9, stepHeightMm: 150, treadDepthMm: 296, angleDeg: 30 },
    )
    expect(next.lowerStepCountMM).toBe('9')
  })

  it('variation: пустые значения и шаг комфорта не затирают форму', () => {
    const next = applyVariation(
      { ...defaultConfig, flight: 'straight', roomWidthMM: '4000', comfortStepMM: '620' },
      {
        id: 'v1',
        title: 'A',
        description: '',
        config: { roomWidthMM: '4200', roomLengthMM: '', comfortStepMM: '', stepHeightMM: '175' },
        fits: true,
        summary: '',
      },
    )
    expect(next.roomWidthMM).toBe('4200')
    expect(next.roomLengthMM).toBe(defaultConfig.roomLengthMM)
    expect(next.comfortStepMM).toBe('620')
    expect(next.stepHeightMM).toBe('175')
  })

  it('variation: пустой approachSpace подменяется дефолтом 1000', () => {
    const next = applyVariation(
      { ...defaultConfig, approachSpaceMM: '' },
      {
        id: 'v1',
        title: 'A',
        description: '',
        config: {},
        fits: true,
        summary: '',
      },
    )
    expect(next.approachSpaceMM).toBe('1000')
  })
})

describe('LiveValidator', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  const fetchOf = (issues: unknown[]) =>
    vi.fn().mockResolvedValue({ valid: false, blocking: true, issues })

  it('схлопывает серию schedule в один запрос (debounce)', async () => {
    const v = new LiveValidator()
    const fetch = fetchOf([storeIssue({})])
    const onChange = vi.fn()
    v.schedule({ key: 'k1', fetch, onChange })
    v.schedule({ key: 'k2', fetch, onChange })
    v.schedule({ key: 'k3', fetch, onChange })
    expect(fetch).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(VALIDATE_DEBOUNCE_MS)
    expect(fetch).toHaveBeenCalledTimes(1)
    await Promise.resolve()
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('дедуп: повтор ключа не делает запрос, отдаёт кэш', async () => {
    const v = new LiveValidator()
    const fetch = fetchOf([storeIssue({})])
    const onChange = vi.fn()
    v.schedule({ key: 'k1', fetch, onChange })
    await vi.advanceTimersByTimeAsync(VALIDATE_DEBOUNCE_MS)
    await Promise.resolve()
    expect(fetch).toHaveBeenCalledTimes(1)
    v.schedule({ key: 'k1', fetch, onChange })
    expect(onChange).toHaveBeenCalledTimes(2) // синхронно из кэша
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('invalidate(key) сбрасывает кэш', async () => {
    const v = new LiveValidator()
    const fetch1 = fetchOf([storeIssue({})])
    const onChange = vi.fn()
    v.schedule({ key: 'k1', fetch: fetch1, onChange })
    await vi.advanceTimersByTimeAsync(VALIDATE_DEBOUNCE_MS)
    await Promise.resolve()
    v.invalidate('k1')
    const fetch2 = fetchOf([storeIssue({})])
    v.schedule({ key: 'k1', fetch: fetch2, onChange })
    expect(fetch1).toHaveBeenCalledTimes(1)
    expect(fetch2).not.toHaveBeenCalled() // тикает дебаунс
    await vi.advanceTimersByTimeAsync(VALIDATE_DEBOUNCE_MS)
    expect(fetch2).toHaveBeenCalledTimes(1)
  })

  it('ошибка сети сбрасывает кэш и зовёт onChange(null)', async () => {
    const v = new LiveValidator()
    const fetch = vi.fn().mockRejectedValue(new Error('net'))
    const onChange = vi.fn()
    v.schedule({ key: 'k1', fetch, onChange })
    await vi.advanceTimersByTimeAsync(VALIDATE_DEBOUNCE_MS)
    await Promise.resolve()
    expect(onChange).toHaveBeenCalledWith(null)
    v.schedule({ key: 'k1', fetch, onChange })
    await vi.advanceTimersByTimeAsync(VALIDATE_DEBOUNCE_MS)
    expect(fetch).toHaveBeenCalledTimes(2) // повторный ввод проверяет заново
  })

  it('cancel() останавливает таймер', () => {
    const v = new LiveValidator()
    const fetch = fetchOf([])
    const onChange = vi.fn()
    v.schedule({ key: 'k1', fetch, onChange })
    v.cancel()
    vi.advanceTimersByTime(VALIDATE_DEBOUNCE_MS * 2)
    expect(fetch).not.toHaveBeenCalled()
  })
})
describe('adaptValidation · регистр ключей', () => {
  it('читает guide/fix из PascalCase-ответа (транспорт админки)', () => {
    const v = adaptValidation({
      Valid: false,
      Blocking: true,
      Issues: [
        {
          Code: 'GEO-SPIRAL-RADIUS',
          Severity: 'error',
          Element: 'configuration',
          Message: 'Радиус спирали не превышает ширину марша',
          Param: 'Радиус спирали',
          Guide: 'Наружный радиус спирали должен быть больше ширины марша.',
          Fix: 'Увеличьте радиус спирали',
          Suggestions: [
            { StepCount: 18, StepHeightMm: 150, TreadDepthMm: 290, AngleDeg: 31.3, OuterRadiusMm: 1050, WidthMm: 900 },
          ],
        },
      ],
    })
    expect(v.blocking).toBe(true)
    expect(v.valid).toBe(false)
    expect(v.issues).toHaveLength(1)
    expect(v.issues[0].guide).toBe('Наружный радиус спирали должен быть больше ширины марша.')
    expect(v.issues[0].fix).toBe('Увеличьте радиус спирали')
    expect(v.issues[0].suggestions?.[0].outerRadiusMm).toBe(1050)
    expect(v.fieldErrors.outerRadiusMM).toBe('Наружный радиус спирали должен быть больше ширины марша.')
  })
})
