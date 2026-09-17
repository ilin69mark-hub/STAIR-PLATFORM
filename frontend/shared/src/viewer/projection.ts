// projection.ts — перенос координат геометрии (ADR-0008: X — подъём, Y —
// ширина, Z — высота) в координаты three.js (Y-up): X→x, Z→y, Y→z.
// Чистая функция без three.js, чтобы ось высоты была покрыта тестом.

export interface Vertex3 {
  X: number
  Y: number
  Z: number
}

export function toThreePositions(vertices: Vertex3[] | null | undefined): number[] {
  if (!vertices || !Array.isArray(vertices)) return []
  const out = new Array<number>(vertices.length * 3)
  for (let i = 0; i < vertices.length; i++) {
    const v = vertices[i]
    out[i * 3] = v.X
    out[i * 3 + 1] = v.Z
    out[i * 3 + 2] = v.Y
  }
  return out
}