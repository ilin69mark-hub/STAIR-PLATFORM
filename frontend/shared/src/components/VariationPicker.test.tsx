import { describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'
import { VariationPicker } from './VariationPicker'
import type { Variation } from '../types'

const v1: Variation = { id: 'a', title: 'A', summary: 'sum A', description: 'desc A' } as Variation
const v2: Variation = { id: 'b', title: 'B', summary: 'sum B', description: 'desc B' } as Variation
const v3: Variation = { id: 'c', title: 'C', summary: 'sum C' } as Variation

describe('VariationPicker', () => {
  it('null on empty', () => {
    const { container } = render(<VariationPicker variations={[]} onApply={vi.fn()} />)
    expect(container.innerHTML).toBe('')
  })

  // Список строк вместо галереи «главная + миниатюры»: в панели 460px
  // главная карточка сжималась до ~250px, и подписи наезжали и обрезались.
  it('renders a plain list of rows without the gallery hint', () => {
    const { container } = render(<VariationPicker variations={[v1, v2, v3]} onApply={vi.fn()} />)
    expect(screen.queryByText('Выберите вариант — применится как превью:')).not.toBeInTheDocument()
    for (const t of ['A', 'B', 'C']) expect(screen.getByText(t)).toBeInTheDocument()
    // Ровно одна строка на вариант и никаких «миниатюр».
    expect(container.querySelectorAll('.variation-picker__row')).toHaveLength(3)
    expect(container.querySelector('.variation-gallery')).toBeNull()
  })

  it('applies on click', () => {
    const fn = vi.fn()
    render(<VariationPicker variations={[v1, v2]} onApply={fn} />)
    fireEvent.click(screen.getByText('B'))
    expect(fn).toHaveBeenCalledWith(v2)
  })

  // Подтверждение выбора: применённый вариант остаётся ОДНОЙ строкой, а
  // остальные скрыты (владелец: «а как мне подтвердить этот выбор? Чтобы
  // все остальное скрылось»).
  it('после применения остаётся одна строка, остальные скрыты', () => {
    const { container } = render(
      <VariationPicker variations={[v1, v2]} onApply={vi.fn()} activeId="a" onReset={vi.fn()} />,
    )
    expect(container.querySelectorAll('.variation-picker__row')).toHaveLength(1)
    expect(screen.getByText('A')).toBeInTheDocument()
    expect(screen.queryByText('B')).not.toBeInTheDocument()
    // Метки «Выбран» больше нет — состояние и так видно по одной строке.
    expect(screen.queryByText('Выбран')).not.toBeInTheDocument()
  })

  it('«Изменить» возвращает список вариантов', () => {
    const onReset = vi.fn()
    render(
      <VariationPicker variations={[v1, v2]} onApply={vi.fn()} activeId="a" onReset={onReset} />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Изменить' }))
    expect(onReset).toHaveBeenCalled()
  })

  it('неизвестный activeId — остаётся списком', () => {
    const { container } = render(<VariationPicker variations={[v1, v2]} onApply={vi.fn()} activeId="x" />)
    expect(container.querySelectorAll('.variation-picker__row')).toHaveLength(2)
  })
})
