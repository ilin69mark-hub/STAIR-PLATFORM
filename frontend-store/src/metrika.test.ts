import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { grantConsent, revokeConsent } from '@shared/consent'
import {
  isMetrikaLoaded,
  loadMetrika,
  loadedCounterId,
  normalizeCounterId,
  resetMetrikaModule,
  startMetrika,
  unloadMetrika,
} from './metrika'
import { get } from '@shared/storefront/api/client'

// Гейт согласия и вебвизор. Оба решения здесь — юридические, и оба должны
// ломаться громко, если кто-то их случайно обойдёт.

vi.mock('@shared/storefront/api/client', () => ({
  get: vi.fn(async () => ({ counters: { yandex_metrika_id: '987654' } })),
}))

const getMock = vi.mocked(get)

beforeEach(() => {
  localStorage.clear()
  resetMetrikaModule()
  getMock.mockClear()
  getMock.mockResolvedValue({ counters: { yandex_metrika_id: '987654' } })
})

afterEach(() => {
  unloadMetrika()
  vi.useRealTimers()
})

describe('ID счётчика', () => {
  // Поле в настройках магазина — свободный текст, а ID уходит в URL
  // стороннего скрипта. Без фильтра туда можно было бы подставить что угодно.
  it('принимает чистые цифры с обрезкой пробелов', () => {
    expect(normalizeCounterId('987654')).toBe('987654')
    expect(normalizeCounterId(' 987654 ')).toBe('987654')
  })

  // Отвергаем ЦЕЛИКОМ, а не выкидываем не-цифры: иначе «<img onerror=alert(1)>»
  // превратился бы в ID «1» и тихо подключил бы чужой счётчик.
  it('отвергает значение с любыми другими символами', () => {
    expect(normalizeCounterId('123;alert(1)')).toBe('')
    expect(normalizeCounterId('<img onerror=alert(1)>')).toBe('')
    expect(normalizeCounterId('<script>')).toBe('')
    expect(normalizeCounterId('987 654')).toBe('')
    expect(normalizeCounterId('abc123')).toBe('')
  })

  it('пустое и мусорное — счётчик не настроен', () => {
    expect(normalizeCounterId('')).toBe('')
    expect(normalizeCounterId(null)).toBe('')
    expect(normalizeCounterId(undefined)).toBe('')
    expect(normalizeCounterId('1'.repeat(13))).toBe('')
  })
})

describe('счётчик грузится только по согласию', () => {
  it('без согласия ничего не грузит и настройки не спрашивает', () => {
    startMetrika()
    expect(getMock).not.toHaveBeenCalled()
    expect(isMetrikaLoaded()).toBe(false)
    expect(document.getElementById('sp-metrika')).toBeNull()
  })

  it('с согласием подключается', async () => {
    grantConsent()
    startMetrika()
    // Настройки приезжают асинхронно, поэтому ждём загрузки.
    await vi.waitFor(() => {
      expect(isMetrikaLoaded()).toBe(true)
    })
    expect(getMock).toHaveBeenCalledWith('/api/v1/public/store-settings')
    expect(loadedCounterId()).toBe('987654')
    expect(document.getElementById('sp-metrika')?.getAttribute('src'))
      .toBe('https://mc.yandex.ru/metrika/tag.js')
  })

  it('согласие, данное позже, подключает счётчик', async () => {
    startMetrika()
    expect(isMetrikaLoaded()).toBe(false)
    grantConsent()
    await vi.waitFor(() => {
      expect(isMetrikaLoaded()).toBe(true)
    })
  })

  it('отзыв согласия снимает скрипт и запрещает повтор', async () => {
    grantConsent()
    startMetrika()
    await vi.waitFor(() => {
      expect(isMetrikaLoaded()).toBe(true)
    })

    revokeConsent()
    expect(isMetrikaLoaded()).toBe(false)
    expect(document.getElementById('sp-metrika')).toBeNull()
    // Повторная инициализация после отзыва не должна произойти, даже если
    // кто-то вызовет загрузку вручную.
    expect(loadMetrika('987654')).toBe(false)
  })

  it('без ID в настройках счётчик не грузится', () => {
    getMock.mockResolvedValue({ counters: { yandex_metrika_id: '' } })
    grantConsent()
    startMetrika()
    expect(isMetrikaLoaded()).toBe(false)
  })

  it('мусорный ID не грузится (иначе подключился бы чужой счётчик)', () => {
    grantConsent()
    expect(loadMetrika('<img onerror=alert(1)>')).toBe(false)
    expect(loadMetrika('123;alert(1)')).toBe(false)
    expect(document.getElementById('sp-metrika')).toBeNull()
  })

  it('недоступные настройки не ломают витрину', () => {
    getMock.mockRejectedValue(new Error('network'))
    grantConsent()
    expect(() => startMetrika()).not.toThrow()
  })
})

// Вебвизор записывает всё движение по странице, включая ввод в формы, а в
// оформлении заявки есть имя, почта и телефон. Отключение должно быть ЯВНЫМ
// в коде, а не «дефолтом сервиса»: дефолт можно вернуть сменой настроек.
describe('вебвизор выключен явно', () => {
  it('в опциях инициализации нет вебвизора и производных карт', () => {
    grantConsent()
    loadMetrika('987654')
    const ym = (window as unknown as { ym: (...a: unknown[]) => void }).ym
    expect(ym).toBeTruthy()
    const calls = (ym as unknown as { a?: unknown[][] }).a ?? []
    const init = calls.find((c) => c[1] === 'init')
    expect(init).toBeTruthy()
    const opts = init?.[2] as Record<string, unknown>
    expect(opts.webvisor).toBe(false)
    expect(opts.clickmap).toBe(false)
    expect(opts.trackLinks).toBe(false)
  })

  it('в разметке витрины нет вебвизорных атрибутов', () => {
    // Скрипт не должен сам добавлять webvisor: из корня проекта проверить
    // нельзя, но хотя бы падение на «лишних» глобальных переменных мы не
    // допускаем: unload чистит и _ym.
    grantConsent()
    loadMetrika('1')
    unloadMetrika()
    expect((window as unknown as Record<string, unknown>)._ym).toBeUndefined()
  })
})
