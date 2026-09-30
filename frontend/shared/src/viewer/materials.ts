// PBR-материалы по ролям деталей (этап 1 «студийный 3D»).
//
// Роли приходят из бэкенда (Mesh.PartRanges: tread, stringer, landing,
// railing_*), код материала — из пользовательского ввода (WOOD-OAK,
// WOOD-ASH, …). Здесь код превращается в PBR-набор: базовый цвет +
// roughness/metalness + ленивая загрузка карт из /static-assets/pbr/<code>/.
//
// Принципы:
//   - сцена никогда не «пустая»: пока текстуры грузятся (или если их нет)
//     используется процедурный материал того же тона;
//   - кэш общий на модуль: повторное переключение материала мгновенное,
//     сеть не дёргается заново;
//   - normal/roughness/AO читаются как ЛИНЕЙНЫЕ данные (NoColorSpace),
//     цвет — как sRGB. Ошибка здесь даёт «мыльный» свет и серые тени.

import * as THREE from 'three'

export type MaterialCode = string

export interface FinishSpec {
  id: string
  label: string
  /** Множитель цвета поверх текстуры (1 = как есть). */
  tint?: number
  /** Оттенок краски/морилки в hex; undefined —natural. */
  color?: number
  roughnessFactor?: number
  metalnessFactor?: number
  clearcoat?: number
}

/** Базовые параметры материалов (когда текстуры ещё нет / их нет). */
const BASE: Record<string, { color: number; roughness: number; metalness: number }> = {
  'WOOD-OAK': { color: 0xc9a678, roughness: 0.62, metalness: 0 },
  'WOOD-WALNUT': { color: 0x6b4630, roughness: 0.5, metalness: 0 },
  'WOOD-ASH': { color: 0xdfc49a, roughness: 0.66, metalness: 0 },
  'WOOD-SOFT': { color: 0xe0c39a, roughness: 0.75, metalness: 0 },
  'STEEL-S235': { color: 0x9aa3ad, roughness: 0.42, metalness: 0.85 },
}

/** Финиши одного материала: меняют вид, но не код материала и не цену. */
export const FINISHES: Record<string, FinishSpec[]> = {
  'WOOD-OAK': [
    { id: 'oil', label: 'Масло', roughnessFactor: 0.62 },
    { id: 'matte', label: 'Матовый лак', roughnessFactor: 0.4, clearcoat: 0.25 },
    { id: 'toned', label: 'Тонировка', color: 0x9a7448, roughnessFactor: 0.55 },
  ],
  'STEEL-S235': [
    { id: 'raw', label: 'Без покрытия', roughnessFactor: 0.42 },
    { id: 'black', label: 'Чёрный', color: 0x24262a, roughnessFactor: 0.46 },
    { id: 'white', label: 'Белый', color: 0xe8e8e6, roughnessFactor: 0.5 },
  ],
}

export function finishFor(code: MaterialCode, finishId?: string): FinishSpec {
  const list = FINISHES[code] ?? []
  return list.find((f) => f.id === finishId) ?? { id: 'natural', label: 'Без финиша' }
}

/** Базовый URL ассетов. Пустая строка → только процедурные материалы. */
let assetsBase = '/static-assets'

export function setAssetsBase(base: string) {
  assetsBase = base.replace(/\/$/, '')
}

interface LoadedSet {
  map?: THREE.Texture
  normalMap?: THREE.Texture
  roughnessMap?: THREE.Texture
}

const cache = new Map<string, LoadedSet>()
const loading = new Map<string, Promise<LoadedSet>>()

const loader = new THREE.TextureLoader()

function loadOne(url: string, srgb: boolean): Promise<THREE.Texture> {
  return new Promise((resolve) => {
    loader.load(
      url,
      (tex) => {
        // Нормали/шероховатость/AO — линейные данные; цвет — sRGB.
        tex.colorSpace = srgb ? THREE.SRGBColorSpace : THREE.NoColorSpace
        tex.wrapS = THREE.RepeatWrapping
        tex.wrapT = THREE.RepeatWrapping
        tex.anisotropy = 8
        resolve(tex)
      },
      undefined,
      () => resolve(undefined as unknown as THREE.Texture),
    )
  })
}

function loadSet(code: MaterialCode): Promise<LoadedSet> {
  const hit = cache.get(code)
  if (hit) return Promise.resolve(hit)
  const inflight = loading.get(code)
  if (inflight) return inflight

  const base = `${assetsBase}/pbr/${code}`
  // Загружаются ТРИ карты на материал. AO-карты не запрашиваются: их нет в
  // проекте, и быть не должно — затенение от окружения (ambient occlusion) не
  // свойство плоской доски, а свойство СЦЕНЫ: оно живёт в стыке проступи с
  // косоуром, в пазу под поручнем. Наклеить такую карту на текстуру доски
  // нельзя. AO появится в рендере (SSAO, фаза 4) и в вершинных цветах меша,
  // где он считается по реальной геометрии.
  const p = Promise.all([
    loadOne(`${base}/color.jpg`, true),
    loadOne(`${base}/normal.jpg`, false),
    loadOne(`${base}/roughness.jpg`, false),
  ])
    .then(([map, normalMap, roughnessMap]) => {
      const set: LoadedSet = { map, normalMap, roughnessMap }
      cache.set(code, set)
      loading.delete(code)
      return set
    })
    .catch(() => {
      cache.set(code, {})
      loading.delete(code)
      return {} as LoadedSet
    })
  loading.set(code, p)
  return p
}

/** Сколько раз на метр материала должна повторяться текстура. */
function repeatFor(role: string, sizeMM: number): number {
  // Крупная деталь (площадка/косоур) — меньше повторов, чтобы рисунок не
  // «мельчил»; ступень — чаще.
  const perM = role === 'tread' ? 3.2 : role.startsWith('railing') ? 1.4 : 1.0
  return Math.max(0.5, (sizeMM / 1000) * perM)
}

export interface StairMaterialOptions {
  code: MaterialCode
  finishId?: string
  role: string
  /** Габарит детали по наибольшей стороне, мм — для масштаба текстуры. */
  sizeMM?: number
}

/**
 * Смещение глубины по роли детали.
 *
 * ГЕОМЕТРИЯ ИМЕЕТ КОНТАКТНЫЕ (СОВПАДАЮЩИЕ) ПЛОСКОСТИ. На каждом уровне
 * ступени три грани лежат ровно в одной плоскости Z = (k+1)·h − st:
 *   - седло пилы косоура (верх гребёнки),
 *   - низ проступи,
 *   - верх подступенка.
 * Для П-марша дополнительно совпадают: низ подступенка верхнего марша с
 * верхом площадки, и боковые грани площадки с внешними гранями маршей.
 *
 * Это физически верно (детали реально стоят друг на друге), но в WebGL
 * две грани в одной плоскости делят одну и ту же ячейку буфера глубины.
 * Победитель определяется порядком обхода и округлением — по поверхности
 * проступами проступает КОСОУР (а на П-марше ступени слипаются в «кучку»).
 *
 * Разводить их в геометрии нельзя: параметрическая модель — источник
 * истины (BC-002), и зазор 0 мм попал бы в раскрой и расчёт массы.
 * Поэтому разводим только РЕНДЕР: polygonOffset сдвигает фрагмент в
 * буфере глубины, не меняя ни вершины, ни объём тела.
 *
 * Приоритет: tread/landing — «наружная» поверхность, она и должна быть
 * видна; stringer/riser/winder отступают, чтобы не пробиваться сквозь неё.
 */
const ROLE_DEPTH_BIAS: Record<string, { factor: number; units: number }> = {
  tread: { factor: 1, units: 1 },
  landing: { factor: 1, units: 1 },
  winder: { factor: 2, units: 2 },
  riser: { factor: 3, units: 3 },
  stringer: { factor: 4, units: 4 },
}

function depthBiasFor(role: string): { polygonOffset: true; polygonOffsetFactor: number; polygonOffsetUnits: number } {
  const b = ROLE_DEPTH_BIAS[role] ?? { factor: 1, units: 1 }
  return {
    polygonOffset: true,
    // factor — зависит от наклона грани, units — от глубины; оба нужны,
    // иначе смещение «плывёт» при наклоне камеры.
    polygonOffsetFactor: -b.factor,
    polygonOffsetUnits: -b.units,
  }
}

/**
 * Создаёт PBR-материал детали. Текстуры грузятся лениво: материал сразу
 * процедурный (сцена жива), при загрузке карт он обновляется на месте.
 */
export function createStairMaterial(opts: StairMaterialOptions): THREE.MeshStandardMaterial {
  const base = BASE[opts.code] ?? { color: 0xb0b0b0, roughness: 0.6, metalness: 0.1 }
  const finish = finishFor(opts.code, opts.finishId)
  const color = finish.color ?? base.color

  const material = new THREE.MeshStandardMaterial({
    color,
    roughness: Math.min(1, Math.max(0.05, finish.roughnessFactor ?? base.roughness)),
    metalness: Math.min(1, Math.max(0, (finish.metalnessFactor ?? 1) * base.metalness)),
    side: THREE.DoubleSide,
    envMapIntensity: 1.0,
    ...depthBiasFor(opts.role),
  })

  if (finish.clearcoat) {
    // Финиш с лаком: Physical-слой даёт блик поверх древесины.
    const physical = new THREE.MeshPhysicalMaterial({
      color: material.color,
      roughness: material.roughness,
      metalness: material.metalness,
      clearcoat: finish.clearcoat,
      clearcoatRoughness: 0.25,
      side: THREE.DoubleSide,
      ...depthBiasFor(opts.role),
    })
    applyMaps(physical, opts)
    void loadSet(opts.code).then((set) => {
      applyMaps(physical, opts, set)
      physical.needsUpdate = true
    })
    return physical
  }

  applyMaps(material, opts)
  void loadSet(opts.code).then((set) => {
    applyMaps(material, opts, set)
    material.needsUpdate = true
  })
  return material
}

function applyMaps(
  material: THREE.MeshStandardMaterial,
  opts: StairMaterialOptions,
  set: LoadedSet = cache.get(opts.code) ?? {},
) {
  if (set.map) {
    const t = set.map.clone()
    t.needsUpdate = true
    const r = repeatFor(opts.role, opts.sizeMM ?? 1000)
    t.repeat.set(r, r)
    material.map = t
    const tint = finishFor(opts.code, opts.finishId).tint
    if (tint) material.color.multiplyScalar(tint)
  }
  if (set.normalMap) {
    const t = set.normalMap.clone()
    const r = repeatFor(opts.role, opts.sizeMM ?? 1000)
    t.repeat.set(r, r)
    material.normalMap = t
    material.normalScale = new THREE.Vector2(0.6, 0.6)
  }
  if (set.roughnessMap) {
    const t = set.roughnessMap.clone()
    const r = repeatFor(opts.role, opts.sizeMM ?? 1000)
    t.repeat.set(r, r)
    material.roughnessMap = t
  }
}

/** Материал ограждения: стекло по умолчанию, металл — по флагу. */
export function createRailingMaterial(metal: boolean): THREE.MeshStandardMaterial {
  if (metal) {
    return new THREE.MeshStandardMaterial({
      color: 0x8f979f,
      roughness: 0.35,
      metalness: 0.9,
      side: THREE.DoubleSide,
      envMapIntensity: 1.2,
    })
  }
  // «Стекло» без transmission (дорого): прозрачность + env-отражения дают
  // убедительный вид при 450 треугольниках сцены.
  return new THREE.MeshStandardMaterial({
    color: 0xcfe3ea,
    roughness: 0.08,
    metalness: 0.1,
    transparent: true,
    opacity: 0.32,
    side: THREE.DoubleSide,
    envMapIntensity: 1.6,
  })
}
