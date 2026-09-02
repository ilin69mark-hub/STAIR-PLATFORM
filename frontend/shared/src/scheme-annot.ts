// Временная цвето-буквенная разметка периметра помещения и рёбер марша.
//
// Назначение (см. план): дать однозначный словарь для общения про «кто куда
// смотрит». Стороны периметра и рёбра марша помечаются цветом + буквой.
// Цвета НЕЗАВИСИМЫ от направления (пёстрая отладочная разметка), стороны и
// рёбра — в разных буквенных пространствах.
//
// Соглашение по буквам: русские В/Н/П/Л (Верх/Низ/Право/Лево) по позиции.
//   - Стороны периметра: В, Н, П, Л (4 шт, фиксированная палитра).
//   - Рёбра марша: <номер марша><буква>, напр. 1В, 1Н, 1П, 1Л, 2В… (цвета из
//     отдельной палитры, не привязаны к стенам).
//
// Перед релизом всю разметку убираем одним переключателем ANNOTATE = false.

export type Side = 'top' | 'bottom' | 'right' | 'left'

// Единый выключатель всей разметки (для лёгкого удаления перед релизом).
export const ANNOTATE = false

// Стороны периметра помещения: фиксированная палитра (цвет → буква).
export const WALLS: Record<Side, { key: string; color: string }> = {
  top: { key: 'В', color: '#e03131' }, // верх / дальняя стена (+Y)
  bottom: { key: 'Н', color: '#1971c2' }, // низ / ближняя стена (-Y)
  right: { key: 'П', color: '#2f9e44' }, // правая стена (+X)
  left: { key: 'Л', color: '#e8590c' }, // левая стена (-X)
}

// Яркая палитра для рёбер марша (независимые цвета, циклически по индексу).
export const EDGE_PALETTE = [
  '#e64980',
  '#f59f00',
  '#0c8599',
  '#7048e8',
  '#d6336c',
  '#82c91e',
  '#1098ad',
  '#f783ac',
  '#5c7cfa',
  '#fab005',
  '#12b886',
  '#e8590c',
]

export function edgeColor(i: number): string {
  const n = EDGE_PALETTE.length
  return EDGE_PALETTE[(((i % n) + n) % n)]
}

export type Rect4 = [number, number, number, number]
export type EdgeSeg = { x1: number; y1: number; x2: number; y2: number; side: Side }

// Четыре ребра осесимметричного прямоугольника [x, y, w, h] в мировых координатах
// (x вправо, y вверх). Порядок: верх, низ, право, лево.
export function rectEdges(rect: Rect4): EdgeSeg[] {
  const [x, y, w, h] = rect
  return [
    { x1: x, y1: y + h, x2: x + w, y2: y + h, side: 'top' },
    { x1: x, y1: y, x2: x + w, y2: y, side: 'bottom' },
    { x1: x + w, y1: y, x2: x + w, y2: y + h, side: 'right' },
    { x1: x, y1: y, x2: x, y2: y + h, side: 'left' },
  ]
}

// Буквенное обозначение ребра марша: <номер марша, с 1><буква стороны>.
export function edgeLabel(flightIdx: number, side: Side): string {
  return `${flightIdx + 1}${WALLS[side].key}`
}
