import { useEffect, useState } from 'react'
import type { DependencyList } from 'react'

// useScrollSpy — подсветка активной секции в липком оглавлении.
// Секции с id из `ids` наблюдаются через IntersectionObserver.
// Если IntersectionObserver недоступен (jsdom / старые браузеры) — безопасный no-op.
// deps — сигнал пере-обсерва после появления секций (например, `[loading]`).
export function useScrollSpy(ids: string[], fallback: string, deps: DependencyList = []): string {
  const [active, setActive] = useState(fallback)

  useEffect(() => {
    if (typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) setActive(entry.target.id)
        }
      },
      { rootMargin: '-20% 0px -70% 0px', threshold: 0 },
    )
    for (const id of ids) {
      const el = document.getElementById(id)
      if (el) observer.observe(el)
    }
    return () => observer.disconnect()
    // ids — стабильная константа на уровне модуля; deps — внешние сигналы.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ids.join('\u0000'), ...deps])

  return active
}