import { describe, expect, it } from 'vitest'
import {
  approachZoneCenterX,
  entranceFaceX,
  EXIT_BLOCK_H,
  exitSlabBox,
  exitWallSide,
  stairTopLineX,
  wallBox,
  wallBoundsOf,
  wallPlaneX,
  wallPlaneZ,
  wallSpanX,
  wallSpanZ,
} from './layout'

// Реальные bbox маршей (H=2700, n=15, h=180, b=270, W=900, st=40) в
// локальных координатах three.js (X=подъём, Y=высота, Z=ширина):
//   - straight: geometry.Generate → kernel X∈[960,5050] → после mirrorX в
//     GeometryViewer локальный box X∈[−5050,−960] (frontend layout.test);
//   - l_shape  правый: builder.go → kernel X∈[−40,2520], Y(ширина)∈[0,3430],
//     Z(высота)∈[0,2700] (lshape_test: (−40,0,0)…(2520,3430,2700)) → three.js
//     x∈[−40,2520], y∈[0,2700], z∈[0,3430];
//   - u_shape  правый: kernel X∈[−810,2520], Y∈[0,1800] (ushape_test) →
//     three.js x∈[−810,2520], y∈[0,2700], z∈[0,1800]. Верхний марш возвращается
//     вдоль −X → верх у box.min.x;
//   - u_shape  левый: зеркальная компоновка (upperT без поворота, подъём +X:
//     x∈[860,3330]) → верх у box.max.x;
//   - spiral: колонна в начале координат, |x|,|y|≤R=800 (spiral_test) →
//     three.js x∈[−800,800], y∈[0,2700], z∈[0,800].
const straightBox = {
  min: { x: -5050, y: 0, z: 0 },
  max: { x: -960, y: 2700, z: 900 },
} as const
const lShapeBox = {
  min: { x: -40, y: 0, z: 0 },
  max: { x: 2520, y: 2700, z: 3430 },
} as const
const uShapeRightBox = {
  min: { x: -810, y: 0, z: 0 },
  max: { x: 2520, y: 2700, z: 1800 },
} as const
const uShapeLeftBox = {
  min: { x: 860, y: 0, z: 0 },
  max: { x: 3330, y: 2700, z: 1800 },
} as const
const spiralBox = {
  min: { x: -800, y: 0, z: 0 },
  max: { x: 800, y: 2700, z: 800 },
} as const

describe('stairTopLineX', () => {
  it('прямой марш: линия верха на стороне box.min.x (верх марша после mirrorX)', () => {
    expect(stairTopLineX(straightBox, 'straight')).toBe(-5050)
  })
  it('l_shape: линия верха на кромке площадки box.max.x (приближение верха)', () => {
    expect(stairTopLineX(lShapeBox, 'l_shape')).toBe(2520)
  })
  it('u_shape правый поворот: верхний марш возвращается вдоль −X — верх у box.min.x', () => {
    expect(stairTopLineX(uShapeRightBox, 'u_shape')).toBe(-810)
    expect(stairTopLineX(uShapeRightBox, 'u_shape', 'right')).toBe(-810)
  })
  it('u_shape левый поворот: зеркальная компоновка — верх у box.max.x', () => {
    expect(stairTopLineX(uShapeLeftBox, 'u_shape', 'left')).toBe(3330)
  })
  it('spiral: bbox симметричный, верх условно у box.max.x', () => {
    expect(stairTopLineX(spiralBox, 'spiral')).toBe(800)
  })
})

describe('entranceFaceX', () => {
  it('прямой марш: грань входа на StepThickness ближе к маршу, чем box.max.x', () => {
    expect(entranceFaceX(straightBox, 'straight', 40)).toBe(-1000)
  })
  it('прочие типы: грань входа остаётся на box.max.x (поведение не меняется)', () => {
    expect(entranceFaceX(straightBox, 'l_shape', 40)).toBe(-960)
  })
})

describe('approachZoneCenterX', () => {
  it('прямой марш: зона примыкает к входу и ровно упирается в стену комнаты (roomWidth = Run+approach)', () => {
    const st = 40
    const ap = 1000
    const offsetX = -straightBox.min.x // computePlacement: прижим к стене П
    const start = entranceFaceX(straightBox, 'straight', st) + offsetX
    const end = start + ap
    expect(start).toBe(4050)
    expect(end).toBe(5050)
    expect(approachZoneCenterX(straightBox, 'straight', st, ap)).toBe(-500)
    expect(approachZoneCenterX(straightBox, 'straight', st, ap) + offsetX).toBe(4550)
  })
})

describe('exitWallSide', () => {
  it('straight — выход у box.min.x, стена Л', () => {
    expect(exitWallSide('straight', undefined)).toBe('left')
  })
  it('l_shape — верхний марш поднимается вдоль ширины, выход вдоль +Z → стена В', () => {
    expect(exitWallSide('l_shape', 'right')).toBe('top')
    expect(exitWallSide('l_shape', 'left')).toBe('top')
  })
  it('u_shape правый — верхний марш у box.min.x → стена Л; левый → стена П', () => {
    expect(exitWallSide('u_shape', 'right')).toBe('left')
    expect(exitWallSide('u_shape', 'left')).toBe('right')
  })
  it('spiral — торец угловой → стена П', () => {
    expect(exitWallSide('spiral', 'cw')).toBe('right')
  })
  it('u_shape без направления — дефолт правый поворот → стена Л', () => {
    expect(exitWallSide('u_shape', undefined)).toBe('left')
  })
  it('спираль без/с другим направлением — всегда стена П', () => {
    expect(exitWallSide('spiral', undefined)).toBe('right')
    expect(exitWallSide('spiral', 'left')).toBe('right')
    expect(exitWallSide('spiral', 'right')).toBe('right')
  })
})

describe('exitSlabBox', () => {
  const d = 1200 // depth
  const t = 200 // толщина
  it('straight: плита вдоль −X от box.min.x, ширина по Z = марш', () => {
    expect(exitSlabBox(straightBox, 'straight', undefined, d, t)).toEqual({
      size: [1200, 200, 900],
      pos: [-5650, 2600, 450],
    })
  })
  it('l_shape: верхний марш поднимается вдоль ширины (+Z) — плита за box.max.z, ширина по X = bbox', () => {
    expect(exitSlabBox(lShapeBox, 'l_shape', 'right', d, t)).toEqual({
      size: [2560, 200, 1200],
      pos: [1240, 2600, 4030],
    })
    expect(exitSlabBox(lShapeBox, 'l_shape', 'left', d, t)).toEqual({
      size: [2560, 200, 1200],
      pos: [1240, 2600, 4030],
    })
  })
  it('u_shape правый поворот: плита вдоль −X от box.min.x', () => {
    expect(exitSlabBox(uShapeRightBox, 'u_shape', 'right', d, t)).toEqual({
      size: [1200, 200, 1800],
      pos: [-1410, 2600, 900],
    })
  })
  it('u_shape без направления — как правый поворот (плита вдоль −X)', () => {
    expect(exitSlabBox(uShapeRightBox, 'u_shape', undefined, d, t)).toEqual({
      size: [1200, 200, 1800],
      pos: [-1410, 2600, 900],
    })
  })
  it('u_shape левый поворот: плита вдоль +X от box.max.x', () => {
    expect(exitSlabBox(uShapeLeftBox, 'u_shape', 'left', d, t)).toEqual({
      size: [1200, 200, 1800],
      pos: [3930, 2600, 900],
    })
  })
  it('spiral: плита вдоль +X от box.max.x независимо от направления', () => {
    const a = exitSlabBox(spiralBox, 'spiral', undefined, d, t)
    expect(a).toEqual({ size: [1200, 200, 800], pos: [1400, 2600, 400] })
    expect(exitSlabBox(spiralBox, 'spiral', 'cw', d, t)).toEqual(a)
    expect(exitSlabBox(spiralBox, 'spiral', 'ccw', d, t)).toEqual(a)
  })
})

describe('wallBox (по периметру помещения)', () => {
  const t = 60
  // Габариты комнаты: 3000 мм по X (длина забега + подход), 4200 мм по Z
  // (ширина марша), высота 2700 = пользовательский ввод.
  const room = { min: { x: 0, y: 0, z: 0 }, max: { x: 3000, y: 2700, z: 4200 } }

  it('все стороны по периметру комнаты', () => {
    // В (top): far wall — z = max.z + half thickness
    expect(wallBox(room, 'top', t)).toEqual({ size: [3000, 2700, 60], pos: [1500, 1350, 4230] })
    // Н (bottom): near wall — z = min.z − half thickness
    expect(wallBox(room, 'bottom', t)).toEqual({ size: [3000, 2700, 60], pos: [1500, 1350, -30] })
    // П (right): x = max.x + half thickness
    expect(wallBox(room, 'right', t)).toEqual({ size: [60, 2700, 4200], pos: [3030, 1350, 2100] })
    // Л (left): x = min.x − half thickness
    expect(wallBox(room, 'left', t)).toEqual({ size: [60, 2700, 4200], pos: [-30, 1350, 2100] })
  })

  it('выходная стена: высота на 200 ниже (уровень низа блока второго этажа)', () => {
    const sf = { side: 'left' as const, height: EXIT_BLOCK_H }
    // Стена Л — выходная: высота = 2700−200 = 2500, верх = 2500, центр = 1250.
    expect(wallBox(room, 'left', t, sf)).toEqual({ size: [60, 2500, 4200], pos: [-30, 1250, 2100] })
    // Стена В — не выходная, полная высота.
    expect(wallBox(room, 'top', t, sf)).toEqual({ size: [3000, 2700, 60], pos: [1500, 1350, 4230] })
  })

  it('secondFloor уменьшает только свою сторону — проверены все 4 стороны', () => {
    const sf = { side: 'top' as const, height: EXIT_BLOCK_H }
    // Выход на В: только top урезан, остальные полные.
    expect(wallBox(room, 'top', t, sf)).toEqual({ size: [3000, 2500, 60], pos: [1500, 1250, 4230] })
    expect(wallBox(room, 'bottom', t, sf)).toEqual({ size: [3000, 2700, 60], pos: [1500, 1350, -30] })
    expect(wallBox(room, 'right', t, sf)).toEqual({ size: [60, 2700, 4200], pos: [3030, 1350, 2100] })
    expect(wallBox(room, 'left', t, sf)).toEqual({ size: [60, 2700, 4200], pos: [-30, 1350, 2100] })
  })

  it('secondFloor выше стены: выходная стена срезается к минимальной толщине 1 мм', () => {
    const sf = { side: 'left' as const, height: 2700 } // = полная высота
    expect(wallBox(room, 'left', t, sf)).toEqual({ size: [60, 1, 4200], pos: [-30, 0, 2100] })
  })

  it('стены начинаются от нижней грани меша (пол помещения не на нуле)', () => {
    // Реальный room_mesh: пол на Y∈[−20,20], верх стен 2700 → высота 2720,
    // центр на 1340.
    const withFloor = { min: { x: 0, y: -20, z: 0 }, max: { x: 3000, y: 2700, z: 4200 } }
    expect(wallBox(withFloor, 'top', t)).toEqual({ size: [3000, 2720, 60], pos: [1500, 1340, 4230] })
    const sf = { side: 'left' as const, height: EXIT_BLOCK_H }
    expect(wallBox(withFloor, 'left', t, sf)).toEqual({ size: [60, 2520, 4200], pos: [-30, 1240, 2100] })
  })

  it('толщина стены двигает плоскость и размер по здоровой оси', () => {
    const thick = 120
    expect(wallBox(room, 'right', thick)).toEqual({ size: [120, 2700, 4200], pos: [3060, 1350, 2100] })
    expect(wallBox(room, 'bottom', thick)).toEqual({ size: [3000, 2700, 120], pos: [1500, 1350, -60] })
  })

  it('высота стен — из параметров (высота марша = 2700), а не из меша', () => {
    const tall = { min: { x: 0, y: 0, z: 0 }, max: { x: 3000, y: 3200, z: 4200 } }
    expect(wallBox(tall, 'right', t)).toEqual({ size: [60, 3200, 4200], pos: [3030, 1600, 2100] })
  })
})

describe('wallBoundsOf (габарит стен = периметр помещения + высота ввода)', () => {
  const sb = { min: { x: -5050, y: 0, z: 0 }, max: { x: -960, y: 2700, z: 900 } } as const
  const rb = { min: { x: 0, y: -20, z: 0 }, max: { x: 3000, y: 20, z: 4200 } } as const

  it('heightMM задаёт верх стен', () => {
    expect(wallBoundsOf({ stairBox: sb, roomWidth: 3000, roomLength: 4200, heightMM: 2900 })).toEqual({
      min: { x: 0, y: 0, z: 0 },
      max: { x: 3000, y: 2900, z: 4200 },
    })
  })

  it('heightMM 0 / undefined / отрицателен — фолбэк на геометрический верх меша', () => {
    for (const h of [undefined, 0, -5]) {
      expect(wallBoundsOf({ stairBox: sb, roomWidth: 3000, roomLength: 4200, heightMM: h })).toEqual({
        min: { x: 0, y: 0, z: 0 },
        max: { x: 3000, y: 2700, z: 4200 },
      })
    }
  })

  it('меш помещения приоритетнее roomWidth/roomLength; Y берётся от меша лестницы', () => {
    expect(
      wallBoundsOf({ stairBox: sb, roomBounds: rb, roomWidth: 3333, roomLength: 4444, heightMM: 2700 }),
    ).toEqual({
      min: { x: 0, y: 0, z: 0 },
      max: { x: 3000, y: 2700, z: 4200 },
    })
  })

  it('без меша помещения и с неполными габаритами — null (виджет залочен)', () => {
    expect(wallBoundsOf({ stairBox: sb })).toBeNull()
    expect(wallBoundsOf({ stairBox: sb, roomWidth: 3000 })).toBeNull() // нет длины
    expect(wallBoundsOf({ stairBox: sb, roomWidth: 0, roomLength: 0 })).toBeNull()
    expect(wallBoundsOf({ stairBox: sb, roomWidth: -3000, roomLength: 4200 })).toBeNull() // отрицательная ширина
  })

  it('меш помещения покрывает неполные габариты: room_mesh есть — стены строятся', () => {
    expect(wallBoundsOf({ stairBox: sb, roomBounds: rb })).toEqual({
      min: { x: 0, y: 0, z: 0 },
      max: { x: 3000, y: 2700, z: 4200 },
    })
  })
})

describe('wallBox ∘ wallBoundsOf (интеграция: габарит из ввода → стены)', () => {
  const t = 60
  it('высота ввода 2900: обычные стены 2900, выходная 2700 (2900 − 200)', () => {
    const bounds = wallBoundsOf({ stairBox: straightBox, roomWidth: 3000, roomLength: 4200, heightMM: 2900 })!
    expect(wallBox(bounds, 'top', t, { side: 'left', height: EXIT_BLOCK_H })).toEqual({
      size: [3000, 2900, 60],
      pos: [1500, 1450, 4230],
    })
    expect(wallBox(bounds, 'left', t, { side: 'left', height: EXIT_BLOCK_H })).toEqual({
      size: [60, 2700, 4200],
      pos: [-30, 1350, 2100],
    })
  })
})

describe('wallPlaneX / wallPlaneZ / wallSpanX / wallSpanZ', () => {
  it('П/Л: центр стены снаружи торцов марша на половину толщины', () => {
    expect(wallPlaneX(straightBox, 'right', 60)).toBe(-930) // box.max.x + 30
    expect(wallPlaneX(straightBox, 'left', 60)).toBe(-5080) // box.min.x − 30
  })
  it('В/Н: центр стены снаружи кромок ширины на половину толщины', () => {
    expect(wallPlaneZ(straightBox, 'top', 60)).toBe(930) // box.max.z + 30
    expect(wallPlaneZ(straightBox, 'bottom', 60)).toBe(-30) // box.min.z − 30
  })
  it('длины стен по осям: В/Н — вдоль X, П/Л — вдоль Z', () => {
    expect(wallSpanX(straightBox)).toEqual({ x0: -5050, x1: -960 })
    expect(wallSpanZ(straightBox)).toEqual({ z0: 0, z1: 900 })
  })
})