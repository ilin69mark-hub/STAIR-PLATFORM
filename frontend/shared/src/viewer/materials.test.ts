import { describe, expect, it } from 'vitest'
import * as THREE from 'three'
import { compactTriangles, railingPartsOf } from './picking'
import {
  createGlassMaterial,
  createRailingMaterialForRole,
  createRailingMetalMaterial,
  createStairMaterial,
} from './materials'

/**
 * Меш ограждения: total треугольников и диапазоны ролей по треугольникам.
 * Ключи диапазонов — как в API (MeshPartRange: Role/Start/End), и это
 * существенно: groupsFromRanges читает именно их, и фикстура с
 * произвольными именами полей молча ушла бы в ветку «старый API без
 * PartRanges» — тест прошёл бы, ничего не проверяя.
 */
function railingMesh(
  total: number,
  ranges: { role: string; start: number; end: number }[],
): any {
  const Triangles: [number, number, number][] = []
  for (let i = 0; i < total; i++) Triangles.push([i, i + 1, i + 2])
  return {
    Vertices: Array.from({ length: total + 2 }, (_, i) => ({ X: i, Y: 0, Z: 0 })),
    Triangles,
    PartRanges: ranges.map((r, i) => ({ Solid: i, Role: r.role, Start: r.start, End: r.end })),
  }
}

describe('разбивка меша ограждения по ролям', () => {
  it('стекло отделяется от поручня и стоек', () => {
    // поручень (0..2), стойки (2..8), стекло (8..14), поручень площадки (14..16)
    const parts = railingPartsOf(railingMesh(16, [
      { role: 'railing', start: 0, end: 2 },
      { role: 'baluster', start: 2, end: 8 },
      { role: 'railing_glass', start: 8, end: 14 },
      { role: 'railing', start: 14, end: 16 },
    ]))
    expect(parts.map((p) => p.role)).toEqual(['railing', 'baluster', 'railing_glass', 'railing'])
    expect(parts.map((p) => p.triangles.length)).toEqual([2, 6, 6, 2])
  })

  it('смежные диапазоны одной роли объединяются в один меш', () => {
    // Без склейки на 15 ступенях получилось бы 30+ мешей, каждый со своим
    // материалом и своей компиляцией шейдера.
    const parts = railingPartsOf(railingMesh(8, [
      { role: 'baluster', start: 0, end: 2 },
      { role: 'baluster', start: 2, end: 4 },
      { role: 'baluster', start: 4, end: 6 },
    ]))
    expect(parts).toHaveLength(1)
    expect(parts[0].triangles).toHaveLength(6)
  })

  it('без PartRanges (старый API) всё ограждение — одна часть', () => {
    const parts = railingPartsOf(railingMesh(3, []))
    expect(parts).toHaveLength(1)
    expect(parts[0].role).toBe('railing')
    expect(parts[0].triangles).toHaveLength(3)
  })

  it('пустой диапазон не даёт меша без треугольников', () => {
    const parts = railingPartsOf(railingMesh(4, [
      { role: 'railing_glass', start: 0, end: 0 },
      { role: 'railing', start: 0, end: 4 },
    ]))
    expect(parts.map((p) => p.role)).toEqual(['railing'])
  })

  it('диапазон за концом массива обрезается, а не читает за границу', () => {
    const parts = railingPartsOf(railingMesh(3, [
      { role: 'railing', start: 0, end: 99 },
      { role: 'railing_glass', start: 90, end: 99 },
    ]))
    expect(parts).toHaveLength(1)
    expect(parts[0].triangles).toHaveLength(3)
  })
})

describe('стекло ограждения', () => {
  it('прозрачно и не преломляет пустоту', () => {
    // Регрессия, найденная на живой сцене: transmission 0.94 давал ЧЁРНЫЕ
    // панели. three.js в renderTransmissionPass обнуляет scene.background,
    // поэтому стекло преломляет только непрозрачную геометрию, а всё
    // остальное приходит нулями. У ограждения половина панели всегда висит
    // на силуэте — там преломлять нечего, и панель становится чёрной.
    const m = createGlassMaterial()
    expect(m.transparent).toBe(true)
    expect(m.transmission ?? 0).toBe(0)
    expect(m.opacity).toBeGreaterThan(0.05)
    expect(m.opacity).toBeLessThan(0.6)
  })

  it('отражает: env-слой сильный, иначе при альфе стекло не видно', () => {
    // При прозрачности 0.28 блик тоже умножается на alpha — без усиленного
    // env-отражения панель просто исчезает, и ограждения не видно.
    const m = createGlassMaterial()
    expect(m.envMapIntensity).toBeGreaterThan(1.5)
    expect(m.clearcoat).toBeGreaterThan(0.5)
  })

  it('гладкая поверхность и настоящий показатель преломления', () => {
    const m = createGlassMaterial()
    expect(m.roughness).toBeLessThan(0.1)
    expect(m.ior).toBeCloseTo(1.52, 2)
  })

  it('панели замкнутые — FrontSide, иначе стекло просвечивает дважды', () => {
    expect(createGlassMaterial().side).toBe(THREE.FrontSide)
  })

  it('depthWrite выключен: 14 панелей не должны запечатывать буфер глубины', () => {
    expect(createGlassMaterial().depthWrite).toBe(false)
  })

  it('роль railing_glass даёт стекло, а не металл', () => {
    const m = createRailingMaterialForRole('railing_glass', false)
    expect(m.transparent).toBe(true)
  })

  it('во «металлическом» ограждении заполнение непрозрачное', () => {
    const m = createRailingMaterialForRole('railing_glass', true) as THREE.MeshStandardMaterial
    expect(m.transparent).toBe(false)
    expect(m.metalness).toBeGreaterThan(0.5)
  })
})

describe('металл ограждения', () => {
  it('непрозрачный и отражающий', () => {
    const m = createRailingMetalMaterial(false)
    expect(m.transparent).toBe(false)
    expect(m.metalness).toBeGreaterThan(0.5)
    expect(m.envMapIntensity).toBeGreaterThan(1)
  })
})

describe('лак поверх древесины', () => {
  it('«Матовый лак» даёт MeshPhysicalMaterial с clearcoat', () => {
    const m = createStairMaterial({ code: 'WOOD-OAK', finishId: 'matte', role: 'tread' })
    expect(m).toBeInstanceOf(THREE.MeshPhysicalMaterial)
    expect((m as THREE.MeshPhysicalMaterial).clearcoat).toBeGreaterThan(0)
  })

  it('регрессия: Physical-финиш не теряет envMapIntensity', () => {
    // Раньше в ветке clearcoat создавался Physical-материал, у которого
    // envMapIntensity не выставлялся вовсе: лаковый финиш был темнее и площе
    // соседних, хотя объявлен блестящим.
    const m = createStairMaterial({ code: 'WOOD-OAK', finishId: 'matte', role: 'tread' })
    expect(m.envMapIntensity).toBeGreaterThan(1)
  })

  it('«Масло» остаётся дешёвым MeshStandardMaterial без clearcoat', () => {
    const m = createStairMaterial({ code: 'WOOD-OAK', finishId: 'oil', role: 'tread' })
    expect(m).toBeInstanceOf(THREE.MeshStandardMaterial)
    expect(m).not.toBeInstanceOf(THREE.MeshPhysicalMaterial)
  })
})

describe('компактификация геометрии по треугольникам', () => {
  // 4 вершины, 2 треугольника: [0,1,2] и [0,2,3]
  const positions = new Float32Array([
    0, 0, 0,
    100, 0, 0,
    100, 100, 0,
    0, 100, 0,
  ])
  const uvs = new Float32Array([0, 0, 1, 0, 1, 1, 0, 1])

  it('оставляет только используемые вершины', () => {
    const out = compactTriangles(positions, uvs, [[0, 1, 2]])
    expect(out!.positions).toHaveLength(9) // 3 вершины по 3 координаты
    expect(out!.indices).toHaveLength(3)
    expect(Array.from(out!.positions)).toEqual([0, 0, 0, 100, 0, 0, 100, 100, 0])
  })

  it('переиндексирует без потери winding', () => {
    // Регрессия: при общей карте «старый→новый» повторно встречающаяся
    // вершина обязана получать ТОТ ЖЕ новый индекс, иначе в треугольнике
    // появится вырожденное ребро, а вместе с ним — неверная нормаль.
    const out = compactTriangles(positions, uvs, [[0, 1, 2], [0, 2, 3]])
    expect(Array.from(out!.indices)).toEqual([0, 1, 2, 0, 2, 3])
    expect(out!.positions).toHaveLength(12)
  })

  it('uv едут вместе с вершинами', () => {
    const out = compactTriangles(positions, uvs, [[0, 1, 2]])
    expect(Array.from(out!.uvs!)).toEqual([0, 0, 1, 0, 1, 1])
  })

  it('без uv остаётся null, а не пустой буфер', () => {
    const out = compactTriangles(positions, null, [[0, 1, 2]])
    expect(out!.uvs).toBeNull()
  })

  it('пустой набор треугольников не даёт геометрии', () => {
    expect(compactTriangles(positions, uvs, [])).toBeNull()
  })

  it('ГЛАВНОЕ: габарит части совпадает с её треугольниками, а не со всем мешем',
    () => {
      // Именно из-за этого панели стекла не рисовались: computeBoundingSphere
      // смотрит на весь буфер position, поэтому у всех частей была сфера
      // середины марша, и frustumCulled выбрасывал панели при приближении
      // камеры. Проверяем, что после компактификации габарит части — её
      // собственный.
      const meshPositions = new Float32Array([
        // Часть A: треугольник у x=0
        0, 0, 0, 10, 0, 0, 10, 10, 0,
        // Часть B: треугольник у x=5000
        5000, 0, 0, 5010, 0, 0, 5010, 10, 0,
      ])
      const a = compactTriangles(meshPositions, null, [[0, 1, 2]])!
      const b = compactTriangles(meshPositions, null, [[3, 4, 5]])!
      // Только X-координаты: в Y и Z там нули, и Math.min по всему буферу
      // всегда дал бы 0.
      const minX = (arr: Float32Array) => Math.min(...Array.from({ length: arr.length / 3 }, (_, i) => arr[i * 3]))
      const minA = minX(a.positions)
      const minB = minX(b.positions)
      expect(minA).toBe(0)
      expect(minB).toBe(5000)
      expect(minB - minA).toBe(5000)
    })
})
