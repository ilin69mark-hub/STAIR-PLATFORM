// Раздел «Магазин» → Ограждение: закупочный прайс покупных деталей, ₽ за штуку
// (CONF-PURCHASED).
//
// Устроен как «Герметик»: позиции берутся из каталога движка, магазин
// переопределяет цену, «Сбросить» возвращает встроенную ставку. Отдельный
// экран — потому что единица закупки другая: ₽/кг у листа, ₽/картридж у
// герметика, ₽/шт у стойки.
//
// Ключ прайса — ВИД детали, а не номер позиции. В спецификации у каждой стойки
// свой номер (PST-01, PST-02, …), и прайс по нему построить нельзя: стойка
// стоит столько-то независимо от того, сколько их на марше. Поэтому в таблице
// вид детали, а каталожный код — справочно.

import { useState } from 'react'
import { storeApi, type PurchasedPrice } from '../../../api/store'
import { useLoad } from '../../../lib/useLoad'
import { ErrorBox, Loading } from './SectionFeedback'

type Status = 'idle' | 'loading' | 'saving' | 'saved' | 'error'

const KIND_LABELS: Record<string, string> = {
  rail_top: 'поручень',
  post: 'стойка',
  rigel: 'ригель',
  tip: 'наконечник',
  flange: 'фланец',
  slip: 'муфта',
  deco: 'накладка',
  end_cap: 'заглушка',
}

export function PurchasedSection() {
  const { data, error, reload } = useLoad<PurchasedPrice[]>(() => storeApi.purchased(), [])
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [status, setStatus] = useState<Status>('idle')
  const [message, setMessage] = useState('')

  // Хуки объявлены до ранних return: вызывать useState после условия нельзя,
  // иначе первая же загрузка данных ломает порядок хуков.
  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />

  const priceOf = (p: PurchasedPrice) => draft[p.kind] ?? String(p.price_per_pc_rub)

  const save = async (kind: string) => {
    setStatus('saving')
    setMessage('')
    try {
      const raw = priceOf(data.find((item) => item.kind === kind)!)
      if (raw.trim() === '') throw new Error('Укажите цену детали')
      const price = Number(raw)
      if (!Number.isFinite(price) || price <= 0) {
        throw new Error('Цена должна быть положительным числом')
      }
      await storeApi.setPurchasedPrice(kind, Math.round(price))
      setDraft((d) => {
        const next = { ...d }
        delete next[kind]
        return next
      })
      setStatus('saved')
      setMessage(`Цена «${KIND_LABELS[kind] ?? kind}» сохранена`)
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }

  const reset = async (kind: string) => {
    setStatus('saving')
    setMessage('')
    try {
      await storeApi.resetPurchasedPrice(kind)
      setDraft((d) => {
        const next = { ...d }
        delete next[kind]
        return next
      })
      setStatus('saved')
      setMessage(`Цена «${KIND_LABELS[kind] ?? kind}» возвращена к встроенной ставке`)
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <section>
      <h2>Ограждение: закупка</h2>
      <p className="hint">
        Цена в ₽ за штуку. До появления этого прайса все 29 покупных позиций
        ограждения попадали в смету с нулевой суммой: цены не существовало ни в
        коде, ни в базе, и расчёт отдавал смету без ограждения. Пока строка
        прайса не задана, действует встроенная ставка 250 ₽/шт — её достаточно,
        чтобы смета не обнулялась, но она не заменяет закупочную. Состав
        каталога задаётся в коде, и новая позиция появится здесь сама.
      </p>

      <table>
        <thead>
          <tr>
            <th>Деталь</th>
            <th>Вид</th>
            <th>Код каталога</th>
            <th>Цена, ₽/шт</th>
            <th>Источник</th>
            <th>Действия</th>
          </tr>
        </thead>
        <tbody>
          {data.map((p) => (
            <tr key={p.kind}>
              <td>{p.name_ru}</td>
              <td>
                <code>{p.kind}</code>
              </td>
              <td>
                <code>{p.catalog_id}</code>
              </td>
              <td>
                <input
                  aria-label={`Цена ${p.name_ru}`}
                  type="number"
                  min={1}
                  step={1}
                  value={priceOf(p)}
                  onChange={(e) => setDraft((d) => ({ ...d, [p.kind]: e.target.value }))}
                />
              </td>
              <td>{p.overridden ? 'задана магазином' : 'встроенная ставка'}</td>
              <td className="actions">
                <button type="button" onClick={() => void save(p.kind)} disabled={status === 'saving'}>
                  Сохранить
                </button>
                {p.overridden && (
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => void reset(p.kind)}
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

      {data.length === 0 && <p className="hint">Каталог покупных деталей пуст.</p>}
      {message && <p className={status === 'error' ? 'error' : 'ok'}>{message}</p>}
    </section>
  )
}