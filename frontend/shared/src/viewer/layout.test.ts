import { describe, expect, it } from 'vitest'
import {
  approachZoneCenterX,
  entranceFaceX,
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

  it('стены начинаются от нижней грани меша (пол помещения не на нуле)', () => {
    // Реальный room_mesh: пол на Y∈[−20,20], верх стен 2700 → высота 2720,
    // центр на 1340.
    const withFloor = { min: { x: 0, y: -20, z: 0 }, max: { x: 3000, y: 2700, z: 4200 } }
    expect(wallBox(withFloor, 'top', t)).toEqual({ size: [3000, 2720, 60], pos: [1500, 1340, 4230] })
    expect(wallBox(withFloor, 'left', t)).toEqual({ size: [60, 2720, 4200], pos: [-30, 1340, 2100] })
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
  it('высота ввода 2900: все стены полной высоты (плиты выхода в 3D нет)', () => {
    const bounds = wallBoundsOf({ stairBox: straightBox, roomWidth: 3000, roomLength: 4200, heightMM: 2900 })!
    expect(wallBox(bounds, 'top', t)).toEqual({ size: [3000, 2900, 60], pos: [1500, 1450, 4230] })
    expect(wallBox(bounds, 'left', t)).toEqual({ size: [60, 2900, 4200], pos: [-30, 1450, 2100] })
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