// VariationPicker — интерактивный выбор альтернативной конфигурации
// (варианты A/B/C) для неблокирующего нарушения. Пользователь видит
// несколько вариантов, нажимает на любой, чтобы применить его как превью
// (без автоприменения): можно перебирать, пока не найдёт подходящий.
import type { Variation } from '../types'

interface Props {
  variations: Variation[]
  onApply: (v: Variation) => void
  activeId?: string
}

export function VariationPicker({ variations, onApply, activeId }: Props) {
  if (variations.length === 0) return null
  return (
    <div className="variation-picker">
      <div className="variation-picker__hint">Выберите вариант — применится как превью:</div>
      <div className="variation-picker__list">
        {variations.map((v) => (
          <button
            key={v.id}
            type="button"
            className={
              'variation-picker__item' + (activeId === v.id ? ' variation-picker__item--active' : '')
            }
            onClick={() => onApply(v)}
          >
            <span className="variation-picker__title">{v.title}</span>
            <span className="variation-picker__summary">{v.summary}</span>
            {v.description ? (
              <span className="variation-picker__desc">{v.description}</span>
            ) : null}
          </button>
        ))}
      </div>
    </div>
  )
}
