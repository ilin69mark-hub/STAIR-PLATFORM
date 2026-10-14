import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { ResultPanel } from './ResultPanel'
import { makeSnapshot } from '../test/fixtures'

// Фрезеровка фасок попадает в цену как ТРУД, а в разбивке цены остаётся одна
// сводная строка. Без сводки по техмаршруту администратор не видит, что именно
// увеличило цену, и объяснить её покупателю не может.
describe('ResultPanel: техмаршрут в админке', () => {
  const snapshotWithMilling = () =>
    makeSnapshot({
      cost: {
        PartCount: 2,
        OperationCount: 5,
        EstimatedMachineTime: 12,
        EstimatedLaborTime: 61.6,
        EstimatedProductionTime: 73.6,
        OperationPlan: {
          Parts: [
            {
              PartNumber: 'P1',
              Operations: [
                { ID: 1, PartNumber: 'P1', Type: 'cutting', Sequence: 1, Machine: 'laser-cutter', EstimatedTime: 2, OperatorRequired: true },
                { ID: 2, PartNumber: 'P1', Type: 'milling', Sequence: 2, Machine: 'manual-workstation', EstimatedTime: 3.6, OperatorRequired: true },
                { ID: 3, PartNumber: 'P1', Type: 'finishing', Sequence: 3, Machine: 'manual-workstation', EstimatedTime: 3, OperatorRequired: true },
              ],
            },
            {
              PartNumber: 'P2',
              Operations: [
                { ID: 4, PartNumber: 'P2', Type: 'cutting', Sequence: 1, Machine: 'laser-cutter', EstimatedTime: 2, OperatorRequired: true },
                { ID: 5, PartNumber: 'P2', Type: 'finishing', Sequence: 2, Machine: 'manual-workstation', EstimatedTime: 3, OperatorRequired: true },
              ],
            },
          ],
        },
      },
    })

  it('показывает сводку операций, включая фрезеровку фасок', () => {
    render(<ResultPanel snapshot={snapshotWithMilling()} />)
    fireEvent.click(screen.getByRole('tab', { name: /Производство/i }))

    expect(screen.getByText('Технологический маршрут')).toBeInTheDocument()
    // Фрезеровка названа по-человечески, а не сырым «milling».
    expect(screen.getByText('Фрезеровка кромки (фаска)')).toBeInTheDocument()
    expect(screen.getByText('Лазерный рез')).toBeInTheDocument()
    // Руки мастера, а не станок: от этого зависит тариф.
    expect(screen.getAllByText('ручное рабочее место').length).toBeGreaterThan(0)
  })

  it('суммирует время по типу операции', () => {
    render(<ResultPanel snapshot={snapshotWithMilling()} />)
    fireEvent.click(screen.getByRole('tab', { name: /Производство/i }))
    // Рез: 2 + 2 = 4 минуты.
    const cutRow = screen.getByText('Лазерный рез').closest('tr') as HTMLElement
    expect(cutRow.textContent).toContain('4,0')
    // Фрезеровка: одна деталь, 3,6 минуты.
    const millRow = screen.getByText('Фрезеровка кромки (фаска)').closest('tr') as HTMLElement
    expect(millRow.textContent).toContain('3,6')
  })

  it('без плана операций раздел не рисуется', () => {
    render(<ResultPanel snapshot={makeSnapshot({ cost: { PartCount: 1 } })} />)
    fireEvent.click(screen.getByRole('tab', { name: /Производство/i }))
    expect(screen.queryByText('Технологический маршрут')).not.toBeInTheDocument()
  })
})
