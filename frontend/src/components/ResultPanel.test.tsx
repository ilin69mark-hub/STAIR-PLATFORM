import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ResultPanel } from './ResultPanel'
import { makeSnapshot } from '../test/fixtures'

describe('ResultPanel', () => {
  it('рендерит все панели для валидного снапшота', () => {
    render(<ResultPanel snapshot={makeSnapshot()} />)

    expect(screen.getByText('Конвейер выполнен полностью.')).toBeInTheDocument()
    expect(screen.getByText('Нарушений не обнаружено.')).toBeInTheDocument()
    expect(screen.getByText('Марш (Solver)')).toBeInTheDocument()
    expect(screen.getByText('Геометрия')).toBeInTheDocument()
    expect(screen.getByText('Производство')).toBeInTheDocument()
    expect(screen.getByText('Стоимость (RUB)')).toBeInTheDocument()
    expect(screen.getByText('Итоговая цена')).toBeInTheDocument()
    expect(screen.getByText('15')).toBeInTheDocument()
  })

  it('без issues показывает «Нарушений не обнаружено»', () => {
    render(<ResultPanel snapshot={makeSnapshot()} />)
    expect(screen.getByText('Нарушений не обнаружено.')).toBeInTheDocument()
  })

  it('рендерит таблицу нарушений из issues', () => {
    const snapshot = makeSnapshot({
      validation: {
        Issues: [
          {
            Code: 'STEP-E-001',
            Severity: 'error',
            Element: 'march.step',
            Message: 'Высота ступени вне диапазона',
            Min: 150,
            Max: 200,
            Fix: 'Уменьшите шаг',
          },
        ],
        Valid: false,
        Blocking: true,
      },
    })
    render(<ResultPanel snapshot={snapshot} />)
    expect(screen.getByText('STEP-E-001')).toBeInTheDocument()
    expect(screen.getByText('Высота ступени вне диапазона')).toBeInTheDocument()
    expect(screen.getByText('Уменьшите шаг')).toBeInTheDocument()
  })

  it('для blocking снапшота останавливает конвейер и не рендерит панели результата', () => {
    const snapshot = makeSnapshot({
      validation: {
        Issues: [{ Code: 'X-1', Severity: 'error', Element: 'x', Message: 'блокер' }],
        Valid: false,
        Blocking: true,
      },
      flight: undefined,
      measurement: undefined,
      manufacturing: undefined,
      pricing: undefined,
    })
    render(<ResultPanel snapshot={snapshot} />)

    expect(screen.getByText('Конвейер остановлен: обнаружены блокирующие нарушения.')).toBeInTheDocument()
    expect(screen.queryByText('Марш (Solver)')).not.toBeInTheDocument()
    expect(screen.queryByText('Производство')).not.toBeInTheDocument()
    expect(screen.queryByText('Стоимость (RUB)')).not.toBeInTheDocument()
    expect(screen.getByText('блокер')).toBeInTheDocument()
  })
})