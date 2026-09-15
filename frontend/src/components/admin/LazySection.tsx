import { useEffect, useRef, useState } from 'react'
import type { ReactNode, RefObject } from 'react'

interface InViewResult<T extends Element> {
  ref: RefObject<T | null>
  inView: boolean
}

function useInView<T extends Element>(rootMargin = '300px'): InViewResult<T> {
  const ref = useRef<T | null>(null)
  const [inView, setInView] = useState(() => typeof IntersectionObserver === 'undefined')

  useEffect(() => {
    if (typeof IntersectionObserver === 'undefined') {
      return
    }
    const el = ref.current
    if (!el) {
      return
    }
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setInView(true)
            observer.disconnect()
          }
        }
      },
      { rootMargin },
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [rootMargin])

  return { ref, inView }
}

interface Props {
  onLoad: () => void
  children: ReactNode
}

export function LazySection({ onLoad, children }: Props) {
  const { ref, inView } = useInView<HTMLDivElement>()
  const loaded = useRef(false)

  useEffect(() => {
    if (inView && !loaded.current) {
      loaded.current = true
      onLoad()
    }
  }, [inView, onLoad])

  return <div ref={ref}>{inView ? children : null}</div>
}