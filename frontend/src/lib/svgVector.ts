/**
 * Векторный перенос SVG в jsPDF (S-177).
 *
 * Зачем. КП — то, что уходит клиенту, и раньше оно вставляло 2D-чертёж
 * растром: PNG из SVG через canvas. Растр мыльный при печати, его нельзя
 * измерить и нельзя отредактировать.
 *
 * Схемы лестницы (StairProfile / StairPlan) рисуются примитивами `<line>`,
 * `<path>` (только M/L), `<rect>` и `<text>` — все переносятся в векторные
 * примитивы PDF без потери качества. 3D-часть КП остаётся растром: это
 * снимок рендера, векторизировать его дороже, чем он того стоит.
 *
 * CSS не разбирается: оформление задано классами, и цвета/толщины берутся
 * из таблицы. Неизвестный класс рисуется нейтральным серым, а не
 * пропускается — лучше лишняя линия, чем пропавший элемент чертежа.
 */

const STYLE: Record<string, { color: [number, number, number]; width: number }> = {
  scheme__line: { color: [30, 30, 30], width: 0.25 },
  scheme__outline: { color: [20, 20, 20], width: 0.5 },
  scheme__tread: { color: [90, 90, 90], width: 0.3 },
  scheme__riser: { color: [120, 120, 120], width: 0.25 },
  scheme__dimline: { color: [70, 110, 180], width: 0.18 },
  scheme__post: { color: [70, 110, 180], width: 0.2 },
  scheme__rail: { color: [150, 120, 70], width: 0.25 },
  scheme__label: { color: [60, 60, 60], width: 0.15 },
}

const FALLBACK = { color: [90, 90, 90] as [number, number, number], width: 0.2 }

/**
 * Минимальная часть jsPDF, которой достаточно вектору.
 *
 * Только line/rect/text — без lines(). У jsPDF разное соглашение по Y у
 * разных примитивов (line переворачивает ось, rect и text — нет, а lines()
 * вдобавок требует явный scale и падает без него). Чтобы не держать в голове
 * две системы координат, весь контур рисуется через line(): это один путь
 * и одна проверка.
 */
export interface VectorDoc {
  setLineWidth(w: number): void
  setDrawColor(r: number, g: number, b: number): void
  setTextColor(r: number, g: number, b: number): void
  setFontSize(size: number): void
  line(x1: number, y1: number, x2: number, y2: number): void
  rect(x: number, y: number, w: number, h: number): void
  text(text: string, x: number, y: number): void
}

function num(v: string | null | undefined): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

/**
 * Разбирает путь «M x y L x y L …» в сегменты относительно текущей точки.
 * Поддерживаются только M/L — этого достаточно для схем лестницы.
 *
 * Возвращает начальную точку и список относительных сегментов
 * [dx, dy, 0, 0] — такой формат принимает doc.lines.
 */
export function parsePolylinePath(d: string): { start: [number, number]; segs: number[][] } {
  let cx = 0
  let cy = 0
  let start: [number, number] = [0, 0]
  let seen = false
  const segs: number[][] = []

  for (const tok of d.match(/[MLml][^MLzml]*/g) ?? []) {
    const cmd = tok[0]
    const nums = (tok.slice(1).match(/-?\d+(?:\.\d+)?/g) ?? []).map(Number)
    const rel = cmd === cmd.toLowerCase()

    if (cmd === 'M' || cmd === 'm') {
      if (nums.length < 2) continue
      cx = rel && seen ? cx + nums[0] : nums[0]
      cy = rel && seen ? cy + nums[1] : nums[1]
      if (!seen) start = [cx, cy]
      seen = true
      continue
    }
    // L: пары координат от текущей точки.
    for (let i = 0; i + 1 < nums.length; i += 2) {
      const nx = rel ? cx + nums[i] : nums[i]
      const ny = rel ? cy + nums[i + 1] : nums[i + 1]
      segs.push([nx - cx, ny - cy, 0, 0])
      cx = nx
      cy = ny
    }
  }
  return { start, segs }
}

/** Атрибут points="x,y x,y …" → пары координат. */
export function parsePoints(points: string): number[] {
  return (points || '').trim().split(/[\s,]+/).filter(Boolean).map(Number)
}

/**
 * Рисует SVG в PDF векторными примитивами.
 *
 * x, y — левый верхний угол области вставки на странице (мм),
 * widthMM — ширина области (мм). Высота берётся по пропорции viewBox.
 *
 * Ось Y в SVG смотрит вниз, в PDF — вверх, поэтому Y переворачивается.
 * Возвращает высоту вставки в мм.
 */
export function drawSvgVector(
  doc: VectorDoc,
  svg: SVGElement,
  x: number,
  y: number,
  widthMM: number,
): number {
  const vb = (svg.getAttribute('viewBox') || '').trim().split(/[\s,]+/).map(Number)
  const vbW = vb.length === 4 && vb[2] > 0 ? vb[2] : num(svg.getAttribute('width')) || 1
  const vbH = vb.length === 4 && vb[3] > 0 ? vb[3] : num(svg.getAttribute('height')) || 1
  const s = widthMM / vbW
  const heightMM = vbH * s

  // jsPDF.line() сам переворачивает Y (проверено на распакованном
  // content stream: переданные 50 мм уходят в файл как 297 − 50). Поэтому
  // координата от верха передаётся КАК ЕСТЬ — свой переворот здесь дал бы
  // двойной переворот и чертёж вверх ногами.
  const px = (svgX: number) => x + svgX * s
  const py = (svgY: number) => y + svgY * s

  /** Один отрезок по экранным координатам SVG. */
  const seg = (x1: number, y1: number, x2: number, y2: number) =>
    doc.line(px(x1), py(y1), px(x2), py(y2))

  const nodes = Array.from(svg.querySelectorAll('line, path, rect, polyline, text'))

  for (const el of nodes) {
    const cls = (el.getAttribute('class') || '').trim()
    const st = STYLE[cls] || FALLBACK
    doc.setDrawColor(st.color[0], st.color[1], st.color[2])
    doc.setLineWidth(st.width)

    switch (el.tagName.toLowerCase()) {
      case 'line': {
        doc.line(
          px(num(el.getAttribute('x1'))), py(num(el.getAttribute('y1'))),
          px(num(el.getAttribute('x2'))), py(num(el.getAttribute('y2'))),
        )
        break
      }
      case 'polyline': {
        const pts = parsePoints(el.getAttribute('points') || '')
        for (let i = 0; i + 3 < pts.length; i += 2) seg(pts[i], pts[i + 1], pts[i + 2], pts[i + 3])
        break
      }
      case 'path': {
        const { start, segs } = parsePolylinePath(el.getAttribute('d') || '')
        let cx = start[0]
        let cy = start[1]
        for (const sg of segs) {
          const nx = cx + sg[0]
          const ny = cy + sg[1]
          seg(cx, cy, nx, ny)
          cx = nx
          cy = ny
        }
        break
      }
      case 'rect': {
        doc.rect(
          px(num(el.getAttribute('x'))),
          py(num(el.getAttribute('y'))),
          num(el.getAttribute('width')) * s,
          num(el.getAttribute('height')) * s,
        )
        break
      }
      case 'text': {
        const t = (el.textContent || '').trim()
        if (!t) break
        const raw = num(el.getAttribute('font-size')) || 12
        // viewBox обычно крупнее страницы; pt = SVG-единицы → мм → пункты.
        const pt = Math.max(4, Math.min(14, raw * s * 2.8346))
        doc.setFontSize(pt)
        doc.setTextColor(st.color[0], st.color[1], st.color[2])
        doc.text(t, px(num(el.getAttribute('x'))), py(num(el.getAttribute('y'))))
        break
      }
      default:
        break
    }
  }
  return heightMM
}