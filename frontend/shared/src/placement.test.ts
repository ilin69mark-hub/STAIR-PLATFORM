import { describe, expect, it } from 'vitest'
import { computePlacement, type BBox2 } from './placement'

// Габарит лестницы (мм): по X — длина марша + подход, по Y — ширина.
const bb: BBox2 = { minX: 0, minY: 0, maxX: 3000, maxY: 1000 }
const rw = 4000
const rl = 3000

// mirror — коэффициент зеркала потребителя: 3D/не-зеркальный = 1;
// 2D-план для L/U при левом повороте = -1. worldX = mirror*x + offsetX;
// worldY = y + offsetY (по Y зеркала нет).
function hitsWall(
  flight: 'straight' | 'l_shape' | 'u_shape' | 'spiral',
  dir: 'left' | 'right' | undefined,
  kind: '1Л' | '1П' | '1Н' | '1В',
  mirror: 1 | -1 = 1,
) {
  const p = computePlacement(flight, dir, rw, rl, bb, mirror)
  const loX = mirror === 1 ? bb.minX + p.offsetX : -bb.maxX + p.offsetX
  const hiX = mirror === 1 ? bb.maxX + p.offsetX : -bb.minX + p.offsetX
  const mnY = bb.minY + p.offsetY
  const mxY = bb.maxY + p.offsetY
  if (kind === '1Л') expect(loX).toBe(0)
  if (kind === '1П') expect(hiX).toBe(rw)
  if (kind === '1Н') expect(mnY).toBe(0)
  if (kind === '1В') expect(mxY).toBe(rl)
}

// Как 2D-план вызывает placement для типа/направления.
const mirrorOf = (flight: string, dir: 'left' | 'right' | undefined): 1 | -1 =>
  flight === 'l_shape' || flight === 'u_shape' ? (dir === 'left' ? -1 : 1) : 1

describe('computePlacement', () => {
  it('без размеров помещения — без сдвига', () => {
    expect(computePlacement('straight', 'right', 0, 0, bb)).toEqual({ offsetX: 0, offsetY: 0 })
    expect(computePlacement('l_shape', 'left', 4000, 0, bb, -1)).toEqual({ offsetX: 0, offsetY: 0 })
  })

  it('прямой: 1Н->Н, 1Л->Л, фикс (direction игнорируется)', () => {
    for (const dir of ['right', 'left', undefined] as const) {
      const p = computePlacement('straight', dir, rw, rl, bb)
      expect(p.offsetX).toBe(0 - bb.minX)
      expect(p.offsetY).toBe(0 - bb.minY) // 1Н→Н (ближняя стена), направление игнорируется
      hitsWall('straight', dir, '1Н')
      hitsWall('straight', dir, '1Л')
    }
  })

  it('L right: 1П->П, 1Н->Н (низ-право)', () => {
    const p = computePlacement('l_shape', 'right', rw, rl, bb)
    expect(p.offsetX).toBe(rw - bb.maxX)
    expect(p.offsetY).toBe(0) // комната в 2D сдвинута, лестница уже у стены Н
    hitsWall('l_shape', 'right', '1П')
    hitsWall('l_shape', 'right', '1Н')
  })

  it('L left (2D mirror=-1): 1Л->Л, 1Н->Н (низ-лево)', () => {
    const p = computePlacement('l_shape', 'left', rw, rl, bb, -1)
    expect(p.offsetX).toBe(bb.maxX)
    expect(p.offsetY).toBe(0)
    hitsWall('l_shape', 'left', '1Л', -1)
    hitsWall('l_shape', 'left', '1Н', -1)
  })

  it('П right: только 1В->В (лестница у левой стенки, угол ВЛ)', () => {
    const p = computePlacement('u_shape', 'right', rw, rl, bb)
    expect(p.offsetX).toBe(0) // X не фиксируется
    expect(p.offsetY).toBe(rl - bb.maxY)
    hitsWall('u_shape', 'right', '1В')
  })

  it('П left (2D mirror=-1): 1В+1Л (угол ВЛ)', () => {
    const p = computePlacement('u_shape', 'left', rw, rl, bb, -1)
    expect(p.offsetX).toBe(bb.maxX)
    expect(p.offsetY).toBe(rl - bb.maxY)
    hitsWall('u_shape', 'left', '1В', -1)
    hitsWall('u_shape', 'left', '1Л', -1)
  })

  it('спираль: 1П->П, 1Н->Н, фикс (direction игнорируется)', () => {
    for (const dir of ['right', 'left', undefined] as const) {
      const p = computePlacement('spiral', dir, rw, rl, bb)
      expect(p.offsetX).toBe(rw - bb.maxX)
      expect(p.offsetY).toBe(0 - bb.minY)
      hitsWall('spiral', dir, '1П')
      hitsWall('spiral', dir, '1Н')
    }
  })

  it('результат оставляет лестницу внутри периметра (с учётом mirror)', () => {
    for (const flight of ['straight', 'l_shape', 'u_shape', 'spiral'] as const) {
      for (const dir of ['left', 'right', undefined] as const) {
        const m = mirrorOf(flight, dir)
        const p = computePlacement(flight, dir, rw, rl, bb, m)
        const loX = m === 1 ? bb.minX + p.offsetX : -bb.maxX + p.offsetX
        const hiX = m === 1 ? bb.maxX + p.offsetX : -bb.minX + p.offsetX
        const mnY = bb.minY + p.offsetY
        const mxY = bb.maxY + p.offsetY
        expect(loX).toBeGreaterThanOrEqual(0)
        expect(hiX).toBeLessThanOrEqual(rw)
        expect(mnY).toBeGreaterThanOrEqual(0)
        expect(mxY).toBeLessThanOrEqual(rl)
      }
    }
  })
})
