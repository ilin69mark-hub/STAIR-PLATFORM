// Раздел «Магазин» → Герметик: прайс герметика в рублях за картридж
// (CONF-SEALANT).
//
// Устроен как «Крепёж» (FastenersSection): позиции берутся из встроенного
// каталога движка, магазин переопределяет цену, кнопка «Сбросить»
// возвращает встроенную ставку. Отдельный экран, а не вторая таблица в
// «Ценах материалов», потому что единица закупки другая: ₽/кг у листа,
// ₽/шт у дюбеля, ₽/картридж у герметика.

import { useState } from 'react'
import { storeApi, type SealantPrice } from '../../../api/store'
import { useLoad } from '../../../lib/useLoad'
import { ErrorBox, Loading } from './SectionFeedback'

type Status = 'idle' | 'loading' | 'saving' | 'saved' | 'error'

const KIND_LABELS: Record<string, string> = {
  silicone: 'силикон',
  pu: 'полиуретан',
}

export function SealantsSection() {
  const { data, error, reload } = useLoad<SealantPrice[]>(() => storeApi.sealants(), [])
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [status, setStatus] = useState<Status>('idle')
  const [message, setMessage] = useState('')

  // Хуки объявлены до ранних return: вызывать useState после условия нельзя,
  // иначе первая же загрузка данных ломает порядок хуков.
  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />

  const priceOf = (s: SealantPrice) => draft[s.code] ?? String(s.price_per_cartridge_rub)

  const save = async (code: string) => {
    setStatus('saving')
    setMessage('')
    try {
      const raw = priceOf(data.find((item) => item.code === code)!)
      if (raw.trim() === '') throw new Error('Укажите цену герметика')
      const price = Number(raw)
      if (!Number.isFinite(price) || price <= 0) {
        throw new Error('Цена должна быть положительным числом')
      }
      await storeApi.setSealantPrice(code, Math.round(price))
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
      await storeApi.resetSealantPrice(code)
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
      <h2>Герметик</h2>
      <p className="hint">
        Цена в ₽ за картридж. Расход герметика считается по формуле V = S × t:
        площадь швов берётся из геометрии обоймы выреза и торца косоура, t —
        толщина слоя. Объём переводится в картриджи с округлением вверх, потому
        что купить 2,08 картриджа нельзя. Здесь меняется цена за картридж;
        состав каталога задаётся в коде, и новая позиция появится здесь сама.
      </p>

      <table>
        <thead>
          <tr>
            <th>Позиция</th>
            <th>Код</th>
            <th>Картридж</th>
            <th>Цена, ₽/шт</th>
            <th>Источник</th>
            <th>Действия</th>
          </tr>
        </thead>
        <tbody>
          {data.map((s) => (
            <tr key={s.code}>
              <td>
                {s.name_ru}
                {KIND_LABELS[s.kind] && (
                  <span className="muted"> · {KIND_LABELS[s.kind]}</span>
                )}
              </td>
              <td>
                <code>{s.code}</code>
              </td>
              <td>{s.volume_ml} мл</td>
              <td>
                <input
                  aria-label={`Цена ${s.name_ru}`}
                  type="number"
                  min={1}
                  step={1}
                  value={priceOf(s)}
                  onChange={(e) => setDraft((d) => ({ ...d, [s.code]: e.target.value }))}
                />
              </td>
              <td>{s.overridden ? 'задана магазином' : 'встроенная ставка'}</td>
              <td className="actions">
                <button type="button" onClick={() => void save(s.code)} disabled={status === 'saving'}>
                  Сохранить
                </button>
                {s.overridden && (
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => void reset(s.code)}
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

      {data.length === 0 && <p className="hint">Каталог герметика пуст.</p>}
      {message && <p className={status === 'error' ? 'error' : 'ok'}>{message}</p>}
    </section>
  )
}