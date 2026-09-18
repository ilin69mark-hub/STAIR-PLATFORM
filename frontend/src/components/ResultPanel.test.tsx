import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ResultPanel } from './ResultPanel'
import {
  lshapeFixture,
  makeSnapshot,
  makeUShapeSnapshot,
  spiralFixture,
  ushapeFixture,
  zeroFlightFixture,
} from '../test/fixtures'

// Захват пропсов GeometryViewer: сам вьювер тестируется в shared послойно
// (WebGL в jsdom не строится). Здесь проверяем, что ResultPanel пробрасывает
// ему ровно то, что получила: heightMM, габариты помещения, тип/направление.
const viewerProps = vi.hoisted(() => ({ current: {} as Record<string, unknown> }))
vi.mock('@shared/viewer/GeometryViewer', () => ({
  GeometryViewer: (p: Record<string, unknown>) => {
    viewerProps.current = p
    return null
  },
}))

const mesh3d = {
  Vertices: [
    { X: 0, Y: 0, Z: 0 },
    { X: 900, Y: 0, Z: 0 },
    { X: 0, Y: 0, Z: 2700 },
  ],
  Triangles: [[0, 1, 2]] as Array<[number, number, number]>,
}

describe('ResultPanel', () => {
  it('рендерит все панели для валидного снапшота', () => {
    render(<ResultPanel snapshot={makeSnapshot()} />)

    // Сводка и статус конвейера виден всегда.
    expect(screen.getByText('Конвейер выполнен полностью.')).toBeInTheDocument()
    expect(screen.getByText('Нарушений не обнаружено.')).toBeInTheDocument()

    // По умолчанию активна вкладка «Марш» (Solver + чертёж).
    expect(screen.getByText('Марш (Solver)')).toBeInTheDocument()

    // Панели Геометрия / Производство / Стоимость переключаются вкладками.
    fireEvent.click(screen.getByRole('tab', { name: 'Геометрия' }))
    expect(screen.getByText('Площадь поверхности')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Производство' }))
    expect(screen.getByText('Экспорт BOM (CSV)')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Стоимость' }))
    expect(screen.getByText('Итоговая цена')).toBeInTheDocument()
  })

  it('рендерит панель L-образного марша, когда есть lshape', () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({ flight: zeroFlightFixture, lshape: lshapeFixture })}
      />,
    )

    expect(screen.getByText('L-образный марш (Solver)')).toBeInTheDocument()
    expect(screen.getByText('6 / 9')).toBeInTheDocument()
    expect(screen.getByText('1 080 мм')).toBeInTheDocument()
    expect(screen.getByText('1 000 мм')).toBeInTheDocument()
    expect(screen.queryByText('Марш (Solver)')).not.toBeInTheDocument()
  })

  it('план отрисовывается для не-прямых типов даже с нулевым flight (REGB-01)', () => {
    render(
      <ResultPanel snapshot={makeSnapshot({ flight: zeroFlightFixture, lshape: lshapeFixture })} />,
    )
    fireEvent.click(screen.getByRole('tab', { name: 'План' }))
    expect(screen.getByRole('img', { name: 'Вид сверху (план) лестницы' })).toBeInTheDocument()
    expect(screen.getByText(/L₁/)).toBeInTheDocument()
  })

  it('L-образный марш: чертёжная секция без «Профиля», план по умолчанию', () => {
    render(
      <ResultPanel snapshot={makeSnapshot({ flight: zeroFlightFixture, lshape: lshapeFixture })} />,
    )
    expect(screen.queryByRole('tab', { name: 'Профиль' })).not.toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Вид сверху (план) лестницы' })).toBeInTheDocument()
  })

  it('П-образный марш: чертёжная секция без «Профиля», план по умолчанию', () => {
    render(
      <ResultPanel snapshot={makeSnapshot({ flight: zeroFlightFixture, ushape: ushapeFixture })} />,
    )
    expect(screen.queryByRole('tab', { name: 'Профиль' })).not.toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Вид сверху (план) лестницы' })).toBeInTheDocument()
  })

  it('рендерит панель П-образного марша, когда есть ushape', () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({
          flight: zeroFlightFixture,
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
          flight: zeroFlightFixture,
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
    expect(screen.getByText('Ошибка')).toBeInTheDocument()
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
          flight: zeroFlightFixture,
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

describe('ResultPanel: проброс параметров в 3D-вьювер', () => {
  it('straight: 3D получает heightMM, габариты помещения и тип', async () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({ mesh: mesh3d, room_mesh: mesh3d })}
        heightMM={2700}
      />,
    )
    fireEvent.click(screen.getByRole('tab', { name: '3D' }))
    await waitFor(() => {
      expect(viewerProps.current).toMatchObject({
        flight: 'straight',
        heightMM: 2700,
        roomWidth: 3000,
        roomLength: 4200,
      })
    })
    expect(viewerProps.current.roomMesh).toBeTruthy()
  })

  it('передаёт step_thickness из снапшота во вьювер', async () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({ mesh: mesh3d, step_thickness: 55 })}
        heightMM={2700}
      />,
    )
    fireEvent.click(screen.getByRole('tab', { name: '3D' }))
    await waitFor(() => expect(viewerProps.current.flight).toBe('straight'))
    expect(viewerProps.current.stepThickness).toBe(55)
  })

  it('вкладка Геометрия (GeometryPanel) тоже передаёт heightMM и габариты', async () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({ mesh: mesh3d, room_mesh: mesh3d })}
        heightMM={2700}
      />,
    )
    fireEvent.click(screen.getByRole('tab', { name: '3D' }))
    await waitFor(() => expect(viewerProps.current.heightMM).toBe(2700))

    fireEvent.click(screen.getByRole('tab', { name: 'Геометрия' }))
    await waitFor(() => {
      expect(viewerProps.current).toMatchObject({
        heightMM: 2700,
        roomWidth: 3000,
        roomLength: 4200,
      })
    })
  })

  it('u_shape: передаёт flight и направление поворота', async () => {
    render(
      <ResultPanel
        snapshot={makeUShapeSnapshot({ mesh: mesh3d })}
        heightMM={2750}
      />,
    )
    fireEvent.click(screen.getByRole('tab', { name: '3D' }))
    await waitFor(() => {
      expect(viewerProps.current).toMatchObject({
        flight: 'u_shape',
        direction: 'right',
        heightMM: 2750,
      })
    })
  })

  it('l_shape: габариты помещения берутся из lshape-результата', async () => {
    render(
      <ResultPanel
        snapshot={makeSnapshot({
          flight: zeroFlightFixture,
          lshape: lshapeFixture,
          mesh: mesh3d,
        })}
        heightMM={2700}
      />,
    )
    fireEvent.click(screen.getByRole('tab', { name: '3D' }))
    await waitFor(() => {
      expect(viewerProps.current).toMatchObject({
        flight: 'l_shape',
        roomWidth: 3000,
        roomLength: 4200,
      })
    })
  })

  it('без heightMM вьювер получает undefined (виджет берёт верх меша)', async () => {
    render(<ResultPanel snapshot={makeSnapshot({ mesh: mesh3d })} />)
    fireEvent.click(screen.getByRole('tab', { name: '3D' }))
    await waitFor(() => expect(viewerProps.current.flight).toBe('straight'))
    expect(viewerProps.current.heightMM).toBeUndefined()
  })
})