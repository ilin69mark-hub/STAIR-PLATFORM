// annotate.ts — расстановка подписей и размеров на схемах без наложений.
// Детерминированный greedy: лейбл пробует позиции вдоль направления
// выноски, отступая на шаг, пока не найдёт свободное место (не пересекает
// контур чертежа и ранее расставленные подписи). Чистые функции без DOM,
// поэтому размеры текста оцениваются арифметически (в jsdom нет getBBox).

export interface Rect {
  x: number
  y: number
  w: number
  h: number
}

export function overlap(a: Rect, b: Rect): boolean {
  return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y
}

// Приблизительная ширина текста: ≈0.62·fontSize на символ + запас.
export function textWidth(s: string, fontSize: number): number {
  return s.length * fontSize * 0.62 + fontSize
}

export function rectAt(cx: number, cy: number, w: number, h: number): Rect {
  return { x: cx - w / 2, y: cy - h / 2, w, h }
}

export interface PlaceLabelOptions {
  ax: number
  ay: number
  w: number
  h: number
  dx: number
  dy: number
  obstacles?: Rect[]
  placed?: Rect[]
  step?: number
  maxSteps?: number
}

export interface Point {
  x: number
  y: number
}

// placeLabel возвращает центр подписи (размер w×h), не пересекающий контур
// чертежа и уже размещённые подписи. Сначала пробует вдоль (dx,dy) с шагом
// step, затем зеркально против (dx,dy). Фолбэк — последняя позиция.
export function placeLabel(o: PlaceLabelOptions): Point {
  const step = o.step ?? 18
  const maxSteps = o.maxSteps ?? 10
  const blocked = (c: Point): boolean => {
    const r = rectAt(c.x, c.y, o.w, o.h)
    if (o.obstacles?.some((b) => overlap(r, b))) return true
    if (o.placed?.some((p) => overlap(r, p))) return true
    return false
  }
  for (let s = 0; s <= maxSteps; s++) {
    const c = { x: o.ax + o.dx * step * s, y: o.ay + o.dy * step * s }
    if (!blocked(c)) return c
  }
  for (let s = 1; s <= maxSteps; s++) {
    const c = { x: o.ax - o.dx * step * s, y: o.ay - o.dy * step * s }
    if (!blocked(c)) return c
  }
  return { x: o.ax + o.dx * step * maxSteps, y: o.ay + o.dy * step * maxSteps }
}

// bboxRect — ограничивающий прямоугольник наборов точек (контур чертежа).
export function bboxRect(pts: Array<{ x: number; y: number }>): Rect {
  let minX = Infinity
  let minY = Infinity
  let maxX = -Infinity
  let maxY = -Infinity
  for (const p of pts) {
    if (p.x < minX) minX = p.x
    if (p.y < minY) minY = p.y
    if (p.x > maxX) maxX = p.x
    if (p.y > maxY) maxY = p.y
  }
  return { x: minX, y: minY, w: maxX - minX, h: maxY - minY }
}
