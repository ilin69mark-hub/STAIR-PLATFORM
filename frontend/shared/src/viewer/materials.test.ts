import { describe, expect, it } from 'vitest'
import * as THREE from 'three'
import { railingPartsOf } from './picking'
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
  it('пропускает свет настоящим transmission, а не opacity', () => {
    // Регрессия: «стекло» было opacity 0.32 — мутная пластина, которая
    // разоблачалась, как только панель оказывалась на переднем плане.
    const m = createGlassMaterial() as THREE.MeshPhysicalMaterial
    expect(m.isMeshPhysicalMaterial).toBe(true)
    expect(m.transmission).toBeGreaterThan(0.8)
    // opacity при transmission = 1: прозрачность даёт двойное смешивание.
    expect(m.opacity).toBe(1)
  })

  it('ior стекла, а не пластика (1.52) и толщина панели 10 мм', () => {
    const m = createGlassMaterial()
    expect(m.ior).toBeCloseTo(1.52, 2)
    expect(m.thickness).toBeCloseTo(0.01, 3)
  })

  it('панели замкнутые — FrontSide, иначе стекло просвечивает дважды', () => {
    expect(createGlassMaterial().side).toBe(THREE.FrontSide)
  })

  it('роль railing_glass даёт стекло, а не металл', () => {
    const m = createRailingMaterialForRole('railing_glass', false)
    expect((m as THREE.MeshPhysicalMaterial).transmission).toBeGreaterThan(0.8)
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
