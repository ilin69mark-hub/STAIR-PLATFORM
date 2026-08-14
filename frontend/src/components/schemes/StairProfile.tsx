// Схема прямого марша (FE-0017): боковой профиль лестницы из данных
// Solver (flight). Строится чисто на фронтенде — SVG, без запросов к API.

import type { FlightResult } from '../../api/types'

interface Props {
  flight: FlightResult
}

const W = 620
const H = 340
const PAD = 62

export function StairProfile({ flight }: Props) {
  const { StepCount, StepHeight, TreadDepth, Run, Stringer, Angle } = flight
  if (StepCount <= 0 || StepHeight <= 0) return null

  const angleDeg = (Angle * 180) / Math.PI

  const totalH = StepCount * StepHeight
  const runTotal = Run > 0 ? Run : StepCount * TreadDepth

  const areaW = W - PAD * 2
  const areaH = H - PAD * 2
  const scale = Math.min(areaW / runTotal, areaH / totalH)
  const ox = PAD + (areaW - runTotal * scale) / 2
  const oy = PAD + (areaH - totalH * scale) / 2

  const px = (mm: number) => ox + mm * scale
  const py = (mm: number) => oy + (totalH - mm) * scale

  const pts: Array<{ x: number; y: number }> = []
  let curX = 0
  let curY = totalH
  pts.push({ x: curX, y: curY })
  for (let i = 0; i < StepCount; i++) {
    curY -= StepHeight
    pts.push({ x: curX, y: curY })
    curX += TreadDepth
    pts.push({ x: curX, y: curY })
  }

  const nosing = pts.map((p) => `${px(p.x)},${py(p.y)}`).join(' ')
  const fillPath = `${nosing} ${px(runTotal)},${py(totalH)} ${px(0)},${py(totalH)}`

  const len = Math.hypot(runTotal, totalH)
  const off = Math.max(8, (Stringer * scale) / 3)
  const nx = (totalH / len) * off
  const ny = (runTotal / len) * off
  const sA = { x: px(0) + nx, y: py(totalH) + ny }
  const sB = { x: px(runTotal) + nx, y: py(0) + ny }

  const dimX = ox - 24
  const dimY = py(totalH) + 22

  const dimLabel = (v: number, unit: string) => `${Math.round(v).toLocaleString('ru-RU')} ${unit}`

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

        <polygon points={fillPath} className="scheme__fill" />
        <polyline points={nosing} className="scheme__outline" />

        <line x1={sA.x} y1={sA.y} x2={sB.x} y2={sB.y} className="scheme__stringer" />

        <text x={px(0) - 10} y={py(0) + 4} className="scheme__label" textAnchor="end">
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
          x={dimX - 6}
          y={(py(totalH) + py(0)) / 2}
          className="scheme__dimtext"
          textAnchor="end"
          transform={`rotate(-90 ${dimX - 6} ${(py(totalH) + py(0)) / 2})`}
        >
          {dimLabel(totalH, 'мм')}
        </text>

        <line
          x1={px(0)}
          y1={dimY}
          x2={px(runTotal)}
          y2={dimY}
          className="scheme__dimline"
          markerStart="url(#sp-arr)"
          markerEnd="url(#sp-arr)"
        />
        <text x={px(runTotal) / 2} y={dimY + 16} className="scheme__dimtext" textAnchor="middle">
          {dimLabel(runTotal, 'мм')}
        </text>

        <line x1={sA.x + 12} y1={sA.y - 8} x2={sB.x + 12} y2={sB.y - 8} className="scheme__dimline" />
        <text
          x={(sA.x + sB.x) / 2 + 12}
          y={(sA.y + sB.y) / 2 - 8}
          className="scheme__dimtext"
          textAnchor="middle"
        >
          {dimLabel(Stringer, 'мм')}
        </text>

        <path
          d={`M ${px(0)} ${py(totalH)} A ${14} ${14} 0 0 0 ${px(0) + 10} ${py(totalH) - 10}`}
          fill="none"
          className="scheme__dimline"
        />
        <text x={px(0) + 18} y={py(totalH) + 4} className="scheme__label">
          {angleDeg.toFixed(1)}°
        </text>
      </svg>
      <p className="scheme__caption">
        Ступени: высота {StepHeight.toLocaleString('ru-RU')} мм · глубина{' '}
        {TreadDepth.toLocaleString('ru-RU')} мм · угол {angleDeg.toFixed(1)}°
      </p>
    </div>
  )
}
