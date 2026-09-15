import { act, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LazySection } from './LazySection'

class MockIntersectionObserver {
  static instances: MockIntersectionObserver[] = []
  private cb: IntersectionObserverCallback
  private targets: Element[] = []
  readonly root = null
  readonly rootMargin = ''
  readonly thresholds: number[] = []

  constructor(cb: IntersectionObserverCallback) {
    this.cb = cb
    MockIntersectionObserver.instances.push(this)
  }
  observe(target: Element) {
    this.targets.push(target)
  }
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] {
    return []
  }
  triggerAll() {
    this.cb(
      this.targets.map((t) => ({ isIntersecting: true, target: t }) as IntersectionObserverEntry),
      this as unknown as IntersectionObserver,
    )
  }
}

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  MockIntersectionObserver.instances = []
})

describe('LazySection', () => {
  it('грузит данные сразу, если IntersectionObserver недоступен', () => {
    const onLoad = vi.fn()
    render(
      <LazySection onLoad={onLoad}>
        <p>контент</p>
      </LazySection>,
    )
    expect(onLoad).toHaveBeenCalledTimes(1)
    expect(screen.getByText('контент')).toBeInTheDocument()
  })

  it('не рендерит контент, пока секция не появилась во вьюпорте', () => {
    vi.stubGlobal('IntersectionObserver', MockIntersectionObserver)
    const onLoad = vi.fn()
    const { container } = render(
      <LazySection onLoad={onLoad}>
        <p>контент</p>
      </LazySection>,
    )
    expect(onLoad).not.toHaveBeenCalled()
    expect(screen.queryByText('контент')).not.toBeInTheDocument()
    expect(container.innerHTML).not.toContain('контент')
  })

  it('грузит данные один раз после пересечения', async () => {
    vi.stubGlobal('IntersectionObserver', MockIntersectionObserver)
    const onLoad = vi.fn()
    render(
      <LazySection onLoad={onLoad}>
        <p>контент</p>
      </LazySection>,
    )
    expect(onLoad).not.toHaveBeenCalled()
    act(() => MockIntersectionObserver.instances[0].triggerAll())
    expect(await screen.findByText('контент')).toBeInTheDocument()
    expect(onLoad).toHaveBeenCalledTimes(1)
  })
})