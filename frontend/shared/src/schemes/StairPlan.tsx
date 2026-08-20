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
  // Перила на виде сверху (прямой марш): высота и эффективная сторона
  // (CONF-RAILING). Отсутствие — legacy: рисуем, если задана высота.
  RailingHeight?: number
  Railing?: string
}

// PlanExtras — дополнительные параметры L/U/спирали (структурно совместима).
export interface PlanExtras {
  lowerStepCount?: number
  upperStepCount?: number
  landingWidth?: number
  lowerRun?: number
  upperRun?: number
  // Направление поворота (CONF-DIRECTION): 'right' — площадка справа (как сейчас),
  // 'left' — план зеркалится по горизонтали.
  direction?: 'left' | 'right'
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

// Размер канваса совпадает со StairProfile (профиль): фиксированный viewBox
// 620×340 с полями PAD — вид сверху выглядит в том же масштабе, что и профиль.
const W = 620
const H = 340
const PAD = 62

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
      const uEnd = lw - f.Width + ur
      const pts = [
        { x: 0, y: -f.Width },
        { x: lr, y: -f.Width },
        { x: lr + lw, y: -f.Width },
        { x: lr + lw, y: uEnd },
        { x: lr + lw - f.Width, y: uEnd },
        { x: lr + lw - f.Width, y: lw - f.Width },
        { x: lr, y: lw - f.Width },
        { x: lr, y: 0 },
        { x: 0, y: 0 },
      ]
      return {
        w: lr + lw,
        h: mathMax(pts.map((q) => q.y)) - mathMin(pts.map((q) => q.y)),
        pts: pts.map((q) => ({ x: (s.direction === 'left' ? -1 : 1) * q.x, y: q.y })),
      }
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
      return {
        w: lr + lw,
        h: gap + f.Width,
        pts: pts.map((q) => ({ x: (s.direction === 'left' ? -1 : 1) * q.x, y: q.y })),
      }
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
  // Центрирование чертежа с учётом знака координат: слева сверху — minX,
  // сверху (screen y) — maxY. Без этого при minX<0<spiral/maxY>0 контент
  // сдвигается влево/вверх и вылезает за пределы viewBox.
  const ox = PAD + (areaW - cw * scale) / 2 - mathMin(pts.map((q) => q.x)) * scale
  const oy = PAD + (areaH - ch * scale) / 2 + mathMax(pts.map((q) => q.y)) * scale
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
    const h = fontSize + 4
    const pt = placeLabel({ ax: cx, ay: cy, w, h, dx, dy, obstacles: [outline], placed, step: 16 })
    // Подпись целиком внутри канваса (фиксированный viewBox): не даём
    // placeLabel увести её за края при тесных раскладках (например, «B …»).
    pt.x = Math.max(w / 2 + 6, Math.min(W - w / 2 - 6, pt.x))
    pt.y = Math.max(h / 2 + 6, Math.min(H - h / 2 - 6, pt.y))
    placed.push(rectAt(pt.x, pt.y, w, h))
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

  // dirArrow — односторонняя стрелка направления (подъём марша).
  const dirArrow = (x1: number, y1: number, x2: number, y2: number, markerId: string) => (
    <line x1={x1} y1={y1} x2={x2} y2={y2} className="scheme__dir" markerEnd={`url(#${markerId})`} />
  )

  // Перила (CONF-RAILING): рисуем, если задана высота и сторона не «без перил».
  const showRail = (f.RailingHeight ?? 0) > 0 && f.Railing !== 'none'

  let body: ReactElement | null = null
  if (kind === 'straight') {
    // Подъём справа налево: левая рука — нижняя кромка экрана (py(0)),
    // правая — верхняя (py(f.Width)).
    const railing = (side: 'left' | 'right') =>
      showRail && (f.Railing === 'both' || f.Railing === side) ? (
        <line x1={px(0)} y1={side === 'left' ? py(0) : py(f.Width)} x2={px(f.Run)} y2={side === 'left' ? py(0) : py(f.Width)} className="scheme__railing" />
      ) : null
    body = (
      <>
        <rect x={px(0)} y={py(f.Width)} width={f.Run * scale} height={f.Width * scale} className="scheme__fill" />
        <rect x={px(0)} y={py(f.Width)} width={f.Run * scale} height={f.Width * scale} className="scheme__outline" fill="none" />
        {railing('left')}
        {railing('right')}
        {Array.from({ length: f.StepCount }, (_, i) => i + 1).map((i) => (
          <line key={i} x1={px(f.Run - i * f.TreadDepth)} y1={py(f.Width)} x2={px(f.Run - i * f.TreadDepth)} y2={py(0)} className="scheme__tread" />
        ))}
        {/* Направление совпадает с профилем (StairProfile): первая ступень справа,
            подъём — справа налево. */}
        {dirArrow(px(f.Run), py(f.Width / 2), px(0), py(f.Width / 2), 'pln-dir')}
        {arrowLine(px(f.Run), py(0) + 18, px(0), py(0) + 18, 'pln-arr')}
        {dim('L ' + fmt(f.Run), (px(0) + px(f.Run)) / 2, py(0) + 18, 'middle', 0, 1)}
        {arrowLine(px(f.Run) + 20, py(f.Width), px(f.Run) + 20, py(0), 'pln-arr')}
        {dim('B ' + fmt(f.Width), px(f.Run) + 20, py(f.Width / 2), 'middle', 1, 0)}
      </>
    )
  } else if (kind === 'l_shape') {
    const lr = s.lowerRun ?? f.Run
    const ur = s.upperRun ?? f.Run
    const lw = s.landingWidth ?? f.Width
    const wBig = f.Width
    // Направление поворота (CONF-DIRECTION): 'left' зеркалит план по горизонтали —
    // площадка в левом нижнем углу, верхний марш по левой стороне.
    const mp = s.direction === 'left' ? -1 : 1
    const P = (x: number) => px(mp * x)
    // «Буква L»: нижний марш внизу, площадка в правом нижнем углу, верхний марш
    // идёт вверх по правой стороне. Левая кромка площадки совпадает с правой
    // кромкой нижнего марша — стык по всей ширине плоскости ступеней.
    const uEnd = lw - wBig + ur
    const rects: Array<[number, number, number, number]> = [
      [0, -wBig, lr, wBig],
      [lr, -wBig, lw, lw],
      [lr + lw - wBig, lw - wBig, wBig, ur],
    ]
    const rim: Array<[number, number]> = [
      [0, -wBig],
      [lr, -wBig],
      [lr + lw, -wBig],
      [lr + lw, uEnd],
      [lr + lw - wBig, uEnd],
      [lr + lw - wBig, lw - wBig],
      [lr, lw - wBig],
      [lr, 0],
      [0, 0],
    ]
    body = (
      <>
        {rects.map(([x, y, w2, h2], i) => (
          <g key={i}>
            <rect x={mp === -1 ? P(x + w2) : P(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__fill" />
            <rect x={mp === -1 ? P(x + w2) : P(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__outline" fill="none" />
          </g>
        ))}
        {showRail && (
          <polyline points={rim.map(([q, r]) => `${P(q)},${py(r)}`).join(' ')} className="scheme__railing" />
        )}
        {Array.from({ length: s.lowerStepCount ?? Math.floor(lr / f.TreadDepth) }, (_, i) => i + 1).map((i) =>
          i * f.TreadDepth < lr ? (
            <line key={`l${i}`} x1={P(i * f.TreadDepth)} y1={py(0)} x2={P(i * f.TreadDepth)} y2={py(-wBig)} className="scheme__tread scheme__tread--lower" />
          ) : null,
        )}
        {Array.from({ length: s.upperStepCount ?? Math.floor(ur / f.TreadDepth) }, (_, i) => i + 1).map((i) =>
          i * f.TreadDepth < ur ? (
            <line key={`u${i}`} x1={P(lr + lw - wBig)} y1={py(lw - wBig + i * f.TreadDepth)} x2={P(lr + lw)} y2={py(lw - wBig + i * f.TreadDepth)} className="scheme__tread scheme__tread--upper" />
          ) : null,
        )}
        {dirArrow(P(0), py(-wBig / 2), P(lr), py(-wBig / 2), 'pln-dir')}
        {dirArrow(P(lr + lw - wBig / 2), py(lw - wBig), P(lr + lw - wBig / 2), py(uEnd), 'pln-dir')}
        {arrowLine(P(0), py(-wBig) + 18, P(lr), py(-wBig) + 18, 'pln-arr')}
        {dim('L₁ ' + fmt(lr), (P(0) + P(lr)) / 2, py(-wBig) + 18, 'middle', 0, 1)}
        {arrowLine(P(lr + lw) + mp * 20, py(lw - wBig), P(lr + lw) + mp * 20, py(uEnd), 'pln-arr')}
        {dim('L₂ ' + fmt(ur), P(lr + lw) + mp * 20, (py(lw - wBig) + py(uEnd)) / 2, 'middle', 1, 0)}
        {arrowLine(P(0) - mp * 18, py(-wBig), P(0) - mp * 18, py(0), 'pln-arr')}
        {dim('B ' + fmt(wBig), P(0) - mp * 18, (py(-wBig) + py(0)) / 2, 'middle', -1, 0)}
      </>
    )
  } else if (kind === 'u_shape') {
    const lr = s.lowerRun ?? f.Run
    const ur = s.upperRun ?? f.Run
    const lw = s.landingWidth ?? f.Width
    const gap = Math.max(60, (lw - f.Width) / 2)
    const wBig = f.Width
    // Направление поворота (CONF-DIRECTION): 'left' зеркалит план по горизонтали.
    const mp = s.direction === 'left' ? -1 : 1
    const P = (x: number) => px(mp * x)
    const rects: Array<[number, number, number, number]> = [
      [0, 0, lr, wBig],
      [lr, 0, lw, gap + wBig],
      [0, gap, ur, wBig],
    ]
    const rim: Array<[number, number]> = [
      [0, wBig],
      [lr, wBig],
      [lr, 0],
      [lr + lw, 0],
      [lr + lw, gap + wBig],
      [ur, gap + wBig],
      [ur, gap],
      [0, gap],
    ]
    body = (
      <>
        {rects.map(([x, y, w2, h2], i) => (
          <g key={i}>
            <rect x={mp === -1 ? P(x + w2) : P(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__fill" />
            <rect x={mp === -1 ? P(x + w2) : P(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__outline" fill="none" />
          </g>
        ))}
        {showRail && (
          <polyline points={rim.map(([q, r]) => `${P(q)},${py(r)}`).join(' ')} className="scheme__railing" />
        )}
        {Array.from({ length: s.lowerStepCount ?? Math.floor(lr / f.TreadDepth) }, (_, i) => i + 1).map((i) =>
          i * f.TreadDepth < lr ? (
            <line key={`l${i}`} x1={P(i * f.TreadDepth)} y1={py(0)} x2={P(i * f.TreadDepth)} y2={py(wBig)} className="scheme__tread scheme__tread--lower" />
          ) : null,
        )}
        {Array.from({ length: s.upperStepCount ?? Math.floor(ur / f.TreadDepth) }, (_, i) => i + 1).map((i) =>
          i * f.TreadDepth < ur ? (
            <line key={`u${i}`} x1={P(i * f.TreadDepth)} y1={py(gap)} x2={P(i * f.TreadDepth)} y2={py(gap + wBig)} className="scheme__tread scheme__tread--upper" />
          ) : null,
        )}
        {dirArrow(P(0), py(wBig / 2), P(lr), py(wBig / 2), 'pln-dir')}
        {dirArrow(P(ur), py(gap + wBig / 2), P(0), py(gap + wBig / 2), 'pln-dir')}
        {arrowLine(P(0), py(gap + wBig) - 18, P(lr), py(gap + wBig) - 18, 'pln-arr')}
        {dim('L₁ ' + fmt(lr), (P(0) + P(lr)) / 2, py(gap + wBig) - 18, 'middle', 0, -1)}
        {arrowLine(P(0), py(gap) + 18, P(ur), py(gap) + 18, 'pln-arr')}
        {dim('L₂ ' + fmt(ur), (P(0) + P(ur)) / 2, py(gap) + 18, 'middle', 0, 1)}
        {arrowLine(P(0) - mp * 18, py(gap + wBig), P(0) - mp * 18, py(gap), 'pln-arr')}
        {dim('B ' + fmt(wBig), P(0) - mp * 18, (py(gap + wBig) + py(gap)) / 2, 'middle', -1, 0)}
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

  // Фиксированный канвас как у StairProfile: вид сверху центрирован с полями
  // PAD со всех сторон — тот же масштаб, что и профиль.
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
          <marker id="pln-dir" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="9" markerHeight="9" orient="auto">
            <path d="M 0 0 L 10 5 L 0 10 z" className="scheme__dir" />
          </marker>
        </defs>
        {body}
      </svg>
      <p className="scheme__caption">План: вид сверху, размеры в мм.</p>
    </div>
  )
}