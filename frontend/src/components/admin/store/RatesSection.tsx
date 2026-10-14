// Раздел «Цены и расчёт» → ставки расчёта: станок, работа, накладные, маржа,
// скидка, НДС.
//
// Отдельная группа с отдельным правом price.rates. До разделения настроек
// лежала вперемешку с контактами и реквизитами, поэтому «выдать право на
// ставки» означало «выдать право на реквизиты» — выдать одно без другого
// было невозможно (EDR-0041 §6).
import { useCallback, useEffect, useState } from 'react'

import { storeApi } from '../../../api/store'
import type { StoreRates, StoreSettings } from '../../../api/store'
import { useLoad } from '../../../lib/useLoad'
import { RATE_MONEY_LABELS, RATE_PERCENT_LABELS } from './storeLabels'

type Status = 'idle' | 'saving' | 'saved' | 'error'

type MoneyKey = keyof typeof RATE_MONEY_LABELS
type PercentKey = keyof typeof RATE_PERCENT_LABELS

export function RatesSection() {
  const { data, error, reload } = useLoad<StoreSettings>(() => storeApi.settings(), [])
  const [draft, setDraft] = useState<StoreSettings | null>(null)
  const [status, setStatus] = useState<Status>('idle')
  const [message, setMessage] = useState('')

  const current = draft ?? data

  // Новая загрузка сбрасывает несохранённую правку, иначе после reload()
  // форма продолжила бы показывать старое поле со словами «изменено».
  useEffect(() => {
    setDraft(null)
  }, [data])

  const save = useCallback(async () => {
    if (!current) return
    setStatus('saving')
    setMessage('')
    try {
      await storeApi.saveSettingsGroup('rates', current)
      setStatus('saved')
      setMessage('Ставки сохранены')
      setDraft(null)
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }, [current, reload])

  if (error) return <p className="alert alert--error" role="alert">{error}</p>
  if (!current) return <p className="muted">Загрузка…</p>

  const setRates = (patch: Partial<StoreRates>) =>
    setDraft({ ...current, rates: { ...current.rates, ...patch } })

  return (
    <section>
      <h2>Ставки расчёта</h2>
      <p className="muted">
        Из этих ставок складывается себестоимость расчёта. Цена материала задаётся
        отдельно — в разделе «Цены материалов».
      </p>
      <fieldset>
        <legend>Стоимость работы</legend>
        {(Object.keys(RATE_MONEY_LABELS) as MoneyKey[]).map((key) => (
          <label key={key} className="field">
            <span className="field__label">{RATE_MONEY_LABELS[key]}</span>
            <input
              className="field__input"
              type="number"
              min={0}
              value={current.rates[key]}
              onChange={(e) => setRates({ [key]: Number(e.target.value) } as Partial<StoreRates>)}
            />
          </label>
        ))}
      </fieldset>
      <fieldset>
        <legend>Проценты</legend>
        {(Object.keys(RATE_PERCENT_LABELS) as PercentKey[]).map((key) => (
          <label key={key} className="field">
            <span className="field__label">{RATE_PERCENT_LABELS[key]}</span>
            <input
              className="field__input"
              type="number"
              min={0}
              max={100}
              value={current.rates[key]}
              onChange={(e) => setRates({ [key]: Number(e.target.value) } as Partial<StoreRates>)}
            />
          </label>
        ))}
      </fieldset>
      <div className="row--actions">
        <button
          type="button"
          className="btn btn--primary"
          disabled={status === 'saving'}
          onClick={() => void save()}
        >
          {status === 'saving' ? 'Сохраняем…' : 'Сохранить ставки'}
        </button>
        {status !== 'idle' && (
          <span className={status === 'error' ? 'alert--error' : 'ok-text'}>{message}</span>
        )}
      </div>
    </section>
  )
}