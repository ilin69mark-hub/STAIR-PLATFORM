// Раздел «Магазин» → Цены материалов и Настройки магазина.
// Перенесено из отдельного приложения frontend-store-admin при слиянии
// кабинетов; локальный useAsync заменён на общий lib/useLoad.

import { useState } from 'react'
import { storeApi, type MaterialPrice } from '../../../api/store'
import { useLoad } from '../../../lib/useLoad'
import { ErrorBox, Loading } from './SectionFeedback'

// Состояние кнопки сохранения (не загрузка данных — этим занимается useLoad).
type Status = 'idle' | 'loading' | 'saving' | 'saved' | 'error'

// PricesSection — прайс материалов магазина (₽/кг): правка, сброс к встроенной
// ставке движка. Сброс и сохранение сбрасывают кэш публичного каталога на
// бэкенде, поэтому новая цена видна на витрине сразу.
export function PricesSection() {
  const { data, error, reload } = useLoad<MaterialPrice[]>(() => storeApi.prices(), [])
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [status, setStatus] = useState<Status>('idle')
  const [message, setMessage] = useState('')

  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />

  const save = async (code: string) => {
    setStatus('saving')
    setMessage('')
    try {
      const rawPrice = draft[code] ?? String(data.find((item) => item.code === code)?.price_per_kg_rub ?? '')
      if (rawPrice.trim() === '') {
        throw new Error('Укажите цену материала')
      }
      const price = Number(rawPrice)
      if (!Number.isFinite(price) || price < 0) {
        throw new Error('Цена должна быть неотрицательным числом')
      }
      await storeApi.setPrice(code, Math.round(price))
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
      await storeApi.resetPrice(code)
      setDraft((current) => {
        const next = { ...current }
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
      <h2>Цены материалов</h2>
      <p className="hint">
        Цена в ₽ за кг. Пустое значение у не переопределённого материала — действует встроенная
        ставка расчётного движка.
      </p>
      <table>
        <thead>
          <tr>
            <th>Код</th>
            <th>Цена, ₽/кг</th>
            <th>Источник</th>
            <th>Действия</th>
          </tr>
        </thead>
        <tbody>
          {data.map((p) => (
            <tr key={p.code}>
              <td>
                <code>{p.code}</code>
              </td>
              <td>
                <input
                  aria-label={`Цена ${p.code}`}
                  type="number"
                  min={0}
                  step={1}
                  value={draft[p.code] ?? String(p.price_per_kg_rub)}
                  onChange={(e) => setDraft((d) => ({ ...d, [p.code]: e.target.value }))}
                />
              </td>
              <td>{p.overridden ? 'задана магазином' : 'встроенная ставка'}</td>
              <td className="actions">
                <button type="button" onClick={() => void save(p.code)} disabled={status === 'saving'}>
                  Сохранить
                </button>
                {p.overridden && (
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => void reset(p.code)}
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
      {message && <p className={status === 'error' ? 'error' : 'ok'}>{message}</p>}
    </section>
  )
}
