// Размещение лестницы у дальней стены/угла помещения (только визуализация).
// Бэкенд Model/Generate не меняется — смещение применяется во фронтенде
// (3D-вьювер и 2D-план), чтобы не затрагивать производственную декомпозицию.
//
// Правило (подтверждено пользователем 2026-08-26, уточнено по буквам меток
// цвето-буквенной разметки): каждая метка 1В/1Н/1Л/1П — ребро марша,
// В/Н/П/Л — стены периметра (В=+Y дальняя, Н=-Y ближняя, П=+X правая, Л=-X левая).
//
// Буквенные правила прижима ребра к стене:
//   straight : 1Н→Н, фикс (direction игнорируется)            -> всегда НЛ
//   l_shape  : налево 1Л→Л, направо 1П→П (Y не фиксируется)   -> ЛН / ПН
//   u_shape  : налево 1В+1Л (угол ВЛ); направо только 1В→В    -> ВЛ / ВЛ
//   spiral   : 1П+1Н, фикс (direction игнорируется)           -> всегда ПН
//
// Координаты потребителя: worldX = mirror * x + offsetX.
// 2D-план зеркалит геометрию L/U через mp и передаёт mirror=mp (±1); 3D-вьювер
// не зеркалит (модель уже сориентирована движком) и передаёт mirror=1.
// Поэтому offsetX считается с учётом mirror, чтобы и там, и там ребро упиралось
// в нужную стену:
//   стена Л (worldX=0):  mirror= 1 → offsetX = -bb.minX;   mirror=-1 → offsetX = +bb.maxX
//   стена П (worldX=rw): mirror= 1 → offsetX = rw-bb.maxX; mirror=-1 → offsetX = rw+bb.minX
//
// Y-сдвиг: l_shape → 0 (в 2D комната рисуется со сдвигом y=-wBig, лестница уже у
// стены Н; в 3D модель движка уже у ближней стены); spiral → Н (anchorMinY);
// straight → Н (anchorMinY, 1Н→Н); u_shape → В (anchorMaxY).
// Комната: roomWidth (по X, марш), roomLength (по Y, ширина).

export interface BBox2 {
  minX: number
  minY: number
  maxX: number
  maxY: number
}

export interface PlacementOffset {
  offsetX: number
  offsetY: number
}

export type StairKind = 'straight' | 'l_shape' | 'u_shape' | 'spiral'
export type TurnDirection = 'left' | 'right'

export function computePlacement(
  flight: StairKind,
  direction: TurnDirection | undefined,
  roomWidth: number,
  roomLength: number,
  bb: BBox2,
  mirror: 1 | -1 = 1,
): PlacementOffset {
  // Без заданных размеров помещения — без привязки (центрируем как раньше).
  if (roomWidth <= 0 || roomLength <= 0) return { offsetX: 0, offsetY: 0 }

  const anchorMaxX = (rw: number): number => rw - bb.maxX
  const anchorMinX = (): number => 0 - bb.minX
  const anchorMaxY = (rl: number): number => rl - bb.maxY
  const anchorMinY = (): number => 0 - bb.minY

  // X-стена по буквам пользователя (mirror влияет только на знак offsetX).
  let xWall: 'Л' | 'П' | 'none'
  if (flight === 'straight') xWall = 'Л'
  else if (flight === 'spiral') xWall = 'П'
  else if (flight === 'l_shape') xWall = direction === 'left' ? 'Л' : 'П'
  else xWall = direction === 'left' ? 'Л' : 'none' // u_shape right: только В

  let offsetX: number
  if (xWall === 'none') offsetX = 0
  else if (xWall === 'Л') offsetX = mirror === 1 ? anchorMinX() : bb.maxX
  else offsetX = mirror === 1 ? anchorMaxX(roomWidth) : roomWidth + bb.minX

  let offsetY: number
  if (flight === 'l_shape') offsetY = 0
  else if (flight === 'spiral') offsetY = anchorMinY()
  else if (flight === 'straight') offsetY = anchorMinY() // 1Н→Н
  else offsetY = anchorMaxY(roomLength) // u_shape: 1В→В

  return { offsetX, offsetY }
}
