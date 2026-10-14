import { useCallback, useEffect, useState } from 'react'

/**
 * useLoad — единственный загрузчик данных в админ-панели.
 *
 * Раньше одинаковые загрузчики жили в трёх местах (`useLoad`,
 * `useLoadRethrow`, `useAsync` в панели магазина) и разошлись: один без
 * перезагрузки, другой без защиты от гонки. Здесь всё в одном —
 * `reload` для ручного перезапроса после мутации, `alive`-флаг против
 * обновления состояния после размонтирования.
 *
 * deps передаётся вторым аргументом и участвует в пересчёте; ручной
 * `reload()` перезапускает загрузку при неизменных deps.
 */
export function useLoad<T>(load: () => Promise<T>, deps: unknown[]): {
  data: T | null
  error: string
  loading: boolean
  reload: () => void
} {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [nonce, setNonce] = useState(0)

  const reload = useCallback(() => setNonce((n) => n + 1), [])

  useEffect(() => {
    let alive = true
    setError('')
    setLoading(true)
    load()
      .then((res) => {
        if (alive) setData(res)
      })
      .catch((e: unknown) => {
        if (alive) setError(e instanceof Error ? e.message : String(e))
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, nonce])

  return { data, error, loading, reload }
}