import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defaultConfig, toRequest, validateForm, type ConfigForm } from './config'
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


// DOM-002 (2026-09-26): вариация GEO-ANGLE обязана реально снимать
// блокировку, для чего шаг комфорта из вариации ОБЯЗАН применяться.
//
// Контракт бэкенда (internal/engine/variation/variation.go):
//   ForAngle перебирает S ∈ {600, 620, 640} и считает проступь b = S − 2h,
//   угол α = atan(h/b). Обещание в summary («Угол 30.2°») верно ТОЛЬКО при
//   том S, который вернулся в config.comfortStepMM.
//
// Регрессия: applyVariation выбрасывал comfortStepMM с комментарием «он всегда
// дефолтный (630)», из-за чего на витрине вариации давали 29,3°/29,3°/36,3°
// вместо 30,2°/32,0°/35,3° и НЕ снимали GEO-ANGLE (а третья ломала проступь).
describe('applyVariation применяет шаг комфорта (DOM-002)', () => {
  // Реальные значения из серверного сценария H=3000, h0=158, S=630:
  // n=19 → блокирующий GEO-ANGLE (26,7°), три вариации со S = 620/600/640.
  const angleVariations = [
    { id: 'angle_30', title: 'Угол 30°', config: { stepHeightMM: '166.66666666666666', comfortStepMM: '620', widthMM: '1000', heightMM: '3000' } },
    { id: 'angle_32', title: 'Угол 32°', config: { stepHeightMM: '166.66666666666666', comfortStepMM: '600', widthMM: '1000', heightMM: '3000' } },
    { id: 'angle_35', title: 'Угол 35°', config: { stepHeightMM: '187.5', comfortStepMM: '640', widthMM: '1000', heightMM: '3000' } },
  ] as const

  it('шаг комфорта из вариации доходит до формы', () => {
    const prev: ConfigForm = { ...defaultConfig, flight: 'straight', heightMM: '3000', stepHeightMM: '158', comfortStepMM: '630' }
    for (const v of angleVariations) {
      const next = applyVariation(prev, { ...v, description: '', fits: true, summary: '' })
      expect(next.comfortStepMM).toBe(v.config.comfortStepMM)
      expect(next.stepHeightMM).toBe(v.config.stepHeightMM)
    }
  })

  it('применённая вариация даёт угол в норме 30–45° (а не 29,3°/36,3°)', () => {
    const prev: ConfigForm = { ...defaultConfig, flight: 'straight', heightMM: '3000', stepHeightMM: '158', comfortStepMM: '630' }
    for (const v of angleVariations) {
      const next = applyVariation(prev, { ...v, description: '', fits: true, summary: '' })
      const h = Number(next.stepHeightMM)
      const comfort = Number(next.comfortStepMM)
      const b = comfort - 2 * h
      const angle = (Math.atan(h / b) * 180) / Math.PI
      expect(b).toBeGreaterThanOrEqual(260) // проступь в норме
      expect(b).toBeLessThanOrEqual(320)
      expect(angle).toBeGreaterThanOrEqual(30) // GEO-ANGLE снят
      expect(angle).toBeLessThanOrEqual(45)
      // Форма остаётся валидной: validateForm не должен блокировать расчёт.
      expect(validateForm(next)).toEqual({})
      // И запрос уходит с этими значениями (точный контракт с бэкендом).
      const req = toRequest(next)
      expect(req.comfort_step_mm).toBe(Number(next.comfortStepMM))
      expect(req.step_height_mm).toBe(h)
    }
  })

  it('пустые значения вариации по-прежнему не затирают выбор пользователя', () => {
    const prev: ConfigForm = { ...defaultConfig, railing: 'left' }
    const next = applyVariation(prev, {
      id: 'room_fit',
      title: 't',
      description: '',
      fits: true,
      summary: '',
      config: { railing: '', comfortStepMM: '640' },
    })
    expect(next.railing).toBe('left')
    expect(next.comfortStepMM).toBe('640')
  })
})

// DOM-002: имена ключей вариации обязаны совпадать с полями ConfigForm.
// Раньше бэкенд отдавал heightMm/clearanceMm/railingMm/
// stringerThicknessMm/stepThicknessMm/winderCount — таких полей в форме нет,
// поэтому эти параметры не применялись ни в одной форме.
describe('ключи вариации совпадают с полями ConfigForm', () => {
  it('applyVariation применяет высоту, просвет, высоту перил, толщины и winder', () => {
    const prev: ConfigForm = { ...defaultConfig, flight: 'u_shape' }
    const next = applyVariation(prev, {
      id: 'angle',
      title: 't',
      description: '',
      fits: true,
      summary: '',
      config: {
        heightMM: '3200',
        clearanceMM: '2100',
        railingHeightMM: '950',
        stringerThicknessMM: '60',
        stepThicknessMM: '8',
        turnKind: 'winder',
        winderCountMM: '3',
      },
    })
    expect(next.heightMM).toBe('3200')
    expect(next.clearanceMM).toBe('2100')
    expect(next.railingHeightMM).toBe('950')
    expect(next.stringerThicknessMM).toBe('60')
    expect(next.stepThicknessMM).toBe('8')
    expect(next.turnKind).toBe('winder')
    expect(next.winderCountMM).toBe('3')
  })
})
