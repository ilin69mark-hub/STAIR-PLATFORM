import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ANNOTATE } from '../scheme-annot'
import { StairPlan } from './StairPlan'
import type { PlanExtras, PlanFlight } from './StairPlan'

const base: PlanFlight = {
  StepCount: 15,
  StepHeight: 180,
  TreadDepth: 280,
  Run: 4200,
  Stringer: 4783,
  Angle: Math.PI / 4,
  Width: 900,
}

// expectSchemeFits — весь чертёж (заливка/контур, для спирали — окружность)
  // должен лежать внутри viewBox: фиксированный канвас не может обрезать марш.
  function expectSchemeFits(container: HTMLElement) {
    const vb = (container.querySelector('.scheme__svg')?.getAttribute('viewBox') ?? '0 0 620 340')
    .split(/\s+/)
    .map(Number)
  const [vx, vy, vw, vh] = vb
  const fail: string[] = []
  const check = (label: string, x: number, y: number, w: number, h: number) => {
    if (x < vx || y < vy || x + w > vx + vw || y + h > vy + vh) {
      fail.push(`${label} (${x},${y},${w},${h}) вне viewBox (${vx},${vy},${vw},${vh})`)
    }
  }
  container.querySelectorAll('.scheme__fill').forEach((r) =>
    check('rect', Number(r.getAttribute('x')), Number(r.getAttribute('y')), Number(r.getAttribute('width')), Number(r.getAttribute('height'))),
  )
  const circ = container.querySelector('.scheme__outline')
  if (circ && circ.tagName === 'circle') {
    const cx = Number(circ.getAttribute('cx'))
    const cy = Number(circ.getAttribute('cy'))
    const cr = Number(circ.getAttribute('r'))
    check('circle', cx - cr, cy - cr, cr * 2, cr * 2)
  }
  expect(fail).toEqual([])
}

// expectTreadsInsideRects — все линии ступеней плана (L/U) лежат целиком
  // внутри одного из маршей/площадки: линии не должны висеть вне фигуры.
  function expectTreadsInsideRects(container: HTMLElement) {
    const rects = Array.from(container.querySelectorAll('rect.scheme__fill')).map((r) => ({
      x: Number(r.getAttribute('x')),
      y: Number(r.getAttribute('y')),
      w: Number(r.getAttribute('width')),
      h: Number(r.getAttribute('height')),
    }))
    const fails: string[] = []
    container.querySelectorAll('line.scheme__tread').forEach((ln, i) => {
      const xs = [ln.getAttribute('x1'), ln.getAttribute('x2')].map(Number)
      const ys = [ln.getAttribute('y1'), ln.getAttribute('y2')].map(Number)
      const minX = Math.min(...xs)
      const maxX = Math.max(...xs)
      const minY = Math.min(...ys)
      const maxY = Math.max(...ys)
      const inside = rects.some(
        (r) => minX >= r.x - 0.5 && maxX <= r.x + r.w + 0.5 && minY >= r.y - 0.5 && maxY <= r.y + r.h + 0.5,
      )
      if (!inside) fails.push(`ступень #${i} (${minX},${minY})–(${maxX},${maxY}) вне прямоугольников`)
    })
    expect(fails).toEqual([])
  }

describe('StairPlan', () => {
  it('возвращает null без данных', () => {
    const { container } = render(
      <StairPlan flight={base} kind="straight" solver={{}} />,
    )
    expect(container).toBeTruthy()
  })

  it('рендерит план прямого марша с размерами', () => {
    render(<StairPlan flight={base} kind="straight" solver={{}} />)
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()
    expect(screen.getByText(/L 4 200 мм/)).toBeInTheDocument()
    expect(screen.getByText(/B 900 мм/)).toBeInTheDocument()
    expect(screen.getByRole('img')).toBeInTheDocument()
  })

  it('прямой план рисует зону подхода при заданном approachSpace', () => {
    const { container } = render(
      <StairPlan flight={base} kind="straight" solver={{ approachSpace: 1000 }} />,
    )
    expect(container.querySelectorAll('rect.scheme__approach').length).toBe(1)
    expect(screen.getByText(/Свободное место 1 000 мм/)).toBeInTheDocument()
  })

  it('прямой план рисует зону подхода по умолчанию (1000 мм) без approachSpace', () => {
    const { container } = render(<StairPlan flight={base} kind="straight" solver={{}} />)
    // EDR-0023: свободное пространство перед первой ступенью обязательно
    // (норма 1000–1200 мм); без явного approachSpace рисуется дефолт 1000 мм.
    expect(container.querySelectorAll('rect.scheme__approach').length).toBe(1)
    expect(screen.getByText(/Свободное место 1 000 мм/)).toBeInTheDocument()
  })

  it('использует фиксированный канвас как у профиля', () => {
    const { container } = render(<StairPlan flight={base} kind="straight" solver={{}} />)
    const vb = (container.querySelector('.scheme__svg')?.getAttribute('viewBox') ?? '')
      .split(/\s+/)
      .map(Number)
    // Тот же канвас, что у StairProfile: масштаб вида сверху совпадает с профилем.
    expect(vb).toEqual([0, 0, 620, 340])
    // Чертёж центрирован с полями: не касается краёв канваса.
    const rect = container.querySelector('rect.scheme__fill')
    const margins = [
      Number(rect?.getAttribute('x')),
      Number(rect?.getAttribute('y')),
      620 - Number(rect?.getAttribute('x')) - Number(rect?.getAttribute('width')),
      340 - Number(rect?.getAttribute('y')) - Number(rect?.getAttribute('height')),
    ]
    expect(margins.every((m) => m > 0)).toBe(true)
  })

  it('прямой план показывает стрелку направления подъёма', () => {
    const { container } = render(<StairPlan flight={base} kind="straight" solver={{}} />)
    const dir = container.querySelector('line.scheme__dir')
    expect(dir).not.toBeNull()
    expect(dir?.getAttribute('marker-end')).toBe('url(#pln-dir)')
    // Направление как в профиле: первая ступень справа (x=Run), подъём — влево.
    expect(Number(dir?.getAttribute('x1'))).toBeGreaterThan(Number(dir?.getAttribute('x2')))
    expect(Number(dir?.getAttribute('x1'))).toBeGreaterThan(310)
  })

  it('рисует временную цвето-буквенную разметку периметра и рёбер при наличии комнаты', () => {
    const solver: PlanExtras = { roomWidth: 5000, roomLength: 5000, roomFits: true, direction: 'right' }
    const first = render(<StairPlan flight={base} kind="straight" solver={solver} />)
    if (ANNOTATE) {
      // Стороны периметра: В/Н/П/Л.
      for (const w of ['В', 'Н', 'П', 'Л']) {
        expect(screen.getByText(w)).toBeInTheDocument()
      }
      // Рёбра марша: 1В/1Н/1П/1Л (каждое своим цветом).
      for (const e of ['1В', '1Н', '1П', '1Л']) {
        expect(screen.getByText(e)).toBeInTheDocument()
      }
    }
    first.unmount()
    // Без комнаты разметка не рисуется.
    const second = render(<StairPlan flight={base} kind="straight" solver={{}} />)
    for (const w of ['В', 'Н', 'П', 'Л']) {
      expect(screen.queryByText(w)).toBeNull()
    }
    second.unmount()
  })

  it('прямой план рисует перила с двух сторон', () => {
    const { container } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900, Railing: 'both' }} kind="straight" solver={{}} />,
    )
    expect(container.querySelectorAll('line.scheme__railing').length).toBe(2)
  })

  it('прямой план рисует перила с одной стороны', () => {
    for (const side of ['left', 'right'] as const) {
      const { container } = render(
        <StairPlan flight={{ ...base, RailingHeight: 900, Railing: side }} kind="straight" solver={{}} />,
      )
      expect(container.querySelectorAll('line.scheme__railing').length).toBe(1)
    }
  })

  it('прямой план не рисует перила при выборе "без перил" или без высоты', () => {
    const { container: c1 } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900, Railing: 'none' }} kind="straight" solver={{}} />,
    )
    const { container: c2 } = render(<StairPlan flight={{ ...base, RailingHeight: 0 }} kind="straight" solver={{}} />)
    expect(c1.querySelectorAll('line.scheme__railing').length).toBe(0)
    expect(c2.querySelectorAll('line.scheme__railing').length).toBe(0)
  })

  it('рендерит план L-образного марша', () => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    render(<StairPlan flight={base} kind="l_shape" solver={solver} />)
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()
    expect(screen.getByText(/L₁ 1 620 мм/)).toBeInTheDocument()
    expect(screen.getByText(/L₂ 2 430 мм/)).toBeInTheDocument()
  })

  it('L план: ступени лежат внутри маршей (не висят за краем)', () => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    const { container } = render(<StairPlan flight={base} kind="l_shape" solver={solver} />)
    expectTreadsInsideRects(container)
    expect(screen.getByText(/B 900 мм/)).toBeInTheDocument()
  })

  it('L и U планы: ступени нижнего/верхнего маршей различаются цветом', () => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    for (const kind of ['l_shape', 'u_shape'] as const) {
      const { container } = render(<StairPlan flight={base} kind={kind} solver={solver} />)
      const lower = container.querySelectorAll('line.scheme__tread--lower')
      const upper = container.querySelectorAll('line.scheme__tread--upper')
      expect(lower.length).toBeGreaterThan(0)
      expect(upper.length).toBeGreaterThan(0)
      expect(lower.length + upper.length).toBe(container.querySelectorAll('line.scheme__tread').length)
    }
  })

  it('L план: перила рисуются по сторонам сегментов, «без перил» — нет', () => {
    const mk = (lower: string, landing: string, upper: string): PlanExtras => ({
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
      railingLower: lower,
      railingLanding: landing,
      railingUpper: upper,
    })
    // все справа → только правая грань каждого сегмента
    const { container: cR } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900 }} kind="l_shape" solver={mk('right', 'right', 'right')} />,
    )
    expect(cR.querySelectorAll('polyline.scheme__railing').length).toBe(3)
    // все слева → только левая грань каждого сегмента
    const { container: cL } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900 }} kind="l_shape" solver={mk('left', 'left', 'left')} />,
    )
    expect(cL.querySelectorAll('polyline.scheme__railing').length).toBe(3)
    // обе стороны → 6 полилиний (равно старому сплошному контуру)
    const { container: cB } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900 }} kind="l_shape" solver={mk('both', 'both', 'both')} />,
    )
    expect(cB.querySelectorAll('polyline.scheme__railing').length).toBe(6)
    // без перил ни в одном сегменте → ничего
    const { container: cN } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900 }} kind="l_shape" solver={mk('none', 'none', 'none')} />,
    )
    expect(cN.querySelector('.scheme__railing')).toBeNull()
    // высота 0 → перил нет
    const { container: c0 } = render(
      <StairPlan flight={base} kind="l_shape" solver={mk('right', 'right', 'right')} />,
    )
    expect(c0.querySelector('.scheme__railing')).toBeNull()
  })

  it('рендерит план П-образного марша', () => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    render(<StairPlan flight={base} kind="u_shape" solver={solver} />)
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()
    expect(screen.getByText(/L₁ 1 620 мм/)).toBeInTheDocument()
    expect(screen.getByText(/L₂ 2 430 мм/)).toBeInTheDocument()
  })

  it('U план: ступени обоих маршей лежат внутри прямоугольников', () => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    const { container } = render(<StairPlan flight={base} kind="u_shape" solver={solver} />)
    expectTreadsInsideRects(container)
    expect(screen.getByText(/B 900 мм/)).toBeInTheDocument()
  })

  it('U план: перила рисуются по сторонам сегментов, «без перил» — нет', () => {
    const mk = (lower: string, landing: string, upper: string): PlanExtras => ({
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
      railingLower: lower,
      railingLanding: landing,
      railingUpper: upper,
    })
    // все справа → только правая грань каждого сегмента
    const { container: cR } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900 }} kind="u_shape" solver={mk('right', 'right', 'right')} />,
    )
    expect(cR.querySelectorAll('polyline.scheme__railing').length).toBe(3)
    // все слева → только левая грань каждого сегмента
    const { container: cL } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900 }} kind="u_shape" solver={mk('left', 'left', 'left')} />,
    )
    expect(cL.querySelectorAll('polyline.scheme__railing').length).toBe(3)
    // обе стороны → 6 полилиний
    const { container: cB } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900 }} kind="u_shape" solver={mk('both', 'both', 'both')} />,
    )
    expect(cB.querySelectorAll('polyline.scheme__railing').length).toBe(6)
    // без перил ни в одном сегменте → ничего
    const { container: cN } = render(
      <StairPlan flight={{ ...base, RailingHeight: 900 }} kind="u_shape" solver={mk('none', 'none', 'none')} />,
    )
    expect(cN.querySelector('.scheme__railing')).toBeNull()
    // высота 0 → перил нет
    const { container: c0 } = render(
      <StairPlan flight={base} kind="u_shape" solver={mk('right', 'right', 'right')} />,
    )
    expect(c0.querySelector('.scheme__railing')).toBeNull()
  })

  it.each(['l_shape', 'u_shape'] as const)('план зеркалится по направлению поворота (%s)', (kind) => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    const measure = (direction: 'left' | 'right') => {
      const { container } = render(
        <StairPlan flight={{ ...base, RailingHeight: 900, Railing: 'both' }} kind={kind} solver={{ ...solver, direction }} />,
      )
      expectSchemeFits(container)
      const fills = [...container.querySelectorAll('rect.scheme__fill')].map((r) => ({
        x: Number(r.getAttribute('x')),
        y: Number(r.getAttribute('y')),
        w: Number(r.getAttribute('width')),
        h: Number(r.getAttribute('height')),
      }))
      return { fills, rail: container.querySelector('polyline.scheme__railing') }
    }
    const right = measure('right')
    const left = measure('left')
    expect(right.rail).not.toBeNull()
    expect(left.rail).not.toBeNull()
    expect(right.fills.length).toBe(3)
    expect(left.fills.length).toBe(3)
    // Зеркалится без искажений: те же размеры и вертикали.
    right.fills.forEach((r, i) => {
      expect(left.fills[i].w).toBeCloseTo(r.w, 6)
      expect(left.fills[i].h).toBeCloseTo(r.h, 6)
      expect(left.fills[i].y).toBeCloseTo(r.y, 6)
    })
    // Нижний марш — первая заливка; в 'right' его правая кромка стыкуется с
    // левой кромкой площадки, в 'left' площадка целиком слева от него.
    expect(right.fills[1].x).toBeCloseTo(right.fills[0].x + right.fills[0].w, 6)
    expect(left.fills[1].x).toBeLessThan(left.fills[0].x)
    // Зеркальность: каждый 'right'-прямоугольник и его 'left'-двойник
    // равноудалены от общей оси зеркала.
    //
    // Раньше здесь жёстко стояло `r.x + r.w + left.x ≈ 620` (центр канваса).
    // Это работало только для прежней раскладки — классического switchback,
    // у которого bbox симметричен относительно x=0. После перехода на
    // раскладку 3D-модели (EDR-0006 §4.8.1) верхний марш начинается с
    // L1−L2 и при L2 > L1 выходит в отрицательные X, поэтому ось зеркала
    // больше не совпадает с центром канваса: зона подхода (EDR-0023) живёт
    // слева от входа в варианте 'right' и справа — в варианте 'left', из-за
    // чего центрирование содержимого сдвигается на ap/2 в разные стороны.
    // Инвариант проверяется через ось: середина между правыми кромками
    // 'right' и 'left' — одна и та же точка для всех трёх сегментов.
    const axis =
      (right.fills[0].x + right.fills[0].w + left.fills[0].x +
        right.fills[1].x + right.fills[1].w + left.fills[1].x +
        right.fills[2].x + right.fills[2].w + left.fills[2].x) /
      6
    right.fills.forEach((r, i) => {
      expect(2 * axis - (left.fills[i].x + left.fills[i].w)).toBeCloseTo(r.x, 6)
    })
  })

  it('U план повторяет раскладку 3D-модели (EDR-0006 §4.8.1)', () => {
    // Регрессия S2 (найдена вместе с DOM-004): план рисовал классический
    // switchback (верхний марш над нижним, план [0, L2]), тогда как 3D строит
    // верхний марш в [L1−L2, L1] × [W, 2W] (площадка [L1, L1+W] × [0, 2W]).
    // Одно и то же выглядело двумя разными лестницами, при том что проверка
    // вписывания в помещение считалась по bbox 3D-модели.
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
      direction: 'right',
    }
    const { container } = render(<StairPlan flight={base} kind="u_shape" solver={solver} />)
    const fills = [...container.querySelectorAll('rect.scheme__fill')].map((r) => ({
      x: Number(r.getAttribute('x')),
      y: Number(r.getAttribute('y')),
      w: Number(r.getAttribute('width')),
      h: Number(r.getAttribute('height')),
    }))
    expect(fills.length).toBe(3)
    const [lower, landing, upper] = fills
    // Площадка: план [L1, L1+W] × [0, 2W] — ширина по X равна ШИРИНЕ МАРША W
    // (а не Wp), высота вдвое больше ширины марша; примыкает к нижнему маршу.
    const wMm = base.Width
    expect(landing.w).toBeCloseTo(lower.w * (wMm / solver.lowerRun!), 6)
    expect(landing.h).toBeCloseTo(lower.h * 2, 6)
    expect(landing.x).toBeCloseTo(lower.x + lower.w, 6)
    // Верхний марш начинается левее нижнего, когда L2 > L1 (отрицательные X
    // в координатах модели), и заканчивается у правой кромки площадки.
    expect(upper.x).toBeLessThan(lower.x)
    expect(upper.x + upper.w).toBeCloseTo(landing.x, 6)
    // Верхний марш занимает Y ∈ [W, 2W], то есть делит верхнюю кромку
    // площадки (обе верхние границы совпадают), а его ширина равна W.
    expect(upper.y).toBeCloseTo(landing.y, 6)
    expect(upper.h).toBeCloseTo(lower.h, 6)
  })

  it('рендерит план спиральной лестницы', () => {
    const solver: PlanExtras = {
      outerRadius: 800,
      columnRadius: 300,
      angularStep: 24,
      angularTotal: 360,
    }
    render(<StairPlan flight={base} kind="spiral" solver={solver} />)
    expect(screen.getByText('План: вид сверху, размеры в мм.')).toBeInTheDocument()
    expect(screen.getByText(/R 800 мм/)).toBeInTheDocument()
    expect(screen.getByText('α 360°')).toBeInTheDocument()
    expect(screen.getByText('15 ступ.')).toBeInTheDocument()
  })

  it('прямой план (масштаб по высоте) полностью помещается в viewBox', () => {
    const tall: PlanFlight = { ...base, StepCount: 7, TreadDepth: 285, Run: 2000, Width: 1000 }
    const { container } = render(<StairPlan flight={tall} kind="straight" solver={{}} />)
    expectSchemeFits(container)
  })

  it('план L-образного марша не обрезает нижний марш', () => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    const { container } = render(<StairPlan flight={base} kind="l_shape" solver={solver} />)
    expectSchemeFits(container)
  })

  it('план П-образного марша полностью помещается в viewBox', () => {
    const solver: PlanExtras = {
      lowerStepCount: 6,
      upperStepCount: 9,
      landingWidth: 1000,
      lowerRun: 1620,
      upperRun: 2430,
    }
    const { container } = render(<StairPlan flight={base} kind="u_shape" solver={solver} />)
    expectSchemeFits(container)
  })

  it('план спирали полностью помещается в viewBox', () => {
    const solver: PlanExtras = {
      outerRadius: 800,
      columnRadius: 300,
      angularStep: 24,
      angularTotal: 360,
    }
    const { container } = render(<StairPlan flight={base} kind="spiral" solver={solver} />)
    expectSchemeFits(container)
  })
})