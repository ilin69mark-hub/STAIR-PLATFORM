// useScrollSpy — подсветка активной секции через IntersectionObserver.
// jsdom не реализует IntersectionObserver, поэтому в тестах ставим его заглушку.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { useScrollSpy } from './useScrollSpy'

type Entry = { target: { id: string }; isIntersecting: boolean }

class MockIntersectionObserver {
  static instances: MockIntersectionObserver[] = []
  callback: (entries: Entry[]) => void
  observed: string[] = []
  disconnected = false

  constructor(cb: (entries: Entry[]) => void) {
    this.callback = cb
    MockIntersectionObserver.instances.push(this)
  }

  observe(el: Element) {
    this.observed.push(el.id)
  }

  unobserve() {}

  disconnect() {
    this.disconnected = true
  }

  trigger(id: string) {
    this.callback([{ target: { id }, isIntersecting: true }])
  }
}

function stubObserver() {
  MockIntersectionObserver.instances = []
  vi.stubGlobal('IntersectionObserver', MockIntersectionObserver)
}

afterEach(() => {
  MockIntersectionObserver.instances = []
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

describe('useScrollSpy', () => {
  it('сразу возвращает fallback и не падает без IntersectionObserver', () => {
    const { result } = renderHook(() => useScrollSpy(['a', 'b'], 'a'))
    expect(result.current).toBe('a')
  })

  it('наблюдает секции из ids', () => {
    stubObserver()
    document.body.innerHTML = '<section id="a"></section><section id="b"></section>'
    renderHook(() => useScrollSpy(['a', 'b'], 'a'))
    const inst = MockIntersectionObserver.instances[0]
    expect(inst.observed).toEqual(['a', 'b'])
  })

  it('пересекающаяся секция становится активной', () => {
    stubObserver()
    document.body.innerHTML = '<section id="a"></section><section id="b"></section>'
    const { result } = renderHook(() => useScrollSpy(['a', 'b'], 'a'))
    act(() => MockIntersectionObserver.instances[0].trigger('b'))
    expect(result.current).toBe('b')
  })

  it('не наблюдает несуществующие ids', () => {
    stubObserver()
    document.body.innerHTML = '<section id="a"></section>'
    renderHook(() => useScrollSpy(['a', 'nope'], 'a'))
    expect(MockIntersectionObserver.instances[0].observed).toEqual(['a'])
  })

  it('disconnect при размонтировании', () => {
    stubObserver()
    document.body.innerHTML = '<section id="a"></section>'
    const { unmount } = renderHook(() => useScrollSpy(['a'], 'a'))
    const inst = MockIntersectionObserver.instances[0]
    unmount()
    expect(inst.disconnected).toBe(true)
  })

  it('смена deps пересоздаёт observer и наблюдает заново', () => {
    stubObserver()
    document.body.innerHTML = '<section id="a"></section><section id="b"></section>'
    const { rerender } = renderHook(({ deps }) => useScrollSpy(['a', 'b'], 'a', deps), {
      initialProps: { deps: [false] as unknown[] },
    })
    expect(MockIntersectionObserver.instances).toHaveLength(1)
    rerender({ deps: [true] as unknown[] })
    expect(MockIntersectionObserver.instances).toHaveLength(2)
    expect(MockIntersectionObserver.instances[1].observed).toEqual(['a', 'b'])
  })
})