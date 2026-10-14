// VariationPicker — выбор альтернативной конфигурации (варианты A/B/C и
// снапшоты «моих вариантов»).
//
// Раньше это была галерея: один вариант крупно, остальные миниатюрами, плюс
// подпись «Выберите вариант — применится как превью» и метка «Выбран» на
// активной карточке. Владелец верстал это в узкой панели 460px, где главная
// карточка сжималась до ~250px, и подписи вариантов наезжали друг на друга и
// обрезались. Разметка «главная + миниатюры» тут вообще не нужна: вариантов
// два-три, и человек выбирает один.
//
// Теперь это простой список строк во всю ширину, текст переносится, ничего не
// обрезается. После клика вариант ПРИМЕНЯЕТСЯ и список сворачивается в одну
// строку «вариант + Изменить»: это и есть подтверждение выбора — остальное
// скрывается, а вернуться можно одной кнопкой (владелец: «а как мне
// подтвердить этот выбор? Чтобы все остальное скрылось»).
import type { Variation } from '../types'

interface Props {
  variations: Variation[]
  onApply: (v: Variation) => void
  activeId?: string
  /** Снять выбор и вернуть список вариантов. */
  onReset?: () => void
}

export function VariationPicker({ variations, onApply, activeId, onReset }: Props) {
  if (variations.length === 0) return null

  const active = activeId ? variations.find((v) => v.id === activeId) ?? null : null

  if (active) {
    return (
      <div className="variation-picker variation-picker--applied">
        <div className="variation-picker__row variation-picker__row--plain">
          <span className="variation-picker__title">{active.title}</span>
          <span className="variation-picker__summary">{active.summary}</span>
        </div>
        {onReset && (
          <button type="button" className="sp-btn variation-picker__change" onClick={onReset}>
            Изменить
          </button>
        )}
      </div>
    )
  }

  return (
    <div className="variation-picker">
      {variations.map((v) => (
        <button
          key={v.id}
          type="button"
          className="variation-picker__row"
          onClick={() => onApply(v)}
        >
          <span className="variation-picker__title">{v.title}</span>
          <span className="variation-picker__summary">{v.summary}</span>
        </button>
      ))}
    </div>
  )
}
