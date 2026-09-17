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

  it('renders main and thumbs', () => {
    render(<VariationPicker variations={[v1, v2, v3]} onApply={vi.fn()} />)
    expect(screen.getByText('Выберите вариант — применится как превью:')).toBeInTheDocument()
    expect(screen.getByText('A')).toBeInTheDocument()
    expect(screen.getByText('B')).toBeInTheDocument()
  })

  it('applies on click', () => {
    const fn = vi.fn()
    render(<VariationPicker variations={[v1, v2]} onApply={fn} />)
    fireEvent.click(screen.getByText('B'))
    expect(fn).toHaveBeenCalledWith(v2)
  })

  it('activeId marks Выбран', () => {
    render(<VariationPicker variations={[v1, v2]} onApply={vi.fn()} activeId="a" />)
    expect(screen.getByText('Выбран')).toBeInTheDocument()
  })

  it('no Выбран when no activeId matches', () => {
    render(<VariationPicker variations={[v1, v2]} onApply={vi.fn()} activeId="x" />)
    expect(screen.queryByText('Выбран')).not.toBeInTheDocument()
  })
})
