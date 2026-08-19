// Схема прямого марша (FE-0017): боковой профиль лестницы из данных
// Solver (flight). Строится чисто на фронтенде — SVG, без запросов к API.
// Рисует реальную геометрию «как посчитано» (BC-002): ступени с
// подступенками (закрытый марш) или без них (открытый), косоур и перила.
// Подписи размеров расставляются с избеганием наложений (annotate), как в
// StairPlan: каждая метка ищет свободное место вне контура чертежа.

import type { FlightResult } from '../types'
import {
  bboxRect,
  placeLabel,
  rectAt,
  textWidth,
  type Point,
  type Rect,
} from './annotate'

interface Props {
  flight: FlightResult
}

const W = 620
const H = 340
const PAD = 62

interface MM {
  x: number
  y: number
}

export function StairProfile({ flight }: Props) {
  const { StepCount, StepHeight, TreadDepth, Run, Stringer, Angle } = flight
  if (StepCount <= 0 || StepHeight <= 0) return null

  const angleDeg = (Angle * 180) / Math.PI
  const closed = flight.Riser !== false
  const stepTh = Math.min(Math.max(flight.StepThickness ?? 0, 0), StepHeight)
  const railingH = Math.max(flight.RailingHeight ?? 0, 0)
  const st = Math.max(flight.StringerThickness ?? 40, 0) // толщина косоура, мм

  const totalH = StepCount * StepHeight
  const runTotal = Run > 0 ? Run : StepCount * TreadDepth
  const len = Math.hypot(runTotal, totalH)

  // Вертикальный охват чертежа: перила сверху, косоур обрезан у пола снизу.
  const topMM = totalH + railingH
  const spanMM = topMM

  const areaW = W - PAD * 2
  const areaH = H - PAD * 2
  const scale = Math.min(areaW / runTotal, areaH / spanMM)
  const ox = PAD + (areaW - runTotal * scale) / 2
  const oy = PAD + (areaH - spanMM * scale) / 2

  const px = (mm: number) => ox + mm * scale
  const py = (mm: number) => oy + (topMM - mm) * scale
  const pt = (p: MM) => `${px(p.x)},${py(p.y)}`

  // Носовая линия проступей (верхние кромки ступеней).
  const pts: MM[] = []
  let curX = 0
  let curY = totalH
  pts.push({ x: curX, y: curY })
  for (let i = 0; i < StepCount; i++) {
    curY -= StepHeight
    pts.push({ x: curX, y: curY })
    curX += TreadDepth
    pts.push({ x: curX, y: curY })
  }

  // Ступени: проступь — пластина толщиной stepTh, лежащая на вырезе
  // косоура (седло пилы = низ проступи). Подступенок — отдельная
  // вертикальная пластина толщиной stepTh и высотой StepHeight, прилегающая
  // к торцевой стороне пилы. При stepTh<=0 закрытый марш падает на полный
  // прямоугольник ступени (без разбиения на детали).
  const sTh = stepTh > 0 ? stepTh : StepHeight
  const steps: string[] = []
  const risers: string[] = []
  for (let i = 0; i < StepCount; i++) {
    const topY = totalH - i * StepHeight
    const botY = topY - StepHeight
    const x0 = i * TreadDepth
    const x1 = (i + 1) * TreadDepth
    if (stepTh <= 0) {
      if (closed) {
        steps.push(`${pt({ x: x0, y: botY })} ${pt({ x: x1, y: botY })} ${pt({ x: x1, y: topY })} ${pt({ x: x0, y: topY })}`)
      }
      continue
    }
    const th = Math.min(stepTh, StepHeight)
    const treadBot = topY - th
    steps.push(`${pt({ x: x0, y: treadBot })} ${pt({ x: x0, y: topY })} ${pt({ x: x1, y: topY })} ${pt({ x: x1, y: treadBot })}`)
    if (closed) {
      const r0 = x1 - th
      risers.push(`${pt({ x: r0, y: botY })} ${pt({ x: r0, y: topY })} ${pt({ x: x1, y: topY })} ${pt({ x: x1, y: botY })}`)
    }
  }

  // Косоур-гребенка: пила вплотную под нижним контуром ступеней —
  // посадочное место под проступью + вертикальный сброс на границе
  // ступеней; спина (низ доски) — прямая, параллельная маршу, на толщину
  // st ниже посадочных мест.
  const ux = len > 0 ? -totalH / len : 0
  const uy = len > 0 ? -runTotal / len : 0
  const tTh = sTh
  const seatY = (i: number) => totalH - i * StepHeight - tTh
  const saw: MM[] = [{ x: 0, y: seatY(0) }]
  for (let i = 0; i < StepCount; i++) {
    const x1 = (i + 1) * TreadDepth
    saw.push({ x: x1, y: seatY(i) })
    saw.push({ x: x1, y: i === StepCount - 1 ? 0 : seatY(i + 1) })
  }
  // Спинка (низ доски): линия посадочных мест, сдвинутая на толщину st
  // перпендикулярно вниз; спереди упирается в пол, сзади — перпендикуляр
  // из головы пилы.
  const P_top: MM = { x: ux * st, y: seatY(0) + uy * st }
  const t = P_top.y / totalH
  const fx = Math.max(0, Math.min(runTotal, P_top.x + t * runTotal))
  const F: MM = { x: fx, y: 0 }
  const comb = `${saw.map((p) => pt(p)).join(' ')} ${pt(F)} ${pt(P_top)} ${pt({ x: 0, y: seatY(0) })}`
  const sA = { x: px(P_top.x), y: py(P_top.y) }
  const sB = { x: px(F.x), y: py(F.y) }

  const dimLabel = (v: number, unit: string) => `${Math.round(v).toLocaleString('ru-RU')} ${unit}`

  const dimX = ox - 24
  const runY = py(0) + 22
  const midY = (py(totalH) + py(0)) / 2

  // Препятствия для подписей: контур чертежа и размерная линия длины.
  const outline: Rect = bboxRect(pts.map((p) => ({ x: px(p.x), y: py(p.y) })))
  const obstacles: Rect[] = [
    outline,
    { x: px(0), y: runY - 1, w: px(runTotal) - px(0), h: 2 },
  ]
  const placed: Rect[] = []
  const place = (
    text: string,
    ax: number,
    ay: number,
    dx: number,
    dy: number,
    rotated = false,
  ): Point => {
    const fontSize = 11
    const w = textWidth(text, fontSize)
    const h = fontSize + 4
    const cw = rotated ? h : w
    const ch = rotated ? w : h
    const pt = placeLabel({ ax, ay, w: cw, h: ch, dx, dy, obstacles, placed, step: 16 })
    placed.push(rectAt(pt.x, pt.y, cw, ch))
    return pt
  }

  const hp = place(dimLabel(totalH, 'мм'), dimX - 8, midY, -1, 0, true)
  const rp = place(
    dimLabel(runTotal, 'мм'),
    (px(0) + px(runTotal)) / 2,
    runY + 16,
    0,
    1,
  )
  const spLine = {
    x: (sA.x + sB.x) / 2,
    y: (sA.y + sB.y) / 2,
  }
  const spD = Math.hypot(sB.x - sA.x, sB.y - sA.y)
  const spUx = spD > 0 ? (sB.y - sA.y) / spD : 0
  const spUy = spD > 0 ? -(sB.x - sA.x) / spD : -1
  const sp = {
    x: Math.max(30, Math.min(W - 30, spLine.x + 12 + spUx * 16)),
    y: Math.max(20, Math.min(H - 20, spLine.y - 8 + spUy * 16)),
  }
  const stp = place(`${StepCount} ступ.`, px(0) - 10, py(0) + 4, -1, 0)
  const ap = place(`${angleDeg.toFixed(1)}°`, px(0) - 34, py(totalH) + 8, -1, 0)

  // Перила: верхняя перекладина параллельна маршу на высоте railingH от
  // носовых кромок, плюс две крайние стойки.
  const rail = railingH > 0 && (
    <>
      <line
        x1={px(0)}
        y1={py(totalH + railingH)}
        x2={px(runTotal)}
        y2={py(railingH)}
        className="scheme__railing"
      />
      <line x1={px(0)} y1={py(totalH)} x2={px(0)} y2={py(totalH + railingH)} className="scheme__post" />
      <line x1={px(runTotal)} y1={py(0)} x2={px(runTotal)} y2={py(railingH)} className="scheme__post" />
    </>
  )

  return (
    <div className="scheme">
      <svg
        className="scheme__svg"
        viewBox={`0 0 ${W} ${H}`}
        role="img"
        aria-label="Боковой профиль прямого марша"
      >
        <defs>
          <marker
            id="sp-arr"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerWidth="7"
            markerHeight="7"
            orient="auto-start-reverse"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" className="scheme__dimline" />
          </marker>
        </defs>

        <polygon points={comb} className="scheme__stringer" />
        {risers.map((poly, i) => (
          <polygon key={i} points={poly} className="scheme__riser" />
        ))}
        {steps.map((poly, i) => (
          <polygon key={i} points={poly} className="scheme__step" />
        ))}
        {rail}

        <text
          x={stp.x}
          y={stp.y}
          className="scheme__label"
          textAnchor="middle"
          dominantBaseline="middle"
        >
          {StepCount} ступ.
        </text>

        <line
          x1={dimX}
          y1={py(totalH)}
          x2={dimX}
          y2={py(0)}
          className="scheme__dimline"
          markerStart="url(#sp-arr)"
          markerEnd="url(#sp-arr)"
        />
        <text
          x={hp.x}
          y={hp.y}
          className="scheme__dimtext"
          textAnchor="middle"
          dominantBaseline="middle"
          transform={`rotate(-90 ${hp.x} ${hp.y})`}
        >
          {dimLabel(totalH, 'мм')}
        </text>

        <line
          x1={px(0)}
          y1={runY}
          x2={px(runTotal)}
          y2={runY}
          className="scheme__dimline"
          markerStart="url(#sp-arr)"
          markerEnd="url(#sp-arr)"
        />
        <text
          x={rp.x}
          y={rp.y}
          className="scheme__dimtext"
          textAnchor="middle"
          dominantBaseline="middle"
        >
          {dimLabel(runTotal, 'мм')}
        </text>

        <line
          x1={sA.x + 12}
          y1={sA.y - 8}
          x2={sB.x + 12}
          y2={sB.y - 8}
          className="scheme__dimline"
        />
        <text
          x={sp.x}
          y={sp.y}
          className="scheme__dimtext scheme__dimtext--stringer"
          textAnchor="middle"
          dominantBaseline="middle"
        >
          {dimLabel(Stringer, 'мм')}
        </text>

        <path
          d={`M ${px(0)} ${py(totalH)} A ${14} ${14} 0 0 0 ${px(0) + 10} ${py(totalH) - 10}`}
          fill="none"
          className="scheme__dimline"
        />
        <text
          x={ap.x}
          y={ap.y}
          className="scheme__label"
          textAnchor="middle"
          dominantBaseline="middle"
        >
          {angleDeg.toFixed(1)}°
        </text>
      </svg>
      <p className="scheme__caption">
        Ступени: высота {StepHeight.toLocaleString('ru-RU')} мм · глубина{' '}
        {TreadDepth.toLocaleString('ru-RU')} мм · угол {angleDeg.toFixed(1)}° ·{' '}
        {closed ? 'с подступенком' : 'открытый марш'}
        {railingH > 0 ? ` · перила ${railingH.toLocaleString('ru-RU')} мм` : ''}
      </p>
    </div>
  )
}