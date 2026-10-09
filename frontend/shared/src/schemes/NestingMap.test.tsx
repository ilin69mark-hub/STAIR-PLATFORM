import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { NestingMap } from './NestingMap'
import type { Nesting, PlacedPart } from '../types'

function part(p: Partial<PlacedPart> & { PartNumber: string }): PlacedPart {
  return { Length: 2400, Width: 300, X: 0, Y: 0, ...p }
}

function nesting(placed: PlacedPart[]): Nesting {
  return {
    Sheets: [{ Material: 'STEEL-S235', Thickness: 8, Length: 6000, Width: 1500, Placed: placed }],
    PartCount: placed.length,
    PartArea: placed.length * 2400 * 300,
    SheetArea: 9000000,
    WasteArea: 9000000 - placed.length * 2400 * 300,
    Utilization: (placed.length * 2400 * 300) / 9000000,
  }
}

describe('NestingMap', () => {
  it('возвращает null без листов', () => {
    const { container } = render(<NestingMap nesting={{ ...nesting([]), Sheets: [] }} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('показывает габариты листа и площади', () => {
    render(<NestingMap nesting={nesting([part({ PartNumber: 'STR-01' })])} />)
    expect(screen.getByText(/Лист 1/)).toBeInTheDocument()
    expect(screen.getByText(/6000×1500 мм/)).toBeInTheDocument()
    expect(screen.getByText(/Деталей: 1/)).toBeInTheDocument()
  })

  it('рисует паз и шип сварного стыка «шип-паз»', () => {
    const { container } = render(
      <NestingMap
        nesting={nesting([
          part({ PartNumber: 'STR-01', HasMortise: true }),
          part({ PartNumber: 'STR-02', X: 2500, HasMortise: true, HasTongue: true }),
        ])}
      />,
    )
    expect(container.querySelectorAll('.nesting-sheet__mortise')).toHaveLength(2)
    // Шип есть только у второго куска: первый стыкуется пазом.
    expect(container.querySelectorAll('.nesting-sheet__tongue')).toHaveLength(1)
    expect(screen.getByText(/Сварные стыки «шип-паз»/)).toBeInTheDocument()
  })

  it('рисует стык колонны поперёк детали', () => {
    const { container } = render(
      <NestingMap
        nesting={nesting([part({ PartNumber: 'CLM-01', HasMortise: true, JointOnWidth: true })])}
      />,
    )
    const mortise = container.querySelector('.nesting-sheet__mortise')
    // Лист 6000×1500 вписан в бокс 300×200 → масштаб по ширине определяет
    // высоту детали; полоса паза лежит на НИЖНЕЙ кромке, а не на правой.
    expect(mortise).toHaveAttribute('x', '0')
    expect(Number(mortise?.getAttribute('y'))).toBeGreaterThan(0)
  })

  it('не рисует знаки стыка у цельных деталей', () => {
    const { container } = render(
      <NestingMap nesting={nesting([part({ PartNumber: 'TRD-01' })])} />,
    )
    expect(container.querySelector('.nesting-sheet__mortise')).toBeNull()
    expect(container.querySelector('.nesting-sheet__tongue')).toBeNull()
    expect(screen.queryByText(/Сварные стыки/)).toBeNull()
  })
})
