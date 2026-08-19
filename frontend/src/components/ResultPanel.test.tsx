import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ResultPanel } from './ResultPanel'
import {
  lshapeFixture,
  makeSnapshot,
  spiralFixture,
  ushapeFixture,
} from '../test/fixtures'

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

  it('рендерит панель L-образного марша, когда есть lshape', () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({ flight: undefined, lshape: lshapeFixture })}
      />,
    )

    expect(screen.getByText('L-образный марш (Solver)')).toBeInTheDocument()
    expect(screen.getByText('6 / 9')).toBeInTheDocument()
    expect(screen.getByText('1 080 мм')).toBeInTheDocument()
    expect(screen.getByText('1 000 мм')).toBeInTheDocument()
    expect(screen.queryByText('Марш (Solver)')).not.toBeInTheDocument()
  })

  it('рендерит панель П-образного марша, когда есть ushape', () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({
          flight: undefined,
          lshape: undefined,
          ushape: ushapeFixture,
        })}
      />,
    )

    expect(screen.getByText('П-образный марш (Solver)')).toBeInTheDocument()
    expect(screen.getByText('6 / 9')).toBeInTheDocument()
    expect(screen.getByText('1 080 мм')).toBeInTheDocument()
    expect(screen.queryByText('Марш (Solver)')).not.toBeInTheDocument()
  })

  it('рендерит панель спирального марша, когда есть spiral', () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({
          flight: undefined,
          lshape: undefined,
          ushape: undefined,
          spiral: spiralFixture,
        })}
      />,
    )

    expect(screen.getByText('Спиральный марш (Solver)')).toBeInTheDocument()
    expect(screen.getByText('800 мм')).toBeInTheDocument()
    expect(screen.getByText('Радиус колонны r')).toBeInTheDocument()
    expect(screen.getByText('125,664 мм / 265,29 мм / 335,103 мм')).toBeInTheDocument()
    expect(screen.queryByText('Марш (Solver)')).not.toBeInTheDocument()
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

  it('переключает вкладки чертёжной секции: профиль, план, 3D', () => {
    render(<ResultPanel snapshot={makeSnapshot()} />)

    expect(screen.getByText('Профиль')).toBeInTheDocument()
    expect(screen.getByText('План')).toBeInTheDocument()
    expect(screen.getByText('3D')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'План' }))
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: '3D' }))
    expect(screen.getByText('Модель 3D недоступна для этого снапшота.')).toBeInTheDocument()
  })

  it('рендерит вкладки чертежей для спирального марша', () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({
          flight: undefined,
          lshape: undefined,
          ushape: undefined,
          spiral: spiralFixture,
        })}
      />,
    )

    fireEvent.click(screen.getByRole('tab', { name: 'План' }))
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()
    expect(screen.getByText(/ступ\./)).toBeInTheDocument()
  })
})