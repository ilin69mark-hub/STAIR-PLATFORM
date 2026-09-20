// session bus (EDR-0040 Session Management): централизованная реакция на 401.
// onUnauthorized/fireUnauthorized/isAuthEndpoint — раньше покрывались только
// e2e; здесь юнит-покрытие потока «подписка → 401 → сброс».
import { afterEach, describe, expect, it, vi } from 'vitest'
import { fireUnauthorized, isAuthEndpoint, onUnauthorized } from './session'

// Без экспортируемого ресета модульного массива handlers изоляция держится
// на гарантированной отписке всего, что зарегистрировано в тесте.
let unsubscribers: Array<() => void> = []

function subscribe(handler: () => void): () => void {
  const unsubscribe = onUnauthorized(handler)
  unsubscribers.push(unsubscribe)
  return unsubscribe
}

afterEach(() => {
  for (const unsubscribe of unsubscribers) unsubscribe()
  unsubscribers = []
})

describe('onUnauthorized', () => {
  it('регистрирует обработчик: fireUnauthorized его вызывает', () => {
    const handler = vi.fn()
    subscribe(handler)
    fireUnauthorized()
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('возвращает функцию отписки: после вызова обработчик не вызывается', () => {
    const handler = vi.fn()
    const unsubscribe = subscribe(handler)
    unsubscribe()
    fireUnauthorized()
    expect(handler).not.toHaveBeenCalled()
  })

  it('отписка идемпотентна и не роняет других подписчиков', () => {
    const a = vi.fn()
    const b = vi.fn()
    const unsubscribeA = subscribe(a)
    subscribe(b)
    unsubscribeA()
    unsubscribeA()
    fireUnauthorized()
    expect(a).not.toHaveBeenCalled()
    expect(b).toHaveBeenCalledTimes(1)
  })
})

describe('fireUnauthorized', () => {
  it('с пустым списком подписчиков — тихий no-op', () => {
    expect(() => fireUnauthorized()).not.toThrow()
  })

  it('вызывает всех подписчиков в порядке регистрации', () => {
    const order: string[] = []
    subscribe(() => order.push('first'))
    subscribe(() => order.push('second'))
    fireUnauthorized()
    expect(order).toEqual(['first', 'second'])
  })

  it('упавший обработчик не мешает остальным и не пробрасывается наружу', () => {
    const faulty = vi.fn(() => {
      throw new Error('boom')
    })
    const healthy = vi.fn()
    subscribe(faulty)
    subscribe(healthy)
    expect(() => fireUnauthorized()).not.toThrow()
    expect(faulty).toHaveBeenCalledTimes(1)
    expect(healthy).toHaveBeenCalledTimes(1)
  })

  it('работает по снапшоту: отписка во время срабатывания не ломает итерацию', () => {
    let unsubscribeOther: () => void = () => {}
    const other = vi.fn()
    const first = vi.fn(() => {
      // во время fireUnauthorized отписываем второго подписчика
      unsubscribeOther()
    })
    subscribe(first)
    unsubscribeOther = subscribe(other)
    fireUnauthorized()
    expect(first).toHaveBeenCalledTimes(1)
    expect(other).toHaveBeenCalledTimes(1)
  })
})

describe('isAuthEndpoint', () => {
  it('true для /api/v1/auth/*', () => {
    expect(isAuthEndpoint('/api/v1/auth/login')).toBe(true)
    expect(isAuthEndpoint('/api/v1/auth/register')).toBe(true)
    expect(isAuthEndpoint('/api/v1/auth/refresh')).toBe(true)
    expect(isAuthEndpoint('http://localhost:8080/api/v1/auth/me')).toBe(true)
  })

  it('false для защищённых и прочих эндпоинтов', () => {
    expect(isAuthEndpoint('/api/v1/projects')).toBe(false)
    expect(isAuthEndpoint('/api/v1/orders')).toBe(false)
    expect(isAuthEndpoint('/api/v2/auth/login')).toBe(false)
    expect(isAuthEndpoint('/api/v1/authx/login')).toBe(false)
    expect(isAuthEndpoint('')).toBe(false)
  })
})