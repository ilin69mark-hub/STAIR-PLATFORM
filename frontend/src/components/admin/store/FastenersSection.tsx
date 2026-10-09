// Раздел «Магазин» → Крепёж: прайс дюбелей и глухарей в рублях за штуку
// (MNT-0001, решение владельца 2026-10-08).
//
// Отдельный экран, а не ещё одна колонка в «Ценах материалов», по двум
// причинам. Первая — единица: материал в рублях за килограмм, крепёж в
// рублях за штуку, и в одной таблице «цена» означала бы то одно, то
// другое. Вторая — количество: позиций крепежа около сорока, материалов
// пять, и общий список на 45 строк перестал бы читаться.

import { useMemo, useState } from 'react'
import { storeApi, type FastenerPrice } from '../../../api/store'
import { useLoad } from '../../../lib/useLoad'
import { ErrorBox, Loading } from './SectionFeedback'

type Status = 'idle' | 'loading' | 'saving' | 'saved' | 'error'

type KindFilter = 'all' | 'screw' | 'plug'

const KIND_LABELS: Record<string, string> = {
  all: 'Все позиции',
  screw: 'Глухари',
  plug: 'Дюбели',
}

export function FastenersSection() {
  const { data, error, reload } = useLoad<FastenerPrice[]>(() => storeApi.fasteners(), [])
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [status, setStatus] = useState<Status>('idle')
  const [message, setMessage] = useState('')
  const [kind, setKind] = useState<KindFilter>('all')
  const [query, setQuery] = useState('')

  // useMemo идёт ДО ранних return: хуки нельзя вызывать условно, иначе
  // React ловит «Rendered more hooks than during the previous render» при
  // первой же загрузке данных.
  //
  // Фильтр по виду и поиск по названию/коду. Сортировка — по размеру, а не
  // по алфавиту: монтажник ищет «10х60», и в списке, отсортированном по коду,
  // это находится с трудом (PLUG-10x100 стоит раньше PLUG-10x60).
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    return (data ?? [])
      .filter((f) => kind === 'all' || f.kind === kind)
      .filter((f) => !q || f.name_ru.toLowerCase().includes(q) || f.code.toLowerCase().includes(q))
      .slice()
      .sort((a, b) => a.diameter_mm - b.diameter_mm || a.length_mm - b.length_mm)
  }, [data, kind, query])

  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />

  const priceOf = (f: FastenerPrice) => draft[f.code] ?? String(f.price_per_pc_rub)

  const save = async (code: string) => {
    setStatus('saving')
    setMessage('')
    try {
      const raw = priceOf(data.find((item) => item.code === code)!)
      if (raw.trim() === '') throw new Error('Укажите цену крепежа')
      const price = Number(raw)
      if (!Number.isFinite(price) || price <= 0) {
        throw new Error('Цена должна быть положительным числом')
      }
      await storeApi.setFastenerPrice(code, Math.round(price))
      setDraft((d) => {
        const next = { ...d }
        delete next[code]
        return next
      })
      setStatus('saved')
      setMessage(`Цена ${code} сохранена`)
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }

  const reset = async (code: string) => {
    setStatus('saving')
    setMessage('')
    try {
      await storeApi.resetFastenerPrice(code)
      setDraft((d) => {
        const next = { ...d }
        delete next[code]
        return next
      })
      setStatus('saved')
      setMessage(`Цена ${code} возвращена к встроенной ставке`)
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <section>
      <h2>Крепёж</h2>
      <p className="hint">
        Цена в ₽ за штуку. Крепёж попадает в смету автоматически по отверстиям пяток:
        глухарь 6х35 на пятки подступенка и ступеней, глухарь 10х60 на пятку пола и пятку
        выхода на перекрытие, плюс дюбель 10х60 к нему, если основание бетонное. Здесь
        меняется цена за штуку; состав каталога задаётся в коде, и новая позиция появится
        здесь сама.
      </p>

      <div className="row--actions">
        <label className="field">
          <span className="field__label">Вид</span>
          <select
            className="field__input"
            value={kind}
            onChange={(e) => setKind(e.target.value as KindFilter)}
          >
            {(Object.keys(KIND_LABELS) as KindFilter[]).map((k) => (
              <option key={k} value={k}>
                {KIND_LABELS[k]}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span className="field__label">Поиск</span>
          <input
            className="field__input"
            type="search"
            placeholder="10х60, дюбель, глухарь…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
      </div>

      <table>
        <thead>
          <tr>
            <th>Позиция</th>
            <th>Код</th>
            <th>Цена, ₽/шт</th>
            <th>Источник</th>
            <th>Действия</th>
          </tr>
        </thead>
        <tbody>
          {visible.map((f) => (
            <tr key={f.code}>
              <td>{f.name_ru}</td>
              <td>
                <code>{f.code}</code>
              </td>
              <td>
                <input
                  aria-label={`Цена ${f.name_ru}`}
                  type="number"
                  min={1}
                  step={1}
                  value={priceOf(f)}
                  onChange={(e) => setDraft((d) => ({ ...d, [f.code]: e.target.value }))}
                />
              </td>
              <td>{f.overridden ? 'задана магазином' : 'встроенная ставка'}</td>
              <td className="actions">
                <button type="button" onClick={() => void save(f.code)} disabled={status === 'saving'}>
                  Сохранить
                </button>
                {f.overridden && (
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => void reset(f.code)}
                    disabled={status === 'saving'}
                  >
                    Сбросить
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {visible.length === 0 && (
        <p className="hint">
          {query ? `Ничего не найдено по запросу «${query}».` : 'Под этот фильтр ничего не попало.'}
        </p>
      )}
      {message && <p className={status === 'error' ? 'error' : 'ok'}>{message}</p>}
    </section>
  )
}