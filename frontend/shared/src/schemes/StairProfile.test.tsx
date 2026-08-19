import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { StairProfile } from './StairProfile'
import type { FlightResult } from '../types'

const base: FlightResult = {
  StepCount: 15,
  StepHeight: 180,
  TreadDepth: 280,
  Run: 4200,
  Stringer: 4867,
  Angle: 0.588,
}

describe('StairProfile', () => {
  it('возвращает null без данных', () => {
    const { container } = render(<StairProfile flight={{ ...base, StepCount: 0 }} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('рендерит профиль прямого марша со всеми подписями', () => {
    render(<StairProfile flight={base} />)
    expect(screen.getByText('15 ступ.')).toBeInTheDocument()
    expect(screen.getByText('2 700 мм')).toBeInTheDocument()
    expect(screen.getByText('4 200 мм')).toBeInTheDocument()
    expect(screen.getByText('4 867 мм')).toBeInTheDocument()
    expect(screen.getByText('33.7°')).toBeInTheDocument()
    expect(screen.getByRole('img')).toBeInTheDocument()
  })

  it('подписи не перекрывают контур чертежа', () => {
    const { container } = render(<StairProfile flight={base} />)
    // Контур = ограничивающий прямоугольник отрисованных ступеней.
    const xs: number[] = []
    const ys: number[] = []
    container.querySelectorAll('.scheme__step').forEach((poly) => {
      poly.getAttribute('points')?.split(/\s+/).forEach((p) => {
        const [x, y] = p.split(',').map(Number)
        xs.push(x)
        ys.push(y)
      })
    })
    const outline = {
      xMin: Math.min(...xs),
      xMax: Math.max(...xs),
      yMin: Math.min(...ys),
      yMax: Math.max(...ys),
    }
    const texts = Array.from(
      container.querySelectorAll('.scheme__label, .scheme__dimtext'),
    ).filter((t) => !t.classList.contains('scheme__dimtext--stringer'))
    expect(texts.length).toBeGreaterThan(0)
    texts.forEach((t) => {
      const x = Number(t.getAttribute('x'))
      const y = Number(t.getAttribute('y'))
      const inside =
        x > outline.xMin && x < outline.xMax && y > outline.yMin && y < outline.yMax
      expect(inside).toBe(false)
    })
  })

  it('детерминирован: одинаковые входные данные дают одинаковый SVG', () => {
    const a = render(<StairProfile flight={base} />)
    const b = render(<StairProfile flight={base} />)
    expect(a.container.innerHTML).toBe(b.container.innerHTML)
  })

  it('узкий и высокий марш не теряет подписи', () => {
    const tall: FlightResult = {
      ...base,
      StepCount: 20,
      StepHeight: 150,
      TreadDepth: 300,
      Run: 6000,
      Stringer: 6245,
      Angle: 0.46,
    }
    render(<StairProfile flight={tall} />)
    expect(screen.getByText('20 ступ.')).toBeInTheDocument()
    expect(screen.getByText('3 000 мм')).toBeInTheDocument()
    expect(screen.getByText('6 000 мм')).toBeInTheDocument()
  })

  it('закрытый марш: одна ступень-многоугольник на каждую ступень', () => {
    const closed: FlightResult = { ...base, Riser: true }
    const { container } = render(<StairProfile flight={closed} />)
    expect(container.querySelectorAll('.scheme__step').length).toBe(15)
  })

  it('закрытый марш с проступями: подступенки рисуются отдельно, по ТЗ', () => {
    const closed: FlightResult = { ...base, Riser: true, StepThickness: 40 }
    const { container } = render(<StairProfile flight={closed} />)
    expect(container.querySelectorAll('.scheme__step').length).toBe(15)
    expect(container.querySelectorAll('.scheme__riser').length).toBe(15)
    const riser = container.querySelector('.scheme__riser')
    expect(riser).not.toBeNull()
    const rpts = (riser!.getAttribute('points') ?? '')
      .trim()
      .split(/\s+/)
      .map((p) => p.split(',').map(Number))
    expect(rpts).toHaveLength(4)
    const xs = rpts.map((p) => p[0])
    const ys = rpts.map((p) => p[1])
    const w = Math.max(...xs) - Math.min(...xs)
    const h = Math.max(...ys) - Math.min(...ys)
    expect(h).toBeGreaterThan(w)
  })

  it('открытый марш без подступенков не рисует вертикальных граней', () => {
    const open: FlightResult = { ...base, Riser: false, StepThickness: 40 }
    const { container } = render(<StairProfile flight={open} />)
    const steps = Array.from(container.querySelectorAll('.scheme__step'))
    expect(steps.length).toBe(15)
    expect(container.querySelectorAll('.scheme__riser').length).toBe(0)
    // Высота прямоугольника проступи ограничена толщиной ступени.
    const yTop = Number(steps[0].getAttribute('points')?.split(' ')[1].split(',')[1])
    const yBot = Number(steps[0].getAttribute('points')?.split(' ')[0].split(',')[1])
    const h = Math.abs(yTop - yBot)
    expect(h).toBeGreaterThan(0)
    expect(h).toBeLessThan(40)
  })

  it('перила рисуются при положительной высоте ограждения', () => {
    const withRail: FlightResult = { ...base, RailingHeight: 900 }
    const { container } = render(<StairProfile flight={withRail} />)
    expect(container.querySelectorAll('.scheme__railing').length).toBe(1)
    expect(container.querySelectorAll('.scheme__post').length).toBe(2)
  })

  it('без высоты ограждения перила не рисуются', () => {
    const { container } = render(<StairProfile flight={base} />)
    expect(container.querySelectorAll('.scheme__railing').length).toBe(0)
  })

  it('косоур — гребенка: пила с посадочными местами и вертикальными сбросами', () => {
    const { container } = render(<StairProfile flight={{ ...base, StringerThickness: 50 }} />)
    const poly = container.querySelector('.scheme__stringer')
    expect(poly).not.toBeNull()
    const pts = (poly!.getAttribute('points') ?? '')
      .trim()
      .split(/\s+/)
      .map((p) => {
        const [x, y] = p.split(',').map(Number)
        return { x, y }
      })
    expect(pts.length).toBeGreaterThan(4)
    // Количество вершин: голова пилы + 2 вершины на ступень + низ(F) +
    // зад(P_top) + закрытие к голове.
    expect(pts.length).toBe(2 * base.StepCount + 4)
    // Есть горизонтальные посадочные места и вертикальные сбросы.
    const hasSeat = pts.some((p, i) => i > 0 && p.y === pts[i - 1].y && p.x > pts[i - 1].x)
    const hasDrop = pts.some((p, i) => i > 0 && p.x === pts[i - 1].x && p.y !== pts[i - 1].y)
    expect(hasSeat).toBe(true)
    expect(hasDrop).toBe(true)
    // Спина доски: низ(F) + зад(P_top) + закрытие к голове пилы.
    const F = pts[pts.length - 3]
    const back = pts[pts.length - 2]
    const head = pts[pts.length - 1]
    expect(head.x).toBe(pts[0].x)
    expect(head.y).toBe(pts[0].y)
    // Спина параллельна маршу.
    const slope = (F.y - back.y) / (F.x - back.x)
    expect(slope).toBeCloseTo((base.StepCount * base.StepHeight) / base.Run, 5)
    // Нижняя линия доски лежит ниже головы пилы, а не поверх ступеней.
    expect(back.y).toBeGreaterThan(head.y)
  })

  it('перила и косоур целиком помещаются в viewBox', () => {
    const tall: FlightResult = { ...base, RailingHeight: 1500, StringerThickness: 50 }
    const { container } = render(<StairProfile flight={tall} />)
    const xs: number[] = []
    const ys: number[] = []
    container.querySelectorAll('.scheme__railing, .scheme__post').forEach((line) => {
      xs.push(Number(line.getAttribute('x1')), Number(line.getAttribute('x2')))
      ys.push(Number(line.getAttribute('y1')), Number(line.getAttribute('y2')))
    })
    container.querySelectorAll('.scheme__step, .scheme__stringer, .scheme__riser').forEach((el) => {
      el.getAttribute('points')?.split(/\s+/).forEach((p) => {
        const [x, y] = p.split(',').map(Number)
        xs.push(x)
        ys.push(y)
      })
    })
    expect(xs.length).toBeGreaterThan(0)
    xs.forEach((x) => {
      expect(x).toBeGreaterThanOrEqual(0)
      expect(x).toBeLessThanOrEqual(620)
    })
    ys.forEach((y) => {
      expect(y).toBeGreaterThanOrEqual(0)
      expect(y).toBeLessThanOrEqual(340)
    })
  })
})