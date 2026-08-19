// StairPlan — вид сверху (план) лестницы для чертёжной секции результата
// расчёта. SVG без запросов к API: каркас (вид сверху) строим из данных
// Solver. Подписи размеров расставляются с избеганием наложений (annotate).

import type { ReactElement } from 'react'
import { bboxRect, placeLabel, rectAt, textWidth, type Rect } from './annotate'

// PlanFlight — структурно совместима с FlightView сторa (плюс Width).
export interface PlanFlight {
  StepCount: number
  StepHeight: number
  TreadDepth: number
  Run: number
  Stringer: number
  Angle: number
  Width: number
}

// PlanExtras — дополнительные параметры L/U/спирали (структурно совместима).
export interface PlanExtras {
  lowerStepCount?: number
  upperStepCount?: number
  landingWidth?: number
  lowerRun?: number
  upperRun?: number
  outerRadius?: number
  columnRadius?: number
  walkRadius?: number
  innerTread?: number
  outerTread?: number
  angularStep?: number
  angularTotal?: number
  arcLength?: number
}

export type PlanKind = 'straight' | 'l_shape' | 'u_shape' | 'spiral'

interface Props {
  flight: PlanFlight
  kind: PlanKind
  solver: PlanExtras
}

const W = 560
const H = 340
const PAD = 66

const fmt = (v: number) => `${Math.round(v).toLocaleString('ru-RU')} мм`

// planGeometry возвращает контур плана в мм (для scale и obstacle-бокса).
function contentMm(p: Props): { w: number; h: number; pts: Array<{ x: number; y: number }> } {
  const { flight: f, kind, solver: s } = p
  switch (kind) {
    case 'straight':
      return { w: f.Run, h: f.Width, pts: [{ x: 0, y: 0 }, { x: f.Run, y: 0 }, { x: f.Run, y: f.Width }, { x: 0, y: f.Width }] }
    case 'l_shape': {
      const lr = s.lowerRun ?? f.Run
      const ur = s.upperRun ?? f.Run
      const lw = s.landingWidth ?? f.Width
      const pts = [
        { x: 0, y: 0 },
        { x: lr, y: 0 },
        { x: lr + lw, y: -lw },
        { x: lr + lw, y: -lw - ur },
        { x: lr + lw - f.Width, y: -lw - ur },
        { x: lr + lw - f.Width, y: -lw },
        { x: lr + lw - f.Width, y: -f.Width },
        { x: lr, y: -f.Width },
      ]
      return { w: lr + lw, h: mathMax(pts.map((q) => q.y)) - mathMin(pts.map((q) => q.y)), pts }
    }
    case 'u_shape': {
      const lr = s.lowerRun ?? f.Run
      const ur = s.upperRun ?? f.Run
      const lw = s.landingWidth ?? f.Width
const gap = Math.max(60, (lw - f.Width) / 2)
      const pts = [
        { x: 0, y: 0 },
        { x: lr, y: 0 },
        { x: lr + lw, y: 0 },
        { x: lr + lw, y: gap + f.Width },
        { x: ur, y: gap + f.Width },
        { x: ur, y: gap },
        { x: 0, y: gap },
      ]
      return { w: lr + lw, h: gap + f.Width, pts }
    }
    case 'spiral': {
      const r = s.outerRadius ?? f.Width + 400
      const d = r * 2
      return { w: d, h: d, pts: [{ x: -r, y: -r }, { x: r, y: -r }, { x: r, y: r }, { x: -r, y: r }] }
    }
  }
}

function mathMax(a: number[]): number {
  return a.reduce((m, v) => (v > m ? v : m), -Infinity)
}
function mathMin(a: number[]): number {
  return a.reduce((m, v) => (v < m ? v : m), Infinity)
}

export function StairPlan({ flight: f, kind, solver: s }: Props) {
  if (!f || f.StepCount <= 0 || f.Width <= 0) return null

  const content = contentMm({ flight: f, kind, solver: s })
  const { w: cw, h: ch, pts } = content
  const areaW = W - PAD * 2
  const areaH = H - PAD * 2
  const scale = Math.min(areaW / cw, areaH / ch)
  const ox = PAD + (areaW - cw * scale) / 2
  const oy = PAD + (areaH - ch * scale) / 2
  const px = (x: number) => ox + x * scale
  const py = (y: number) => oy + -y * scale

  // Контур чертежа как препятствие для подписей.
  const outline: Rect = bboxRect(pts.map((p) => ({ x: px(p.x), y: py(p.y) })))
  const placed: Rect[] = []

  const dim = (
    text: string,
    cx: number,
    cy: number,
    anchor: 'start' | 'middle' | 'end',
    dx: number,
    dy: number,
    fontSize = 11,
  ) => {
    const w = textWidth(text, fontSize)
    const pt = placeLabel({ ax: cx, ay: cy, w, h: fontSize + 4, dx, dy, obstacles: [outline], placed, step: 16 })
    placed.push(rectAt(pt.x, pt.y, w, fontSize + 4))
    return (
      <text
        x={pt.x}
        y={pt.y}
        textAnchor={anchor}
        dominantBaseline="middle"
        className="scheme__dimtext"
        fontSize={fontSize}
      >
        {text}
      </text>
    )
  }

  const arrowLine = (x1: number, y1: number, x2: number, y2: number, markerId: string) => (
    <line
      x1={x1}
      y1={y1}
      x2={x2}
      y2={y2}
      className="scheme__dimline"
      markerStart={`url(#${markerId})`}
      markerEnd={`url(#${markerId})`}
    />
  )

  let body: ReactElement | null = null
  if (kind === 'straight') {
    body = (
      <>
        <rect x={px(0)} y={py(f.Width)} width={f.Run * scale} height={f.Width * scale} className="scheme__fill" />
        <rect x={px(0)} y={py(f.Width)} width={f.Run * scale} height={f.Width * scale} className="scheme__outline" fill="none" />
        {Array.from({ length: f.StepCount }, (_, i) => i + 1).map((i) => (
          <line key={i} x1={px(i * f.TreadDepth)} y1={py(f.Width)} x2={px(i * f.TreadDepth)} y2={py(0)} className="scheme__tread" />
        ))}
        <line x1={px(0)} y1={py(f.Width / 2)} x2={px(f.Run)} y2={py(f.Width / 2)} className="scheme__axis" />
        {arrowLine(px(0), py(f.Width) + 18, px(f.Run), py(f.Width) + 18, 'pln-arr')}
        {dim('L ' + fmt(f.Run), px(f.Run) / 2, py(f.Width) + 18, 'middle', 0, 1)}
        {arrowLine(px(0) - 20, py(f.Width), px(0) - 20, py(0), 'pln-arr')}
        {dim('B ' + fmt(f.Width), px(0) - 20, py(f.Width / 2), 'middle', -1, 0)}
      </>
    )
  } else if (kind === 'l_shape') {
    const lr = s.lowerRun ?? f.Run
    const ur = s.upperRun ?? f.Run
    const lw = s.landingWidth ?? f.Width
    const wBig = f.Width
    const rects: Array<[number, number, number, number]> = [
      [0, 0, lr, wBig],
      [lr, -lw, lw, lw],
      [lr + lw - wBig, -lw - ur, wBig, ur],
    ]
    body = (
      <>
        {rects.map(([x, y, w2, h2], i) => (
          <g key={i}>
            <rect x={px(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__fill" />
            <rect x={px(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__outline" fill="none" />
          </g>
        ))}
        {Array.from({ length: s.lowerStepCount ?? Math.floor(lr / f.TreadDepth) }, (_, i) => i + 1).map((i) =>
          i * f.TreadDepth < lr ? (
            <line key={`l${i}`} x1={px(i * f.TreadDepth)} y1={py(0)} x2={px(i * f.TreadDepth)} y2={py(-wBig)} className="scheme__tread" />
          ) : null,
        )}
        {Array.from({ length: s.upperStepCount ?? Math.floor(ur / f.TreadDepth) }, (_, i) => i + 1).map((i) =>
          i * f.TreadDepth < ur ? (
            <line key={`u${i}`} x1={px(lr + lw - wBig)} y1={py(-wBig - i * f.TreadDepth)} x2={px(lr + lw)} y2={py(-wBig - i * f.TreadDepth)} className="scheme__tread" />
          ) : null,
        )}
        <line x1={px(0)} y1={py(0)} x2={px(lr)} y2={py(0)} className="scheme__axis" />
        {arrowLine(px(0), py(-wBig - 24), px(lr), py(-wBig - 24), 'pln-arr')}
        {dim('L₁ ' + fmt(lr), px(lr) / 2, py(-wBig - 24), 'middle', 0, 1)}
        {arrowLine(px(lr + lw), py(-lw), px(lr + lw), py(-lw - ur - 24), 'pln-arr')}
        {dim('L₂ ' + fmt(ur), px(lr + lw), py(-lw - ur / 2 - 24), 'middle', 0, 1)}
        {dim('B ' + fmt(wBig), px(0) - 18, py(-wBig / 2), 'middle', -1, 0)}
      </>
    )
  } else if (kind === 'u_shape') {
    const lr = s.lowerRun ?? f.Run
    const ur = s.upperRun ?? f.Run
    const lw = s.landingWidth ?? f.Width
    const gap = Math.max(60, (lw - f.Width) / 2)
    const wBig = f.Width
    const rects: Array<[number, number, number, number]> = [
      [0, 0, lr, wBig],
      [lr, 0, lw, gap + wBig],
      [0, gap, ur, wBig],
    ]
    body = (
      <>
        {rects.map(([x, y, w2, h2], i) => (
          <g key={i}>
            <rect x={px(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__fill" />
            <rect x={px(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__outline" fill="none" />
          </g>
        ))}
        {arrowLine(px(0), py(-wBig - 24), px(lr), py(-wBig - 24), 'pln-arr')}
        {dim('L₁ ' + fmt(lr), px(lr) / 2, py(-wBig - 24), 'middle', 0, 1)}
        {arrowLine(px(ur), py(gap + wBig + 24), px(0) + (0), py(gap + wBig + 24), 'pln-arr')}
        {dim('L₂ ' + fmt(ur), px(ur / 2), py(gap + wBig + 24), 'middle', 0, 1)}
        {dim('B ' + fmt(wBig), px(0) - 18, py(-wBig / 2), 'middle', -1, 0)}
      </>
    )
  } else {
    // spiral
    const r = s.outerRadius ?? f.Width + 400
    const cr = s.columnRadius ?? 80
    const cx = 0
    const cy = 0
    const steps = Array.from({ length: f.StepCount }, (_, i) => i + 1).map((i) => {
      const a = (i * (s.angularStep ?? 15) * Math.PI) / 180
      return { a }
    })
    body = (
      <>
        <circle cx={px(cx)} cy={py(cy)} r={r * scale} className="scheme__outline" fill="none" />
        <circle cx={px(cx)} cy={py(cy)} r={cr * scale} className="scheme__column" />
        {steps.map((st, i) => {
          const x1 = Math.cos(st.a) * cr
          const y1 = Math.sin(st.a) * cr
          const x2 = Math.cos(st.a) * r
          const y2 = Math.sin(st.a) * r
          return <line key={i} x1={px(x1)} y1={py(y1)} x2={px(x2)} y2={py(y2)} className="scheme__tread" />
        })}
        <line x1={px(0)} y1={py(0)} x2={px(r)} y2={py(0)} className="scheme__dimline" markerStart="url(#pln-arr)" markerEnd="url(#pln-arr)" />
        {dim('R ' + fmt(r), px(r / 2), py(0) - 16, 'middle', 0, -1)}
        {dim('α ' + (s.angularTotal ?? 360).toFixed(0) + '°', px(cx), py(cy) - cr * scale - 14, 'middle', 0, -1)}
        <text x={px(cx)} y={py(cy)} textAnchor="middle" dominantBaseline="middle" className="scheme__label">
          {f.StepCount} ступ.
        </text>
      </>
    )
  }

  return (
    <div className="scheme">
      <svg
        className="scheme__svg"
        viewBox={`0 0 ${W} ${H}`}
        role="img"
        aria-label="Вид сверху (план) лестницы"
      >
        <defs>
          <marker id="pln-arr" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" className="scheme__dimline" />
          </marker>
        </defs>
        {body}
      </svg>
      <p className="scheme__caption">План: вид сверху, размеры в мм.</p>
    </div>
  )
}