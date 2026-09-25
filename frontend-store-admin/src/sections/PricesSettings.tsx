import { useCallback, useEffect, useState } from 'react'
import { storeApi, type MaterialPrice, type StoreSettings } from '../api/store'

type Status = 'idle' | 'loading' | 'saving' | 'saved' | 'error'

function useAsync<T>(load: () => Promise<T>, deps: unknown[]): {
  data: T | null
  error: string
  reload: () => void
} {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [nonce, setNonce] = useState(0)
  const reload = useCallback(() => setNonce((n) => n + 1), [])

  useEffect(() => {
    let alive = true
    setError('')
    load()
      .then((res) => {
        if (alive) setData(res)
      })
      .catch((e: unknown) => {
        if (alive) setError(e instanceof Error ? e.message : String(e))
      })
    return () => {
      alive = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, nonce])

  return { data, error, reload }
}

function field<K extends keyof StoreSettings>(
  settings: StoreSettings,
  key: K,
): StoreSettings[K] {
  return settings[key]
}

// PricesSection — прайс материалов магазина (₽/кг): правка, сброс к встроенной
// ставке движка. Сброс и сохранение сбрасывают кэш публичного каталога на
// бэкенде, поэтому новая цена видна на витрине сразу.
export function PricesSection() {
  const { data, error, reload } = useAsync<MaterialPrice[]>(() => storeApi.prices(), [])
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

type TextRatesKey = 'machine_per_hour_rub' | 'labor_per_hour_rub'
type PercentRatesKey =
  | 'overhead_percent'
  | 'margin_percent'
  | 'discount_percent'
  | 'tax_percent'

// SettingsSection — настройки магазина: контакты, реквизиты, соцсети, SEO,
// счётчики и параметры расчёта. Публичные ручки отдают только безопасное
// подмножество, поэтому цена и ставки редактируются здесь, а не на витрине.
export function SettingsSection() {
  const { data, error, reload } = useAsync<StoreSettings>(() => storeApi.settings(), [])
  const [draft, setDraft] = useState<StoreSettings | null>(null)
  const [status, setStatus] = useState<Status>('idle')
  const [message, setMessage] = useState('')

  const current = draft ?? data
  if (error) return <ErrorBox text={error} />
  if (!current) return <Loading />

  const setSection = <K extends keyof StoreSettings>(key: K, value: StoreSettings[K]) =>
    setDraft({ ...current, [key]: value })

  const save = async () => {
    setStatus('saving')
    setMessage('')
    try {
      await storeApi.saveSettings(current)
      setStatus('saved')
      setMessage('Настройки сохранены')
      setDraft(null)
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <section>
      <h2>Настройки магазина</h2>
      <fieldset>
        <legend>Контакты</legend>
        {(['phone', 'email', 'address', 'work_hours'] as const).map((key) => (
          <label key={key}>
            {CONTACT_LABELS[key]}
            <input
              value={current.contacts[key]}
              onChange={(e) => setSection('contacts', { ...current.contacts, [key]: e.target.value })}
            />
          </label>
        ))}
      </fieldset>
      <fieldset>
        <legend>Реквизиты</legend>
        {(['name', 'legal_name', 'inn', 'ogrn', 'email', 'website'] as const).map((key) => (
          <label key={key}>
            {COMPANY_LABELS[key]}
            <input
              value={current.company[key]}
              onChange={(e) => setSection('company', { ...current.company, [key]: e.target.value })}
            />
          </label>
        ))}
      </fieldset>
      <fieldset>
        <legend>Соцсети</legend>
        {(['telegram', 'vk', 'whatsapp', 'youtube'] as const).map((key) => (
          <label key={key}>
            {SOCIAL_LABELS[key]}
            <input
              value={current.social[key]}
              onChange={(e) => setSection('social', { ...current.social, [key]: e.target.value })}
            />
          </label>
        ))}
      </fieldset>
      <fieldset>
        <legend>SEO</legend>
        <label>
          Заголовок по умолчанию
          <input
            value={current.seo.default_title}
            onChange={(e) =>
              setSection('seo', { ...current.seo, default_title: e.target.value })
            }
          />
        </label>
        <label>
          Описание по умолчанию
          <textarea
            value={current.seo.default_description}
            onChange={(e) =>
              setSection('seo', { ...current.seo, default_description: e.target.value })
            }
          />
        </label>
        <label>
          OG-изображение (URL)
          <input
            value={current.seo.og_image}
            onChange={(e) => setSection('seo', { ...current.seo, og_image: e.target.value })}
          />
        </label>
      </fieldset>
      <fieldset>
        <legend>Счётчики</legend>
        {(['yandex_metrika_id', 'ga4_measurement_id'] as const).map((key) => (
          <label key={key}>
            {COUNTER_LABELS[key]}
            <input
              value={current.counters[key]}
              onChange={(e) => setSection('counters', { ...current.counters, [key]: e.target.value })}
            />
          </label>
        ))}
      </fieldset>
      <fieldset>
        <legend>Параметры расчёта</legend>
        {(['machine_per_hour_rub', 'labor_per_hour_rub'] as const).map((key) => (
          <label key={key}>
            {RATE_MONEY_LABELS[key]}
            <input
              type="number"
              min={0}
              value={field(current, 'rates')[key as TextRatesKey]}
              onChange={(e) =>
                setSection('rates', {
                  ...current.rates,
                  [key]: Number(e.target.value),
                })
              }
            />
          </label>
        ))}
        {(['overhead_percent', 'margin_percent', 'discount_percent', 'tax_percent'] as const).map(
          (key) => (
            <label key={key}>
              {RATE_PERCENT_LABELS[key]}
              <input
                type="number"
                min={0}
                max={100}
                step={0.1}
                value={field(current, 'rates')[key as PercentRatesKey]}
                onChange={(e) =>
                  setSection('rates', {
                    ...current.rates,
                    [key]: Number(e.target.value),
                  })
                }
              />
            </label>
          ),
        )}
      </fieldset>
      <div className="actions">
        <button type="button" onClick={() => void save()} disabled={status === 'saving'}>
          Сохранить настройки
        </button>
        {data && (
          <button type="button" className="secondary" onClick={() => setDraft(null)}>
            Отменить
          </button>
        )}
      </div>
      {message && <p className={status === 'error' ? 'error' : 'ok'}>{message}</p>}
      {data?.updated_at && <p className="hint">Обновлено: {new Date(data.updated_at).toLocaleString('ru-RU')}</p>}
    </section>
  )
}

const CONTACT_LABELS: Record<keyof StoreSettings['contacts'], string> = {
  phone: 'Телефон',
  email: 'Email',
  address: 'Адрес',
  work_hours: 'Часы работы',
}

const COMPANY_LABELS: Record<keyof StoreSettings['company'], string> = {
  name: 'Название',
  legal_name: 'Юридическое лицо',
  inn: 'ИНН',
  ogrn: 'ОГРН',
  email: 'Email для писем',
  website: 'Сайт',
}

const SOCIAL_LABELS: Record<keyof StoreSettings['social'], string> = {
  telegram: 'Telegram',
  vk: 'VK',
  whatsapp: 'WhatsApp',
  youtube: 'YouTube',
}

const COUNTER_LABELS: Record<keyof StoreSettings['counters'], string> = {
  yandex_metrika_id: 'Яндекс.Метрика (ID)',
  ga4_measurement_id: 'GA4 (Measurement ID)',
}

const RATE_MONEY_LABELS: Record<TextRatesKey, string> = {
  machine_per_hour_rub: 'Станок, ₽/час',
  labor_per_hour_rub: 'Работа, ₽/час',
}

const RATE_PERCENT_LABELS: Record<PercentRatesKey, string> = {
  overhead_percent: 'Накладные, %',
  margin_percent: 'Маржа, %',
  discount_percent: 'Скидка, %',
  tax_percent: 'НДС, %',
}

export function Loading() {
  return <p className="hint">Загрузка…</p>
}

export function ErrorBox({ text }: { text: string }) {
  return <p className="error">{text}</p>
}
