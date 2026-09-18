// layout.ts — позиции вспомогательных элементов 3D-вьювера (GeometryViewer):
// фиолетовая линия верха марша, полупрозрачная зона подхода (EDR-0023),
// стены периметра (виджет В/Н/П/Л) и плита «выхода на 2-й этаж».
// Чистые функции от bounding box отражённого меша в локальных координатах;
// отделены от three.js, чтобы покрыть тестами на реальных данных маршей
// всех типов (straight, l_shape, u_shape, spiral).

export interface Box3Like {
  min: { x: number; y: number; z: number }
  max: { x: number; y: number; z: number }
}

// Габарит бокса (three.js BoxGeometry): size = [w,h,d] по осям X,Y,Z и
// позиция центра. Позиция в локальных координатах stairGroup.
export interface BoxSpec {
  size: [number, number, number]
  pos: [number, number, number]
}

// X линии верха марша (в группе марша). Для straight меш зеркалится по X
// (mirrorX): верх марша оказывается у box.min.x. Для u_shape верх — конец
// верхнего марша, возвращающегося вдоль −X (правый поворот: box.min.x; левый
// зеркалит компоновку — box.max.x). Для l_shape и spiral верх считается по
// box.max.x: для l_shape это кромка площадки (приближение верха), для spiral —
// радиальное приближение. direction учитывается только для u_shape.
export function stairTopLineX(box: Box3Like, flight: string, direction?: 'left' | 'right'): number {
  if (flight === 'straight') return box.min.x
  if (flight === 'u_shape') return direction === 'left' ? box.max.x : box.min.x
  return box.max.x
}

// Грань входа в марш — отсюда начинается свободное пространство перед первой
// ступенью (зона подхода). Для straight первый шаг на +X, и лицевая панель
// марша отстоит от box.max.x на StepThickness (свес первой проступи,
// builder.go: первая проступь [−stepThickness, b]). Для прочих типов позиция
// не меняется (вход у box.max.x, как сейчас).
export function entranceFaceX(box: Box3Like, flight: string, stepThickness: number): number {
  return flight === 'straight' ? box.max.x - stepThickness : box.max.x
}

// Центр зоны подхода по X (плоскость ложится на пол: rotation.x = −π/2,
// поэтому PlaneGeometry конвертится вдоль X). Ширина зоны = approachSpace,
// левый край совпадает с гранью входа.
export function approachZoneCenterX(
  box: Box3Like,
  flight: string,
  stepThickness: number,
  approachSpace: number,
): number {
  return entranceFaceX(box, flight, stepThickness) + approachSpace / 2
}

// Стороны периметра лестницы (виджет «стены»): top/В — дальняя (Z max),
// bottom/Н — ближняя (Z min), right/П — правый торец (X max),
// left/Л — левый торец (X min). Совпадает с разметкой scheme-annot.ts.
export type WallSide = 'top' | 'bottom' | 'right' | 'left'

// Высота «блока второго этажа» (плиты выхода), мм: статична, не зависит от
// состояния чекбокса. Передаётся в wallBox, чтобы выходная стена доводилась
// ровно до низа блока (не «прорезала» его) в обоих состояниях тумблера.
export const EXIT_BLOCK_H = 200

// Сторона периметра, куда уходит выход на 2-й этаж (направление плиты в
// exitSlabBox). На этой стене высота уменьшается на высоту блока:
//   - straight: выход у box.min.x → стена Л (left);
//   - u_shape: правый поворот — верхний марш у box.min.x → Л; левый — П;
//   - l_shape: верхний марш поднимается вдоль +Z → стена В (top);
//   - spiral: торец угловой → П (right).
export function exitWallSide(flight: string, direction?: 'left' | 'right'): WallSide {
  if (flight === 'l_shape') return 'top'
  if (flight === 'spiral') return 'right'
  if (flight === 'u_shape') return direction === 'left' ? 'right' : 'left'
  return 'left'
}

// Входные данные для построения периметра стен.
export interface WallBoundsInput {
  // Габарит лестницы (stair.geo.boundingBox): даёт высоту (Y) и гарантирует,
  // что меш существует. Без меша стены не строятся.
  stairBox: Box3Like
  // Габарит помещения (room.geo.boundingBox, мировые координаты): даёт точный
  // периметр. Если меш помещения есть — приоритет над roomWidth/roomLength.
  roomBounds?: Box3Like | null
  roomWidth?: number
  roomLength?: number
  // Высота марша из ввода пользователя (поле «Высота», мм): верхняя кромка
  // стен. При 0/отсутствии — геометрический верх меша (stairBox.max.y).
  heightMM?: number
}

// Габарит стен по периметру ПОМЕЩЕНИЯ: стены строятся по размерам комнаты
// (room_mesh в мировых координатах), а не по габаритам лестницы. Верх стен —
// пользовательская высота марша (heightMM) с фолбэком на верх меша. Null, если
// нет ни меша лестницы, ни меша помещения (и габариты комнаты не заданы) —
// в этом случае виджет «стены» залочен.
export function wallBoundsOf({
  stairBox,
  roomBounds,
  roomWidth,
  roomLength,
  heightMM,
}: WallBoundsInput): Box3Like | null {
  if (!stairBox) return null
  const rb = roomBounds || null
  const rw = roomWidth ?? 0
  const rl = roomLength ?? 0
  if (!rb && !(rw > 0 && rl > 0)) return null
  return {
    min: { x: rb ? rb.min.x : 0, y: stairBox.min.y, z: rb ? rb.min.z : 0 },
    max: {
      x: rb ? rb.max.x : rw,
      y: heightMM && heightMM > 0 ? heightMM : stairBox.max.y,
      z: rb ? rb.max.z : rl,
    },
  }
}

// X-центр вертикальной стены на стороне П/Л (стена вдоль ширины, ось Z):
// правый торец — снаружи box.max.x, левый — снаружи box.min.x.
export function wallPlaneX(box: Box3Like, side: WallSide, thickness: number): number {
  return side === 'right' ? box.max.x + thickness / 2 : box.min.x - thickness / 2
}

// Z-центр вертикальной стены на стороне В/Н (стена вдоль подъёма, ось X):
// дальняя стена — снаружи box.max.z, ближняя — снаружи box.min.z.
export function wallPlaneZ(box: Box3Like, side: WallSide, thickness: number): number {
  return side === 'top' ? box.max.z + thickness / 2 : box.min.z - thickness / 2
}

// Длина стены В/Н вдоль подъёма (ось X) — от box.min.x до box.max.x.
export function wallSpanX(box: Box3Like): { x0: number; x1: number } {
  return { x0: box.min.x, x1: box.max.x }
}

// Длина стены П/Л вдоль ширины (ось Z) — от box.min.z до box.max.z.
export function wallSpanZ(box: Box3Like): { z0: number; z1: number } {
  return { z0: box.min.z, z1: box.max.z }
}

// Прямоугольный бокс вертикальной стены на стороне `side`. Стены строятся по
// ПЕРИМЕТРУ ПОМЕЩЕНИЯ (bounds — габариты комнаты по X/Z, высота по Y), а не по
// габаритам лестницы: В — дальняя стена позади марша (Z max), Н — ближняя,
// перед маршем внутри помещения (Z min), П — правая (X max), Л — левая (X min).
// Высота стены — из Y-диапазона bounds (верх = заданная пользователем высота
// марша). Выходная стена (secondFloor.side, куда уходит выход на 2-й этаж)
// доводится ровно до низа блока выхода: её высота на secondFloor.height меньше,
// блок «ложится» на стену, поверхности на торце заподлицо. Расчёт не зависит от
// того, включён ли тумблер выхода (плита только меняет .visible).
export function wallBox(
  box: Box3Like,
  side: WallSide,
  thickness: number,
  secondFloor?: { side: WallSide; height: number },
): BoxSpec {
  const atExit = secondFloor != null && secondFloor.side === side
  const topY = atExit ? box.max.y - secondFloor!.height : box.max.y
  const yH = Math.max(topY - box.min.y, 1)
  const midY = (box.min.y + topY) / 2
  if (side === 'top' || side === 'bottom') {
    const xs = wallSpanX(box)
    return {
      size: [xs.x1 - xs.x0, yH, thickness],
      pos: [(xs.x0 + xs.x1) / 2, midY, wallPlaneZ(box, side, thickness)],
    }
  }
  const zs = wallSpanZ(box)
  return {
    size: [thickness, yH, zs.z1 - zs.z0],
    pos: [wallPlaneX(box, side, thickness), midY, (zs.z0 + zs.z1) / 2],
  }
}

// Плита «выхода на 2-й этаж»: плоский бокс, верх на уровне верха марша,
// примыкает к верхней ступени и уходит наружу на depth. Направление/ось
// зависят от типа марша:
//   - straight: верхний торец у box.min.x (после mirrorX) → глубина вдоль −X,
//     ширина по Z (марш);
//   - u_shape: правый поворот — верхний марш возвращается вдоль −X (мин.
//     кромка); левый зеркалит компоновку (макс. кромка) → глубина вдоль X;
//   - l_shape: верхний марш поднимается вдоль ширины (+Y в плане = +Z в
//     three.js) → глубина вдоль +Z от box.max.z, ширина по X (bbox-приближение);
//   - spiral: торец угловой, bbox симметричный → приближение вдоль +X.
export function exitSlabBox(
  box: Box3Like,
  flight: string,
  direction: 'left' | 'right' | undefined,
  depth: number,
  thickness: number,
): BoxSpec {
  const topY = box.max.y
  if (flight === 'l_shape') {
    const xs = wallSpanX(box)
    return {
      size: [xs.x1 - xs.x0, thickness, depth],
      pos: [(xs.x0 + xs.x1) / 2, topY - thickness / 2, box.max.z + depth / 2],
    }
  }
  const zs = wallSpanZ(box)
  const zSpan = zs.z1 - zs.z0
  const midZ = (zs.z0 + zs.z1) / 2
  const atMinX = flight === 'straight' || (flight === 'u_shape' && direction !== 'left')
  const inner = atMinX ? box.min.x : box.max.x
  const dir = atMinX ? -1 : 1
  return {
    size: [depth, thickness, zSpan],
    pos: [inner + (dir * depth) / 2, topY - thickness / 2, midZ],
  }
}