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
  const roughness = Math.min(1, Math.max(0.05, finish.roughnessFactor ?? base.roughness))
  const metalness = Math.min(1, Math.max(0, (finish.metalnessFactor ?? 1) * base.metalness))

  // Финиш с лаком требует MeshPhysicalMaterial (слой clearcoat), остальное
  // держится на дешёвом MeshStandardMaterial. Материал выбирается ОДИН раз:
  // раньше здесь сначала создавался MeshStandardMaterial, а при clearcoat он
  // молча выбрасывался и на сцену уходил Physical — с потерянным
  // envMapIntensity, то есть финиш «Матовый лак» выглядел темнее и площе
  // остальных, хотя объявлен как блестящий.
  const material: THREE.MeshStandardMaterial = finish.clearcoat
    ? new THREE.MeshPhysicalMaterial({
        color,
        roughness,
        metalness,
        clearcoat: finish.clearcoat,
        // Лаковый слой гладкий и тонкий: roughness слоя заметно ниже
        // roughness древесины, иначе блик размазывается в пятно.
        clearcoatRoughness: Math.min(0.2, roughness * 0.5),
        side: THREE.DoubleSide,
        envMapIntensity: 1.15,
        ...depthBiasFor(opts.role),
      })
    : new THREE.MeshStandardMaterial({
        color,
        roughness,
        metalness,
        side: THREE.DoubleSide,
        // Металл живёт ОТРАЖЕНИЯМИ: у него почти нет диффузной
        // составляющей, и при envMapIntensity 1.0 грани, повёрнутые от
        // источников и от яркой части HDRI, уходили в чёрное — стальной
        // косоур выглядел дырой в картинке. Дереву и камню 1.15 не нужен:
        // там отражения — лишь тонкий налёт поверх диффузии.
        envMapIntensity: metalness > 0.5 ? 1.4 : 1.15,
        ...depthBiasFor(opts.role),
      })

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

/**
 * Стекло ограждения.
 *
 * Сначала здесь стояло `transmission: 0.94` — «настоящее» стекло с
 * преломлением. На этой сцене оно даёт ЧЁРНЫЕ панели, и виноват не материал:
 * three.js в `renderTransmissionPass` НАМЕРЕННО обнуляет `scene.background`
 * (иначе преломлялся бы фон, а не геометрия), поэтому стекло преломляет
 * картинку, в которую попала только непрозрачная геометрия. Всё, что не
 * закрыто деталью, приходит в шейдер нулями, и панель становится чёрной.
 *
 * Проверено экспериментом: стекло над непрозрачной подложкой показывает её
 * цвет (229,40,40), а стекло на пустом месте — чёрное. У ограждения почти
 * всегда половина панели висит на силуэте, то есть на пустом месте.
 *
 * Поэтому здесь честная альтернатива: альфа-прозрачность + зеркальный слой от
 * HDRI. Принципиальный признак стекла — не преломление, а ЯРКИЙ ПОВЕРХНОСТНЫЙ
 * БЛИК при высокой прозрачности, и его альбера даёт именно env-отражение.
 * Преломление в масштабе, где лестница целиком в кадре, всё равно не видно.
 * Прозрачность не зависит от того, что за панелью, — стекло одинаково читается
 * и на силуэте, и на фоне ступеней.
 */
export function createGlassMaterial(): THREE.MeshPhysicalMaterial {
  return new THREE.MeshPhysicalMaterial({
    // Лёгкий зеленовато-голубой тон: кромка стекла на просвет всегда
    // чуть зеленее (примесь Fe2+ в float-стекле), и это отличает его от
    // бесцветного пластика.
    color: 0xe4f2f4,
    roughness: 0.04,
    metalness: 0,
    // Прозрачность намеренно НЕ нулевая и не 0.32: при 0.32 панель
    // перестаёт просвечивать, при 0.05 исчезает совсем.
    transparent: true,
    opacity: 0.28,
    // Панели — отдельные непересекающиеся тела, но их 14, и при depthWrite
    // первая же панель «запечатывает» буфер глубины: дальние панели и
    // лестница за ними начинают пропадать в зависимости от порядка.
    depthWrite: false,
    side: THREE.FrontSide,
    // Основной признак стекла — отражение. При прозрачности 0.28 блик
    // тоже умножается на alpha, поэтому envMapIntensity поднят сильно.
    envMapIntensity: 2.2,
    // Глянцевый верхний слой поверх «тела» стекла: даёт резкий блик на
    // верхней кромке, ради которого панель и выглядит стеклом.
    clearcoat: 1,
    clearcoatRoughness: 0.02,
    ior: 1.52,
  })
}

/** Металл ограждения: поручень, стойки и (в металлическом варианте) панели. */
export function createRailingMetalMaterial(panel: boolean): THREE.MeshStandardMaterial {
  return new THREE.MeshStandardMaterial({
    color: panel ? 0x7d858e : 0x9aa3ad,
    roughness: panel ? 0.42 : 0.3,
    metalness: 0.9,
    side: THREE.DoubleSide,
    envMapIntensity: 1.25,
  })
}

/** Материал роли ограждения. Стекло — прозрачное, остальное — по флагу. */
export function createRailingMaterialForRole(role: string, metal: boolean): THREE.Material {
  const isGlass = role === 'railing_glass'
  if (isGlass && !metal) return createGlassMaterial()
  return createRailingMetalMaterial(isGlass)
}
