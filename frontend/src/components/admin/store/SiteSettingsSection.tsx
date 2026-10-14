// Раздел «Сайт и витрина» → группы настроек: контакты, реквизиты, соцсети,
// SEO со счётчиками.
//
// Почему группа, а не одна форма: настройки лежат одной записью JSON, и
// выдать «менять SEO», не выдав «менять реквизиты», было невозможно — PUT
// перезаписывал весь документ. Теперь сервер принимает группу
// (PUT /api/v1/admin/store/settings/{group}, EDR-0041 §6), поэтому каждая
// группа присылает и сохраняет только свою часть.
//
// Побочная польза: правка контактов больше не может затереть реквизиты —
// раньше форма держала копию всего документа.
import { useCallback, useEffect, useState } from 'react'

import { storeApi } from '../../../api/store'
import type { StoreSettings } from '../../../api/store'
import { useLoad } from '../../../lib/useLoad'
import {
  COMPANY_LABELS,
  CONTACT_LABELS,
  COUNTER_LABELS,
  SOCIAL_LABELS,
} from './storeLabels'

export type SettingsGroupId = 'contacts' | 'company' | 'social' | 'seo'

const GROUPS: Record<
  SettingsGroupId,
  { title: string; path: 'contacts' | 'company' | 'social' | 'seo' }
> = {
  contacts: { title: 'Контакты и адрес', path: 'contacts' },
  company: { title: 'Реквизиты компании', path: 'company' },
  social: { title: 'Соцсети', path: 'social' },
  seo: { title: 'SEO и счётчики', path: 'seo' },
}

type Status = 'idle' | 'saving' | 'saved' | 'error'

export function SiteSettingsSection({ group }: { group: SettingsGroupId }) {
  const meta = GROUPS[group]
  const { data, error, reload } = useLoad<StoreSettings>(() => storeApi.settings(), [])
  const [draft, setDraft] = useState<StoreSettings | null>(null)
  const [status, setStatus] = useState<Status>('idle')
  const [message, setMessage] = useState('')

  const current = draft ?? data

  // Новая загрузка с сервера сбрасывает несохранённую правку: иначе после
  // reload() форма продолжила бы показывать старое поле со словами
  // «изменено, но не сохранено».
  useEffect(() => {
    setDraft(null)
  }, [data])

  const save = useCallback(async () => {
    if (!current) return
    setStatus('saving')
    setMessage('')
    try {
      await storeApi.saveSettingsGroup(meta.path, current)
      setStatus('saved')
      // Подпись с названием группы: на странице плитки кнопок «Сохранить»
      // несколько, и «Сохранено» без указания какой группы ни о чём не говорит.
      setMessage(`${meta.title} сохранены`)
      setDraft(null)
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }, [current, meta.path, reload])

  if (error) return <p className="alert alert--error" role="alert">{error}</p>
  if (!current) return <p className="muted">Загрузка…</p>

  return (
    <section>
      <h2>{meta.title}</h2>
      <GroupFields
        group={group}
        settings={current}
        onChange={(next) => setDraft({ ...current, ...next })}
      />
      <div className="row--actions">
        <button
          type="button"
          className="btn btn--primary"
          disabled={status === 'saving'}
          onClick={() => void save()}
        >
          {status === 'saving' ? 'Сохраняем…' : 'Сохранить'}
        </button>
        {status !== 'idle' && <span className={status === 'error' ? 'alert--error' : 'ok-text'}>{message}</span>}
      </div>
    </section>
  )
}

function GroupFields({
  group,
  settings,
  onChange,
}: {
  group: SettingsGroupId
  settings: StoreSettings
  onChange: (next: Partial<StoreSettings>) => void
}) {
  switch (group) {
    case 'contacts':
      return (
        <fieldset>
          <legend>Как с вами связаться</legend>
          {(['phone', 'email', 'address', 'work_hours'] as const).map((key) => (
            <label key={key} className="field">
              <span className="field__label">{CONTACT_LABELS[key]}</span>
              <input
                className="field__input"
                value={settings.contacts[key]}
                onChange={(e) => onChange({ contacts: { ...settings.contacts, [key]: e.target.value } })}
              />
            </label>
          ))}
        </fieldset>
      )
    case 'company':
      return (
        <fieldset>
          <legend>Реквизиты</legend>
          {(['name', 'legal_name', 'inn', 'ogrn', 'email', 'website'] as const).map((key) => (
            <label key={key} className="field">
              <span className="field__label">{COMPANY_LABELS[key]}</span>
              <input
                className="field__input"
                value={settings.company[key]}
                onChange={(e) => onChange({ company: { ...settings.company, [key]: e.target.value } })}
              />
            </label>
          ))}
        </fieldset>
      )
    case 'social':
      return (
        <fieldset>
          <legend>Ссылки на соцсети</legend>
          {(['telegram', 'vk', 'whatsapp', 'youtube'] as const).map((key) => (
            <label key={key} className="field">
              <span className="field__label">{SOCIAL_LABELS[key]}</span>
              <input
                className="field__input"
                value={settings.social[key]}
                onChange={(e) => onChange({ social: { ...settings.social, [key]: e.target.value } })}
              />
            </label>
          ))}
        </fieldset>
      )
    case 'seo':
      return (
        <>
          <fieldset>
            <legend>SEO</legend>
            {(['default_title', 'default_description', 'og_image'] as const).map((key) => (
              <label key={key} className="field">
                <span className="field__label">{SEO_LABELS[key]}</span>
                <input
                  className="field__input"
                  value={settings.seo[key]}
                  onChange={(e) => onChange({ seo: { ...settings.seo, [key]: e.target.value } })}
                />
              </label>
            ))}
          </fieldset>
          <fieldset>
            <legend>Счётчики</legend>
            {(['yandex_metrika_id', 'ga4_measurement_id'] as const).map((key) => (
              <label key={key} className="field">
                <span className="field__label">{COUNTER_LABELS[key]}</span>
                <input
                  className="field__input"
                  value={settings.counters[key]}
                  onChange={(e) => onChange({ counters: { ...settings.counters, [key]: e.target.value } })}
                />
              </label>
            ))}
          </fieldset>
        </>
      )
  }
}

const SEO_LABELS: Record<'default_title' | 'default_description' | 'og_image', string> = {
  default_title: 'Заголовок сайта',
  default_description: 'Описание сайта',
  og_image: 'Картинка для ссылок (OG)',
}