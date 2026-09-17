// StairPlan — вид сверху (план) лестницы для чертёжной секции результата
// расчёта. SVG без запросов к API: каркас (вид сверху) строим из данных
// Solver. Подписи размеров расставляются с избеганием наложений (annotate).

import type { ReactElement } from 'react'
import { bboxRect, placeLabel, rectAt, textWidth, type Rect } from './annotate'
import { computePlacement, type BBox2 } from '../placement'
import { ANNOTATE, edgeColor, edgeLabel, rectEdges, WALLS, type Side } from '../scheme-annot'

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
  landingDepth?: number
  roomWidth?: number
  roomLength?: number
  roomFits?: boolean
  // approachSpace — свободное пространство перед первой ступенью прямого
  // марша (мм). Равно сдвигу модели от стены (BoundingBox.Min.X); зона
  // рисуется перед первой ступенью на плане (EDR-0023).
  approachSpace?: number
  lowerRun?: number
  upperRun?: number
  // Сторона перил по сегментам (CONF-RAILING): первый марш / площадка / второй
  // марш. Отрисовка ведётся по контуру с учётом стороны движения (см. ветки
  // l_shape/u_shape). 'none' — без перил на сегменте; не задано — legacy (рисуем
  // как 'both').
  railingLower?: string
  railingLanding?: string
  railingUpper?: string
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

// Зона подхода (EDR-0023) совпадает с бэкендом (engine.go): при 0 или
// отсутствии значения движок использует 1000 мм. 2D-план и 3D-вьювер должны
// рисовать ту же зону, что и расчётное ядро, иначе подход «вылазиет» или
// отсутствует несогласованно с геометрией.
const effectiveApproach = (v: number | undefined): number => (v && v > 0 ? v : 1000)

// planGeometry возвращает контур плана в мм (для scale и obstacle-бокса).
function contentMm(p: Props): { w: number; h: number; pts: Array<{ x: number; y: number }> } {
  const { flight: f, kind, solver: s } = p
  switch (kind) {
    case 'straight':
      // Учитываем свободное пространство перед первой ступенью (EDR-0023):
      // оно рисуется справа от марша, поэтому включаем в габарит по X.
      // ВАЖНО: угол зоны подхода (f.Run+approach) должен входить в pts, иначе
      // габарит размещения (sb) игнорирует 1000 мм и лестница прижимается
      // к стене без резерва подхода (для L/U/спирали pts это уже учитывает).
      const ap = effectiveApproach(s.approachSpace)
      return {
        w: f.Run + ap,
        h: f.Width,
        pts: [
          { x: 0, y: 0 },
          { x: f.Run + ap, y: 0 },
          { x: f.Run + ap, y: f.Width },
          { x: 0, y: f.Width },
        ],
      }
    case 'l_shape': {
      const lr = s.lowerRun ?? f.Run
      const ur = s.upperRun ?? f.Run
      const lw = s.landingWidth ?? f.Width
      const ld = s.landingDepth ?? f.Width
      const wBig = f.Width
      // Габариты «буквы L»: X простирается до max(глубина площадки, ширина
      // марша), Y — от низа нижнего марша до верха верхнего марша.
      const maxX = lr + Math.max(ld, wBig)
      const maxY = lw - wBig + ur
      const minY = -wBig
      // Свободное пространство перед первой ступенью (EDR-0023) рисуется слева
      // от входа (нижнего марша, x=0) и учитывается в габарите по X.
      const ap = effectiveApproach(s.approachSpace)
      const pts = [
        { x: 0, y: minY },
        { x: maxX, y: minY },
        { x: maxX, y: maxY },
        { x: 0, y: maxY },
        { x: -ap, y: 0 },
      ]
      return {
        w: maxX + ap,
        h: maxY - minY,
        pts: pts.map((q) => ({ x: (s.direction === 'left' ? -1 : 1) * q.x, y: q.y })),
      }
    }
    case 'u_shape': {
      const lr = s.lowerRun ?? f.Run
      const ur = s.upperRun ?? f.Run
      const lw = s.landingWidth ?? f.Width
      const gap = Math.max(60, (lw - f.Width) / 2)
      // Свободное пространство перед первой ступенью (EDR-0023) слева от входа
      // (нижнего марша, x=0); учитывается в габарите по X.
      const ap = effectiveApproach(s.approachSpace)
      const pts = [
        { x: 0, y: 0 },
        { x: lr, y: 0 },
        { x: lr + lw, y: 0 },
        { x: lr + lw, y: gap + f.Width },
        { x: ur, y: gap + f.Width },
        { x: ur, y: gap },
        { x: 0, y: gap },
        { x: -ap, y: 0 },
      ]
      // Габарит по X: верхний марш (0..ur) может выступать за площадку
      // (lr..lr+lw) при малом нижнем марше (напр. lower=1, upper=17) —
      // иначе масштаб завышается и чертёж вылезает за viewBox.
      return {
        w: Math.max(lr + lw, ur) + ap,
        h: gap + f.Width,
        pts: pts.map((q) => ({ x: (s.direction === 'left' ? -1 : 1) * q.x, y: q.y })),
      }
    }
    case 'spiral': {
      const r = s.outerRadius ?? f.Width + 400
      const d = r * 2
      // Свободное пространство перед первой ступенью (EDR-0023) — снаружи у
      // входа (внешний радиус, +X); учитывается в габарите по X.
      const ap = effectiveApproach(s.approachSpace)
      return {
        w: d + ap,
        h: d,
        pts: [
          { x: -r, y: -r },
          { x: r, y: -r },
          { x: r, y: r },
          { x: -r, y: r },
          { x: r + ap, y: 0 },
        ],
      }
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

  // Размещение лестницы у дальней стены/угла помещения (placement.ts).
  // Габарит лестницы (включая зону подхода) считаем по фактическим точкам
  // плана; сдвиг применяем к группе марша, оставляя прямоугольник помещения
  // на месте.
  const sb: BBox2 = {
    minX: mathMin(pts.map((q) => q.x)),
    minY: mathMin(pts.map((q) => q.y)),
    maxX: mathMax(pts.map((q) => q.x)),
    maxY: mathMax(pts.map((q) => q.y)),
  }
  // Коэффициент зеркала потребителя: 2D-план зеркалит геометрию L/U через mp
  // (±1), иначе 1. Передаётся в placement, чтобы offsetX корректно прижимал
  // ребро к стене и в зеркальном варианте.
  const mirror: 1 | -1 =
    kind === 'l_shape' || kind === 'u_shape' ? (s.direction === 'left' ? -1 : 1) : 1
  const placement = computePlacement(kind, s.direction, s.roomWidth ?? 0, s.roomLength ?? 0, sb, mirror)
  // Признак переполнения периметра помещения (room_fit). Рендерим предупреждение
  // отдельной подписью ПОД SVG, чтобы оно не пересекалось с размерными линиями
  // («Гл», «L₁», «L₂», «B») внутри чертежа.
  const rw0 = s.roomWidth ?? 0
  const rl0 = s.roomLength ?? 0
  const roomOverflow = rw0 > 0 && rl0 > 0 && !(s.roomFits ?? false)
  // Требуемые габариты помещения для прямого марша: по оси X — длина забега
   // Требуемые габариты помещения показываем под планом (см. ясность
   // room_fit, EDR-0023) — берём из фактического габарита плана (cw/ch),
   // который уже включает свободное пространство перед первой ступенью.
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

  // Временная разметка рёбер марша: каждый прямоугольник (марш/площадка) получает
  // 4 цветных ребра с буквой <номер><В/Н/П/Л>. Цвета независимы (пёстрая отладка).
  const renderEdges = (rs: Array<[number, number, number, number]>) => {
    if (!ANNOTATE) return null
    return (
      <g className="scheme__annot">
        {rs.map((rect, fi) =>
          rectEdges(rect).map((seg, si) => {
            const color = edgeColor(fi * 4 + si)
            const x1 = px(seg.x1)
            const y1 = py(seg.y1)
            const x2 = px(seg.x2)
            const y2 = py(seg.y2)
            return (
              <g key={`${fi}-${seg.side}`}>
                <line x1={x1} y1={y1} x2={x2} y2={y2} className="scheme__edge" style={{ stroke: color }} />
                <text
                  x={(x1 + x2) / 2}
                  y={(y1 + y2) / 2}
                  className="scheme__elabel"
                  style={{ fill: color }}
                  textAnchor="middle"
                  dominantBaseline="middle"
                >
                  {edgeLabel(fi, seg.side)}
                </text>
              </g>
            )
          }),
        )}
      </g>
    )
  }

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
    const approach = effectiveApproach(s.approachSpace)
    const rw = s.roomWidth ?? 0
    const rl = s.roomLength ?? 0
    const roomFits = s.roomFits ?? false
    const roomRect =
      rw > 0 && rl > 0 ? (
        <rect
          x={Math.min(px(0), px(rw))}
          y={Math.min(py(0), py(rl))}
          width={Math.abs(px(rw) - px(0))}
          height={Math.abs(py(rl) - py(0))}
          className={roomFits ? 'scheme__room' : 'scheme__room scheme__room--overflow'}
        />
      ) : null
    const mp = 1
    const dx = mp * placement.offsetX * scale
    const dy = -placement.offsetY * scale
    body = (
      <>
        {roomRect}
        <g transform={`translate(${dx} ${dy})`}>
        <rect x={px(0)} y={py(f.Width)} width={f.Run * scale} height={f.Width * scale} className="scheme__fill" />
        <rect x={px(0)} y={py(f.Width)} width={f.Run * scale} height={f.Width * scale} className="scheme__outline" fill="none" />
        {renderEdges([[0, 0, f.Run, f.Width]])}
        {/* Свободное пространство перед первой ступенью (EDR-0023): зона, в
            которой человек должен встать, рисуется справа от марша (перед
            первой ступенью). */}
        {approach > 0 && (
          <>
            <rect
              x={px(f.Run)}
              y={py(f.Width)}
              width={approach * scale}
              height={f.Width * scale}
              className="scheme__approach"
            />
            {arrowLine(px(f.Run), py(0) + 18, px(f.Run + approach), py(0) + 18, 'pln-arr')}
            {dim('Свободное место ' + fmt(approach), (px(f.Run) + px(f.Run + approach)) / 2, py(0) + 18, 'middle', 0, 1)}
          </>
        )}
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
      </g>
      </>
    )
  } else if (kind === 'l_shape') {
    const lr = s.lowerRun ?? f.Run
    const ur = s.upperRun ?? f.Run
    const lw = s.landingWidth ?? f.Width
    const ld = s.landingDepth ?? f.Width
    const wBig = f.Width
    const ap = effectiveApproach(s.approachSpace)
    // Направление поворота (CONF-DIRECTION): 'left' зеркалит план по горизонтали —
    // площадка в левом нижнем углу, верхний марш по левой стороне.
    const mp = s.direction === 'left' ? -1 : 1
    const P = (x: number) => px(mp * x)
    // «Буква L»: нижний марш внизу, площадка-прямоугольник (глубина ld × ширина
    // lw) в правом нижнем углу, верхний марш идёт вверх по правой грани площадки
    // (прилегает к левой кромке площадки, X = lr).
    const uEnd = lw - wBig + ur
    const rects: Array<[number, number, number, number]> = [
      [0, -wBig, lr, wBig],
      [lr, -wBig, ld, lw],
      [lr, lw - wBig, wBig, ur],
    ]
    // rail — перила одного сегмента на выбранной стороне (CONF-RAILING):
    // 'right' → ВНЕШНЯЯ грань сегмента, 'left' → ВНУТРЕННЯЯ грань сегмента
    // (едино для всех маршей и площадок). 'both' рисует обе.
    const rail = (pts: Array<[number, number]>, sel: string | undefined, want: 'left' | 'right') => {
      const side = sel ?? 'both'
      if ((f.RailingHeight ?? 0) <= 0) return null
      if (!(side === 'both' || side === want)) return null
      return (
        <polyline points={pts.map(([qx, qy]) => `${P(qx)},${py(qy)}`).join(' ')} className="scheme__railing" />
      )
    }
    // Периметр помещения (roomWidth × roomLength, мм). Признак вписываемости
    // (roomFits) рассчитан по фактическому габаритному боксу лестницы и
    // совпадает с проверкой на бэкенде (room_fit).
    const rw = s.roomWidth ?? 0
    const rl = s.roomLength ?? 0
    const roomFits = s.roomFits ?? false
    const roomRect =
      rw > 0 && rl > 0 ? (
        <rect
          x={Math.min(px(0), px(rw))}
          y={Math.min(py(-wBig), py(-wBig + rl))}
          width={Math.abs(px(rw) - px(0))}
          height={Math.abs(py(-wBig + rl) - py(-wBig))}
          className={roomFits ? 'scheme__room' : 'scheme__room scheme__room--overflow'}
        />
      ) : null
    body = (
      <>
        {roomRect}
        <g transform={`translate(${placement.offsetX * scale} ${-placement.offsetY * scale})`}>
        {rects.map(([x, y, w2, h2], i) => (
          <g key={i}>
            <rect x={mp === -1 ? P(x + w2) : P(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__fill" />
            <rect x={mp === -1 ? P(x + w2) : P(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__outline" fill="none" />
          </g>
        ))}
        {renderEdges(rects)}
        {rail(
          [
            [0, -wBig],
            [lr, -wBig],
          ],
          s.railingLower,
          'right',
        )}
        {rail(
          [
            [0, 0],
            [lr, 0],
          ],
          s.railingLower,
          'left',
        )}
        {rail(
          [
            [lr, -wBig],
            [lr + ld, -wBig],
            [lr + ld, lw - wBig],
          ],
          s.railingLanding,
          'left',
        )}
        {rail(
          [
            [lr, 0],
            [lr, lw - wBig],
            [lr + ld, lw - wBig],
          ],
          s.railingLanding,
          'right',
        )}
        {rail(
          [
            [lr + wBig, lw - wBig],
            [lr + wBig, uEnd],
          ],
          s.railingUpper,
          'left',
        )}
        {rail(
          [
            [lr, lw - wBig],
            [lr, uEnd],
          ],
          s.railingUpper,
          'right',
        )}
        {Array.from({ length: s.lowerStepCount ?? Math.floor(lr / f.TreadDepth) }, (_, i) => i + 1).map((i) =>
          i * f.TreadDepth < lr ? (
            <line key={`l${i}`} x1={P(i * f.TreadDepth)} y1={py(0)} x2={P(i * f.TreadDepth)} y2={py(-wBig)} className="scheme__tread scheme__tread--lower" />
          ) : null,
        )}
        {Array.from({ length: s.upperStepCount ?? Math.floor(ur / f.TreadDepth) }, (_, i) => i + 1).map((i) =>
          i * f.TreadDepth < ur ? (
            <line key={`u${i}`} x1={P(lr)} y1={py(lw - wBig + i * f.TreadDepth)} x2={P(lr + wBig)} y2={py(lw - wBig + i * f.TreadDepth)} className="scheme__tread scheme__tread--upper" />
          ) : null,
        )}
        {dirArrow(P(0), py(-wBig / 2), P(lr), py(-wBig / 2), 'pln-dir')}
        {dirArrow(P(lr + wBig / 2), py(lw - wBig), P(lr + wBig / 2), py(uEnd), 'pln-dir')}
        {arrowLine(P(0), py(-wBig) + 18, P(lr), py(-wBig) + 18, 'pln-arr')}
        {dim('L₁ ' + fmt(lr), (P(0) + P(lr)) / 2, py(-wBig) + 18, 'middle', 0, 1)}
        {arrowLine(P(lr + wBig) + mp * 20, py(lw - wBig), P(lr + wBig) + mp * 20, py(uEnd), 'pln-arr')}
        {dim('L₂ ' + fmt(ur), P(lr + wBig) + mp * 20, (py(lw - wBig) + py(uEnd)) / 2, 'middle', 1, 0)}
        {arrowLine(P(lr), py(-wBig) - 18, P(lr + ld), py(-wBig) - 18, 'pln-arr')}
        {dim('Гл ' + fmt(ld), (P(lr) + P(lr + ld)) / 2, py(-wBig) - 18, 'middle', 0, -1)}
        {arrowLine(P(0) - mp * 18, py(-wBig), P(0) - mp * 18, py(0), 'pln-arr')}
        {dim('B ' + fmt(wBig), P(0) - mp * 18, (py(-wBig) + py(0)) / 2, 'middle', -1, 0)}
        {/* Свободное пространство перед первой ступенью (EDR-0023): зона перед
            входом (нижним маршем, x=0), рисуется слева. */}
        {ap > 0 && (
          <>
            <rect
              x={Math.min(P(0), P(-ap))}
              y={py(0)}
              width={Math.abs(P(0) - P(-ap))}
              height={wBig * scale}
              className="scheme__approach"
            />
            {arrowLine(P(0), py(-wBig / 2), P(-ap), py(-wBig / 2), 'pln-arr')}
            {dim('Свободное место ' + fmt(ap), (P(0) + P(-ap)) / 2, py(-wBig / 2), 'middle', 0, 1)}
          </>
        )}
        </g>
      </>
    )
  } else if (kind === 'u_shape') {
    const lr = s.lowerRun ?? f.Run
    const ur = s.upperRun ?? f.Run
    const lw = s.landingWidth ?? f.Width
    const gap = Math.max(60, (lw - f.Width) / 2)
    const wBig = f.Width
    const ap = effectiveApproach(s.approachSpace)
    // Направление поворота (CONF-DIRECTION): 'left' зеркалит план по горизонтали.
    const mp = s.direction === 'left' ? -1 : 1
    const P = (x: number) => px(mp * x)
    const rects: Array<[number, number, number, number]> = [
      [0, 0, lr, wBig],
      [lr, 0, lw, gap + wBig],
      [0, gap, ur, wBig],
    ]
    // rail — см. ветку l_shape: 'right' → ВНЕШНЯЯ грань сегмента,
    // 'left' → ВНУТРЕННЯЯ грань сегмента (едино для всех маршей/площадок).
    const rail = (pts: Array<[number, number]>, sel: string | undefined, want: 'left' | 'right') => {
      const side = sel ?? 'both'
      if ((f.RailingHeight ?? 0) <= 0) return null
      if (!(side === 'both' || side === want)) return null
      return (
        <polyline points={pts.map(([qx, qy]) => `${P(qx)},${py(qy)}`).join(' ')} className="scheme__railing" />
      )
    }
    const rw = s.roomWidth ?? 0
    const rl = s.roomLength ?? 0
    const roomFits = s.roomFits ?? false
    const roomRect =
      rw > 0 && rl > 0 ? (
        <rect
          x={Math.min(px(0), px(rw))}
          y={Math.min(py(0), py(rl))}
          width={Math.abs(px(rw) - px(0))}
          height={Math.abs(py(rl) - py(0))}
          className={roomFits ? 'scheme__room' : 'scheme__room scheme__room--overflow'}
        />
      ) : null
    body = (
      <>{roomRect}
      <g transform={`translate(${placement.offsetX * scale} ${-placement.offsetY * scale})`}>
        {rects.map(([x, y, w2, h2], i) => (
          <g key={i}>
            <rect x={mp === -1 ? P(x + w2) : P(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__fill" />
            <rect x={mp === -1 ? P(x + w2) : P(x)} y={py(y + h2)} width={w2 * scale} height={h2 * scale} className="scheme__outline" fill="none" />
          </g>
        ))}
        {renderEdges(rects)}
        {rail(
          [
            [0, 0],
            [lr, 0],
          ],
          s.railingLower,
          'right',
        )}
        {rail(
          [
            [0, wBig],
            [lr, wBig],
          ],
          s.railingLower,
          'left',
        )}
        {rail(
          [
            [lr, 0],
            [lr + lw, 0],
            [lr + lw, gap + wBig],
          ],
          s.railingLanding,
          'right',
        )}
        {rail(
          [
            [lr, wBig],
            [lr, gap],
            [ur, gap],
          ],
          s.railingLanding,
          'left',
        )}
        {rail(
          [
            [lr + lw, gap + wBig],
            [ur, gap + wBig],
            [0, gap + wBig],
          ],
          s.railingUpper,
          'right',
        )}
        {rail(
          [
            [ur, gap],
            [0, gap],
          ],
          s.railingUpper,
          'left',
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
        {/* Свободное пространство перед первой ступенью (EDR-0023): зона перед
            входом (нижним маршем, x=0), рисуется слева. */}
        {ap > 0 && (
          <>
            <rect
              x={Math.min(P(0), P(-ap))}
              y={py(gap + wBig)}
              width={Math.abs(P(0) - P(-ap))}
              height={wBig * scale}
              className="scheme__approach"
            />
            {arrowLine(P(0), py(gap + wBig / 2), P(-ap), py(gap + wBig / 2), 'pln-arr')}
            {dim('Свободное место ' + fmt(ap), (P(0) + P(-ap)) / 2, py(gap + wBig / 2), 'middle', 0, 1)}
          </>
        )}
      </g>
      </>
    )
  } else {
    // spiral
    const r = s.outerRadius ?? f.Width + 400
    const ap = effectiveApproach(s.approachSpace)
    const cr = s.columnRadius ?? 80
    const cx = 0
    const cy = 0
    const steps = Array.from({ length: f.StepCount }, (_, i) => i + 1).map((i) => {
      const a = (i * (s.angularStep ?? 15) * Math.PI) / 180
      return { a }
    })
    const rw = s.roomWidth ?? 0
    const rl = s.roomLength ?? 0
    const roomFits = s.roomFits ?? false
    const roomRect =
      rw > 0 && rl > 0 ? (
        <rect
          x={Math.min(px(0), px(rw))}
          y={Math.min(py(0), py(rl))}
          width={Math.abs(px(rw) - px(0))}
          height={Math.abs(py(rl) - py(0))}
          className={roomFits ? 'scheme__room' : 'scheme__room scheme__room--overflow'}
        />
      ) : null
    body = (
      <>{roomRect}
      <g transform={`translate(${placement.offsetX * scale} ${-placement.offsetY * scale})`}>
        <circle cx={px(cx)} cy={py(cy)} r={r * scale} className="scheme__outline" fill="none" />
        {ANNOTATE && (
          <>
            <circle cx={px(cx)} cy={py(cy)} r={r * scale} className="scheme__edge" style={{ stroke: edgeColor(0) }} fill="none" strokeWidth={3} />
            <text x={px(cx)} y={py(cy) - r * scale - 6} className="scheme__elabel" style={{ fill: edgeColor(0) }} textAnchor="middle" dominantBaseline="middle">К</text>
          </>
        )}
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
        {/* Свободное пространство перед первой ступенью (EDR-0023): зона снаружи
            у входа (внешний радиус, +X). */}
        {ap > 0 && (
          <>
            <rect x={px(r)} y={py(f.Width / 2)} width={ap * scale} height={f.Width * scale} className="scheme__approach" />
            {arrowLine(px(r), py(0), px(r + ap), py(0), 'pln-arr')}
            {dim('Свободное место ' + fmt(ap), (px(r) + px(r + ap)) / 2, py(0) + 16, 'middle', 0, 1)}
            {ANNOTATE && (
              <text x={px(r + ap / 2)} y={py(f.Width / 2)} className="scheme__elabel" style={{ fill: edgeColor(1) }} textAnchor="middle" dominantBaseline="middle">Пд</text>
            )}
          </>
        )}
      </g>
      </>
    )
  }

  // Временная разметка сторон периметра помещения: 4 цветные стены с буквой
  // В/Н/П/Л. Рисуется вне группы марша (не сдвигается).
  const walls =
    ANNOTATE && rw0 > 0 && rl0 > 0 ? (
      <g className="scheme__annot">
        {(['top', 'bottom', 'right', 'left'] as Side[]).map((side) => {
          const w = WALLS[side]
          // Для Г-образной комната в 2D рисуется со сдвигом y=-wBig; сдвигаем и
          // метки периметра, чтобы В/Н совпадали с реальными стенами.
          const wallYShift = kind === 'l_shape' ? -f.Width : 0
          const yB = py(0 + wallYShift)
          const yT = py(rl0 + wallYShift)
          let x1: number, y1: number, x2: number, y2: number, lx: number, ly: number
          if (side === 'top') {
            x1 = px(0); y1 = yT; x2 = px(rw0); y2 = yT; lx = (px(0) + px(rw0)) / 2; ly = yT - 8
          } else if (side === 'bottom') {
            x1 = px(0); y1 = yB; x2 = px(rw0); y2 = yB; lx = (px(0) + px(rw0)) / 2; ly = yB + 16
          } else if (side === 'right') {
            x1 = px(rw0); y1 = yB; x2 = px(rw0); y2 = yT; lx = px(rw0) + 10; ly = (yB + yT) / 2
          } else {
            x1 = px(0); y1 = yB; x2 = px(0); y2 = yT; lx = px(0) - 10; ly = (yB + yT) / 2
          }
          return (
            <g key={side}>
              <line x1={x1} y1={y1} x2={x2} y2={y2} className="scheme__wall" style={{ stroke: w.color }} />
              <text
                x={lx}
                y={ly}
                className="scheme__wlabel"
                style={{ fill: w.color }}
                textAnchor="middle"
                dominantBaseline="middle"
              >
                {w.key}
              </text>
            </g>
          )
        })}
      </g>
    ) : null

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
        {walls}
        {body}
      </svg>
      <p className="scheme__caption">План: вид сверху, размеры в мм.</p>
      {roomOverflow && (
        <p className="scheme__warncaption" style={{ color: '#c0392b', fontWeight: 600, margin: '4px 0 0' }}>
          Не вписывается в помещение {fmt(rw0)} × {fmt(rl0)}
        </p>
      )}
      {(
        <p className="scheme__caption" style={{ color: '#495057', margin: '4px 0 0' }}>
          Нужно помещение ≥ {fmt(cw)} × {fmt(ch)} мм (вкл. свободное пространство перед первой ступенью)
        </p>
      )}
    </div>
  )
}