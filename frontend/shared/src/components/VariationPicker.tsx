// VariationPicker — интерактивный выбор альтернативной конфигурации
// (варианты A/B/C) для неблокирующего нарушения. Пользователь видит
// варианты как галерею: один выбранный/активный — крупно слева, остальные —
// компактными карточками справа. Клик по любой карточке применяет её как
// превью (без модалки), активная становится крупной и помечается «Выбран».
import type { Variation } from '../types'

interface Props {
  variations: Variation[]
  onApply: (v: Variation) => void
  activeId?: string
}

export function VariationPicker({ variations, onApply, activeId }: Props) {
  if (variations.length === 0) return null
  // Активная — крупная карточка; остальные — уменьшенные справа. Если активной
  // ещё нет (первый показ), показываем первый вариант как основной без метки.
  const activeVar = variations.find((v) => v.id === activeId) ?? null
  const others = activeVar ? variations.filter((v) => v.id !== activeVar.id) : variations.slice(1)
  const main = activeVar ?? variations[0]

  const renderCard = (v: Variation, kind: 'main' | 'thumb') => {
    const isActiveMain = activeVar !== null && v.id === main.id
    return (
      <button
        key={v.id}
        type="button"
        className={
          'variation-picker__item variation-picker__item--' + kind +
          (isActiveMain ? ' variation-picker__item--active' : '')
        }
        onClick={() => onApply(v)}
      >
        <span className="variation-picker__title">{v.title}</span>
        <span className="variation-picker__summary">{v.summary}</span>
        {kind === 'main' && v.description ? (
          <span className="variation-picker__desc">{v.description}</span>
        ) : null}
        {isActiveMain ? (
          <span className="variation-picker__applied">Выбран</span>
        ) : null}
      </button>
    )
  }

  return (
    <div className="variation-picker">
      <div className="variation-picker__hint">Выберите вариант — применится как превью:</div>
      <div className="variation-gallery">
        <div className="variation-gallery__main">{renderCard(main, 'main')}</div>
        {others.length > 0 && (
          <div className="variation-gallery__thumbs">{others.map((v) => renderCard(v, 'thumb'))}</div>
        )}
      </div>
    </div>
  )
}