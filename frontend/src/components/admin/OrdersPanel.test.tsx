// OrdersPanel — presentational: заказы store и смена статуса.
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { OrderDTO } from '@shared/types'
import { OrdersPanel } from './OrdersPanel'

function order(over: Partial<OrderDTO> = {}): OrderDTO {
  return {
    id: 'order-1',
    kind: 'order',
    status: 'new',
    contact: { name: 'Иван Клиент', email: 'buyer@example.com' },
    config: { width_mm: 900, height_mm: 2700 },
    price: { final_price_rub: 180000 },
    created_at: '2026-08-17T10:00:00Z',
    updated_at: '2026-08-17T10:00:00Z',
    ...over,
  }
}

describe('OrdersPanel', () => {
  it('показывает empty-state, когда заказов нет', () => {
    render(<OrdersPanel orders={[]} onStatusChange={vi.fn()} />)
    expect(screen.getByText('Заказов пока нет.')).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it('рендерит строку заказа: номер, бейдж, контакт, детали, статус, дата', () => {
    render(<OrdersPanel orders={[order()]} onStatusChange={vi.fn()} />)
    const row = screen.getByText('order-1').closest('tr') as HTMLTableRowElement
    expect(row).toHaveTextContent('order-1')
    expect(row).toHaveTextContent('Заказ')
    expect(row).toHaveTextContent('Иван Клиент')
    expect(row).toHaveTextContent('buyer@example.com')
    expect(row).toHaveTextContent('900 × 2700 мм')
    expect(row).toHaveTextContent(/180\s*000/)
    expect(screen.getByLabelText('Статус заказа order-1')).toHaveValue('new')
  })

  it('рендерит консультацию: бейдж, вопрос и селект статуса', () => {
    render(
      <OrdersPanel
        orders={[order({ id: 'order-2', kind: 'consultation', config: { question: 'Минимальная ширина лестницы?' } })]}
        onStatusChange={vi.fn()}
      />,
    )
    const row = screen.getByText('order-2').closest('tr') as HTMLTableRowElement
    expect(row).toHaveTextContent('Консультация')
    expect(row).toHaveTextContent('Минимальная ширина лестницы?')
    expect(screen.getByLabelText('Статус заказа order-2')).toBeInTheDocument()
  })

  it('без размеров и цены показывает «—» в деталях', () => {
    render(<OrdersPanel orders={[order({ config: {}, price: undefined })]} onStatusChange={vi.fn()} />)
    const row = screen.getByText('order-1').closest('tr') as HTMLTableRowElement
    const cells = Array.from(row.querySelectorAll('td')) as HTMLTableCellElement[]
    const details = cells[cells.length - 3]
    expect(details.textContent).toBe('—')
  })

  it('только цена без размеров показывает цену', () => {
    render(<OrdersPanel orders={[order({ config: null })]} onStatusChange={vi.fn()} />)
    const row = screen.getByText('order-1').closest('tr') as HTMLTableRowElement
    expect(row).toHaveTextContent(/180\s*000/)
    expect(row).not.toHaveTextContent(/мм/)
  })

  it('только размеры без цены показывают размер', () => {
    render(<OrdersPanel orders={[order({ price: undefined })]} onStatusChange={vi.fn()} />)
    const row = screen.getByText('order-1').closest('tr') as HTMLTableRowElement
    expect(row).toHaveTextContent('900 × 2700 мм')
    expect(row).not.toHaveTextContent(/180\s*000/)
  })

  it('консультация без текста вопроса показывает «—»', () => {
    render(<OrdersPanel orders={[order({ kind: 'consultation', config: null })]} onStatusChange={vi.fn()} />)
    const row = screen.getByText('order-1').closest('tr') as HTMLTableRowElement
    const cells = Array.from(row.querySelectorAll('td')) as HTMLTableCellElement[]
    const details = cells[cells.length - 3]
    expect(details.textContent).toBe('—')
  })

  it('смена статуса вызывает onStatusChange(id, status)', () => {
    const onStatusChange = vi.fn()
    render(<OrdersPanel orders={[order()]} onStatusChange={onStatusChange} />)
    fireEvent.change(screen.getByLabelText('Статус заказа order-1'), { target: { value: 'completed' } })
    expect(onStatusChange).toHaveBeenCalledWith('order-1', 'completed')
  })
})