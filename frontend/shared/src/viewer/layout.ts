// layout.ts — позиции вспомогательных элементов 3D-вьювера (GeometryViewer):
// фиолетовая линия верха марша и полупрозрачная зона подхода (EDR-0023).
// Чистые функции от bounding box отражённого меша в локальных координатах;
// отделены от three.js, чтобы покрыть тестами на реальных данных прямого марша.

export interface Box3Like {
  min: { x: number; y: number; z: number }
  max: { x: number; y: number; z: number }
}

// X линии верха марша (в группе марша). Для straight меш зеркалится по X
// (mirrorX): верх марша оказывается у box.min.x, для прочих типов — box.max.x.
export function stairTopLineX(box: Box3Like, flight: string): number {
  return flight === 'straight' ? box.min.x : box.max.x
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