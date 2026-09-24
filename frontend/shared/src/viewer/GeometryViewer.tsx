// 3D-вьювер геометрии (FE-0017, ENG-GEO-0008): отображение preview mesh
// из снапшота. Вращение — ЛКМ, панорама — ПКМ/средняя, зум — колесо.

import { useEffect, useRef, useState } from 'react'
import * as THREE from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { RoomEnvironment } from 'three/examples/jsm/environments/RoomEnvironment.js'
import { RGBELoader } from 'three/examples/jsm/loaders/RGBELoader.js'
import type { Mesh as ApiMesh } from '../types'
import { toThreePositions } from './projection'
import { computePlacement } from '../placement'
import { approachZoneCenterX, EXIT_BLOCK_H, exitSlabBox, exitWallSide, stairTopLineX, wallBox, wallBoundsOf, type Box3Like, type BoxSpec, type WallSide } from './layout'
import { ANNOTATE, edgeColor, WALLS } from '../scheme-annot'

interface Props {
  mesh: ApiMesh
  roomMesh?: ApiMesh
  // railingMesh — декоративные перила отдельным мешем (бэкенд RailingMesh).
  // Рисуем сплошным материалом БЕЗ каркаса, чтобы между балясинами и
  // поручнями не появлялись лишние линии.
  railingMesh?: ApiMesh
  // stairTop — маркер для фиолетовой линии ширины марша: линия рисуется
  // горизонтально на полу (z = box.min.z) под верхним торцом марша (x =
  // box.max.x), вдоль ширины (из bounding box). Поле rise историческое и
  // сейчас не используется (линия всегда на полу).
  stairTop?: { rise: number }
  // Параметры размещения лестницы у дальней стены/угла помещения
  // (см. placement.ts). Без них лестница центрируется как раньше.
  flight?: string
  direction?: 'left' | 'right'
  roomWidth?: number
  roomLength?: number
  // Свободное пространство перед первой ступенью (EDR-0023), мм. Рисуем
  // полупрозрачную зону перед входом в марш, чтобы было видно место для
  // постановки ноги (норма 1000–1200 мм).
  approachSpace?: number
  // Толщина проступи (StepThickness, мм): лицевая панель марша отстоит от
  // box.max.x на эту величину (свес первой проступи), поэтому зона подхода
  // и линия верха привязаны к ней. По умолчанию 40 (default бэкенда).
  stepThickness?: number
  // Глубина плиты «выхода на 2-й этаж» (мм), примыкающей к верхнему торцу
  // марша. По умолчанию 1200.
  secondFloorDepth?: number
  // Высота марша из ввода пользователя (поле «Высота», мм). Задаёт верхнюю
  // кромку стен; при отстуствии берётся геометрический верх меша (sb.max.y).
  heightMM?: number
  // environmentHDRI — URL студийного HDRI (Poly Haven, CC0) для отражений.
  // Необязателен: при ошибке загрузки остаётся процедурный RoomEnvironment.
  environmentHDRI?: string
  // castShadow — принимают ли лестница/перила/пол тень (этап 1, студийный вид).
  castShadow?: boolean
}

// Временная метка-буква для 3D-разметки (debug, см. scheme-annot.ts): рисуем
// букву на прозрачном sprite поверх сцены. Значение в мм (сцена в мм).
function makeLabelSprite(text: string, color: string): THREE.Sprite {
  const size = 128
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d')
  if (ctx) {
    ctx.font = 'bold 84px sans-serif'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.lineJoin = 'round'
    ctx.lineWidth = 12
    ctx.strokeStyle = '#ffffff'
    ctx.strokeText(text, size / 2, size / 2)
    ctx.fillStyle = color
    ctx.fillText(text, size / 2, size / 2)
  }
  const tex = new THREE.CanvasTexture(canvas)
  const mat = new THREE.SpriteMaterial({ map: tex, depthTest: false, transparent: true })
  const sprite = new THREE.Sprite(mat)
  const s = 600
  sprite.scale.set(s, s, s)
  return sprite
}

// Отражение геометрии по оси X с сохранением корректных нормалей: зеркалим
// координаты вершин и переворачиваем порядок обхода граней (winding), чтобы
// внешние нормали остались внешними. Используется, чтобы 3D-меш прямого марша
// совпадал по ориентации с 2D-планом (первый шаг на +X).
function mirrorX(geo: THREE.BufferGeometry) {
  const pos = geo.attributes.position as THREE.BufferAttribute
  for (let i = 0; i < pos.count; i++) pos.setX(i, -pos.getX(i))
  pos.needsUpdate = true
  if (geo.index) {
    const idx = geo.index
    for (let i = 0; i < idx.count; i += 3) {
      const a = idx.getX(i + 1)
      const b = idx.getX(i + 2)
      idx.setX(i + 1, b)
      idx.setX(i + 2, a)
    }
    idx.needsUpdate = true
  } else {
    const arr = pos.array as Float32Array
    for (let i = 0; i < arr.length; i += 9) {
      for (let k = 0; k < 3; k++) {
        const t = arr[i + 3 + k]
        arr[i + 3 + k] = arr[i + 6 + k]
        arr[i + 6 + k] = t
      }
    }
  }
  geo.computeVertexNormals()
}

// Превью WebGL-контекста: не каждый браузер/устройство поддерживает трёхмерный
// рендер (P0-6). Проверяем доступность контекста заранее, чтобы не создавать
// THREE.WebGLRenderer без поддержки.
function webglSupported(): boolean {
  try {
    const canvas = document.createElement('canvas')
    return !!(canvas.getContext('webgl2') || canvas.getContext('webgl'))
  } catch {
    return false
  }
}

export function GeometryViewer({
  mesh,
  roomMesh,
  railingMesh,
  stairTop,
  flight,
  direction,
  roomWidth,
  environmentHDRI,
  castShadow = true,
  roomLength,
  approachSpace,
  stepThickness = 40,
  secondFloorDepth = 1200,
  heightMM,
}: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  // WebGL в браузере может быть отключён/недоступен (старые драйверы,
  // headless-окружение, аппаратное ограничение) — вместо краха компонента
  // (P0-6) показываем текстовый фолбэк, геометрия и цена доступны в таблицах.
  const [webglError, setWebglError] = useState(false)
  // Виджет «стены»: независимый тумблер на каждую из 4 сторон периметра
  // (В/Н/П/Л). По умолчанию все стены выключены — лестница рисуется чистой.
  const [walls, setWalls] = useState<Record<WallSide, boolean>>({
    top: false,
    bottom: false,
    right: false,
    left: false,
  })
  // Плита «выхода на 2-й этаж» — отдельный тумблер, включён по умолчанию.
  const [showExit, setShowExit] = useState(true)
  // Ссылки на объекты контекста (стены/плита), чтобы переключатели меняли
  // только .visible без пересоздания сцены (иначе сбрасывается камера).
  const wallsRef = useRef<Record<WallSide, THREE.Object3D | null>>({
    top: null,
    bottom: null,
    right: null,
    left: null,
  })
  const exitRef = useRef<THREE.Object3D | null>(null)
  // Актуальные значения тумблеров для эффекта построения сцены: туда они НЕ
  // входят зависимостями (переключение не должно пересобирать сцену), поэтому
  // начальную видимость читаем из зеркальных рефов.
  const wallsStateRef = useRef(walls)
  const showExitStateRef = useRef(showExit)

  useEffect(() => {
    wallsStateRef.current = walls
    if (wallsRef.current.top) wallsRef.current.top.visible = walls.top
    if (wallsRef.current.bottom) wallsRef.current.bottom.visible = walls.bottom
    if (wallsRef.current.right) wallsRef.current.right.visible = walls.right
    if (wallsRef.current.left) wallsRef.current.left.visible = walls.left
  }, [walls])

  useEffect(() => {
    showExitStateRef.current = showExit
    if (exitRef.current) exitRef.current.visible = showExit
  }, [showExit])

  useEffect(() => {
    const container = containerRef.current
    if (!container || !mesh?.Vertices || mesh.Vertices.length === 0 || !mesh.Triangles) return

    if (!webglSupported()) {
      setWebglError(true)
      return
    }

    const width = container.clientWidth || 600
    const height = container.clientHeight || 380

    const scene = new THREE.Scene()
    scene.background = new THREE.Color('#f7f9fc')

    const camera = new THREE.PerspectiveCamera(45, width / height, 1, 100000)
    let renderer: THREE.WebGLRenderer
    try {
      renderer = new THREE.WebGLRenderer({
        antialias: true,
        preserveDrawingBuffer: true,
        // Этап 1 «студийный 3D»: явный high-performance — влияет на выбор
        // GPU и на мобильных (Android/iOS переключают браузер на старый GL).
        powerPreference: 'high-performance',
      })
    } catch (err) {
      console.error('WebGL renderer init failed:', err)
      setWebglError(true)
      return
    }
    renderer.setSize(width, height)
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
    // Студийная цветокоррекция: ACES сжимает яркие блики металла/дерева без
    // выгорания, SRGB — правильная гамма для вывода на экран (до этого
    // сцена рендерилась в линейном пространстве и выглядела «бледно»).
    renderer.toneMapping = THREE.ACESFilmicToneMapping
    renderer.toneMappingExposure = 1.05
    renderer.outputColorSpace = THREE.SRGBColorSpace
    renderer.shadowMap.enabled = true
    renderer.shadowMap.type = THREE.PCFSoftShadowMap
    container.appendChild(renderer.domElement)

    const controls = new OrbitControls(camera, renderer.domElement)
    controls.enableDamping = true
    controls.dampingFactor = 0.08
    // Сохраняем последний кадр для КП — вариант А (твой ракурс)
    const saveLastFrame = () => {
      try {
        const dataURL = renderer.domElement.toDataURL('image/png')
        ;(window as Window & { __stairLast3D?: string }).__stairLast3D = dataURL
        try { sessionStorage.setItem('stairLast3D', dataURL) } catch {}
      } catch {}
    }

    // Студийный свет (этап 1): IBL даёт мягкие отражения и заполняющие
    // полутона, Directional — чёткую тень. HDRI грузится асинхронно; пока он
    // не готов (или не загрузился вовсе), работает процедурный RoomEnvironment
    // из three — сцена никогда не остаётся «чёрной».
    const pmrem = new THREE.PMREMGenerator(renderer)
    const roomEnv = new RoomEnvironment()
    const roomTarget = pmrem.fromScene(roomEnv, 0.04)
    scene.environment = roomTarget.texture
    scene.environmentIntensity = 0.85
    const hemi = new THREE.HemisphereLight(0xffffff, 0xbfc8d8, 0.45)
    scene.add(hemi)
    const dir = new THREE.DirectionalLight(0xffffff, 1.6)
    dir.position.set(2000, 4000, 3000)
    dir.castShadow = true
    dir.shadow.mapSize.set(1024, 1024)
    // Тени у лестницы: ортокамера по габаритам сцены (выставляется ниже,
    // когда известен bounding box марша).
    dir.shadow.camera.near = 100
    dir.shadow.camera.far = 20000
    dir.shadow.bias = -0.0012
    scene.add(dir)
    const dir2 = new THREE.DirectionalLight(0xffffff, 0.35)
    dir2.position.set(-2000, -1000, -3000)
    scene.add(dir2)

    // Студийный HDRI (Poly Haven, CC0) — необязательный: при ошибке сети
    // остаётся RoomEnvironment, разница только в характере бликов.
    const hdrUrl = environmentHDRI
    if (hdrUrl) {
      new RGBELoader().load(
        hdrUrl,
        (hdr: THREE.Texture) => {
          const target = pmrem.fromEquirectangular(hdr)
          scene.environment = target.texture
          scene.environmentIntensity = 1.0
          hdr.dispose()
        },
        undefined,
        () => {
          console.warn('3D: HDRI не загружен, используется процедурное окружение')
        },
      )
    }

    const makeMesh = (api: ApiMesh, material: THREE.Material) => {
      if (!api?.Vertices || !api?.Triangles) return null as unknown as { mesh: THREE.Mesh; geo: THREE.BufferGeometry }
      const positions = new Float32Array(toThreePositions(api.Vertices))
      const indices = new Uint32Array(api.Triangles.length * 3)
      api.Triangles.forEach((t, i) => {
        indices[i * 3] = t[0]
        indices[i * 3 + 1] = t[1]
        indices[i * 3 + 2] = t[2]
      })
      const g = new THREE.BufferGeometry()
      g.setAttribute('position', new THREE.BufferAttribute(positions, 3))
      g.setIndex(new THREE.BufferAttribute(indices, 1))
      g.computeVertexNormals()
      g.computeBoundingBox()
      const m = new THREE.Mesh(g, material)
      return { mesh: m, geo: g }
    }

    const stairMat = new THREE.MeshStandardMaterial({
      color: 0x4f8df7,
      roughness: 0.55,
      metalness: 0.12,
      side: THREE.DoubleSide,
    })
    stairMat.envMapIntensity = 1.0
    const stair = makeMesh(mesh, stairMat)
    if (castShadow) {
      stair.mesh.castShadow = true
      stair.mesh.receiveShadow = true
    }

    // ADR: 2D-план рисует первый шаг на +X, а 3D-меш генерирует первый шаг на
    // −X (зеркально). Чтобы 2D и 3D совпадали (один угол ВЛ, одна сторона
    // подхода, одинаковая ориентация начала марша), зеркалим меш прямого марша
    // по X. Затрагивает только геометрию лестницы, не помещение. l/u/spiral —
    // отдельная задача.
    if (flight === 'straight') {
      mirrorX(stair.geo)
      stair.geo.computeBoundingBox()
    }

    // Периметр помещения (room_mesh) — полупрозрачный, выделенным цветом,
    // чтобы визуально отделить «пол комнаты» от несущей лестницы.
    let room: { mesh: THREE.Mesh; geo: THREE.BufferGeometry } | null = null
    if (roomMesh?.Vertices?.length && roomMesh?.Triangles) {
      const roomMat = new THREE.MeshStandardMaterial({
        color: 0xffa94d,
        roughness: 0.9,
        metalness: 0.0,
        transparent: true,
        opacity: 0.32,
        side: THREE.DoubleSide,
      })
      room = makeMesh(roomMesh, roomMat)
      scene.add(room.mesh)
    }

    // Перила — отдельный меш БЕЗ каркаса (Issue 1): сплошной материал, без
    // EdgesGeometry, чтобы между балясинами и поручнями не рисовались лишние
    // линии. Координаты совпадают с телом марша, поэтому для прямого марша
    // зеркалим так же, как stair.geo.
    let railing: { mesh: THREE.Mesh; geo: THREE.BufferGeometry } | null = null
    if (railingMesh?.Vertices?.length && railingMesh?.Triangles) {
      const railMat = new THREE.MeshStandardMaterial({
        color: 0x9aa7b8,
        roughness: 0.5,
        metalness: 0.2,
        side: THREE.DoubleSide,
      })
      const positions = new Float32Array(toThreePositions(railingMesh.Vertices))
      const indices = new Uint32Array(railingMesh.Triangles.length * 3)
      railingMesh.Triangles.forEach((t, i) => {
        indices[i * 3] = t[0]
        indices[i * 3 + 1] = t[1]
        indices[i * 3 + 2] = t[2]
      })
      const g = new THREE.BufferGeometry()
      g.setAttribute('position', new THREE.BufferAttribute(positions, 3))
      g.setIndex(new THREE.BufferAttribute(indices, 1))
      g.computeVertexNormals()
      g.computeBoundingBox()
      const m = new THREE.Mesh(g, railMat)
      if (flight === 'straight') {
        mirrorX(g)
      }
      railing = { mesh: m, geo: g }
    }

    // Размещение лестницы у дальней стены/угла помещения (placement.ts):
    // сдвигаем всю группу марша, перила едут вместе с ней.
    let offset = { offsetX: 0, offsetY: 0 }
    const sb = stair.geo.boundingBox
    const ap = approachSpace && approachSpace > 0 ? approachSpace : 1000
    // Габарит стен: периметр ПОМЕЩЕНИЯ (room_mesh в мировых координатах),
    // высота — пользовательская высота марша (heightMM) с фолбэком на верх
    // меша. Без размеров помещения стены не строим (виджет залочен).
    let wallBounds: Box3Like | null = null
    if (sb) {
      wallBounds = wallBoundsOf({
        stairBox: sb,
        roomBounds: room?.geo.boundingBox,
        roomWidth,
        roomLength,
        heightMM,
      })
    }
    if (sb) {
      // Ортокамера теней по габаритам сцены (этап 1): иначе тень либо не
      // попадает в кадр, либо «мылится» на большой лестнице.
      const span = Math.max(
        sb.max.x - sb.min.x,
        sb.max.y - sb.min.y,
        sb.max.z - sb.min.z,
        1000,
      )
      const cam = dir.shadow.camera
      cam.left = -span * 1.2
      cam.right = span * 1.2
      cam.top = span * 1.2
      cam.bottom = -span * 1.2
      cam.updateProjectionMatrix()
    }
    if (sb) {
      let rw = roomWidth ?? 0
      let rl = roomLength ?? 0
      if ((rw <= 0 || rl <= 0) && room && room.geo.boundingBox) {
        const rb = room.geo.boundingBox
        if (rw <= 0) rw = rb.max.x
        // ADR-0008: ширина помещения в three.js — ось Z (Y из API).
        // Берём max.z, а не max.y (высота пола ≈ 0).
        if (rl <= 0) rl = rb.max.z
      }
      // Учитываем подход (EDR-0023) в габарите для ВСЕХ типов марша, чтобы
      // тело+подход вписывались в комнату как единый блок. Дефолт зоны
      // подхода совпадает с бэкендом (1000 мм при 0/отсутствии).
      const approachExt = ap
      offset = computePlacement(
        (flight as 'straight' | 'l_shape' | 'u_shape' | 'spiral') ?? 'straight',
        direction,
        rw,
        rl,
        // ADR-0008: 2D-плоскость placement — X=подъём, Y=ширина.
        // В three.js ширина идёт по Z, поэтому bb по Y берём из sb.min.z/max.z.
        { minX: sb.min.x, minY: sb.min.z, maxX: sb.max.x + approachExt, maxY: sb.max.z },
      )
    }
    const stairGroup = new THREE.Group()
    stairGroup.add(stair.mesh)
    if (railing) stairGroup.add(railing.mesh)
    // ADR-0008: трёхмерные оси — X=подъём(2D +X), Z=ширина(2D +Y), Y=высота.
    // Сдвиг placement.offsetX идёт вдоль подъёма (X), offsetY — вдоль ширины (Z).
    stairGroup.position.set(offset.offsetX, 0, offset.offsetY)
    scene.add(stairGroup)

    const box = stair.geo.boundingBox ?? new THREE.Box3(new THREE.Vector3(), new THREE.Vector3(1, 1, 1))
    // Габарит контекста (стены + плита выхода) в локальных координатах: стены
    // на 60 мм выступают за периметр, плита выхода уходит от верхнего торца на
    // secondFloorDepth. Включаем в кадр всегда (даже при выключенных тумблерах),
    // чтобы кадр не прыгал при переключении.
    const WALL_T = 60
    const SLAB_T = 200
    const ctxMin = new THREE.Vector3(box.min.x, box.min.y, box.min.z)
    const ctxMax = new THREE.Vector3(box.max.x, box.max.y, box.max.z)
    if (sb) {
      // Габарит плиты выхода (для straight/u/spiral вдоль X, для l_shape вдоль Z)
      // + запас на толщину стен (60 мм по периметру). Включаем в кадр всегда
      // (даже при выключенных тумблерах), чтобы кадр не прыгал при кликах.
      const slab = exitSlabBox(sb, flight ?? '', direction, secondFloorDepth, SLAB_T)
      const hs = slab.size.map((v) => v / 2)
      ctxMin.x = Math.min(ctxMin.x, slab.pos[0] - hs[0])
      ctxMin.z = Math.min(ctxMin.z, slab.pos[2] - hs[2])
      ctxMax.x = Math.max(ctxMax.x, slab.pos[0] + hs[0])
      ctxMax.z = Math.max(ctxMax.z, slab.pos[2] + hs[2])
      ctxMin.x -= WALL_T
      ctxMin.z -= WALL_T
      ctxMax.x += WALL_T
      ctxMax.z += WALL_T
    }
    // Кадрируем камеру по объединённому габариту «лестница (со сдвигом) ∪
    // помещение ∪ стены/выход», чтобы было видно, что лестница прижата к
    // нужной стене/углу.
    const unionMin = new THREE.Vector3(ctxMin.x + offset.offsetX, ctxMin.y, ctxMin.z + offset.offsetY)
    const unionMax = new THREE.Vector3(ctxMax.x + offset.offsetX, ctxMax.y, ctxMax.z + offset.offsetY)
    if (room && room.geo.boundingBox) {
      unionMin.min(room.geo.boundingBox.min)
      unionMax.max(room.geo.boundingBox.max)
    }
    const center = unionMin.clone().add(unionMax).multiplyScalar(0.5)
    const radius = Math.max(unionMax.clone().sub(unionMin).length() / 2, 1000)

    // Пол — большая «бесконечная» сетка (визуализация; проверка вписывания
    // в комнату выполняется расчётом независимо). Размер ограничен дальней
    // плоскостью камеры (far = 100000), чтобы сетка не обрезалась.
    const gridSize = Math.min(radius * 30, 90000)
    const gridDiv = Math.min(200, Math.max(20, Math.round(gridSize / 500)))
    const grid = new THREE.GridHelper(gridSize, gridDiv, 0x94a3b8, 0xcdd6e0)
    grid.position.y = unionMin.y
    scene.add(grid)

    // Фиолетовая линия: горизонтально на уровне пола (z = box.min.z), ровно
    // под местом, где заканчивается марш (для straight меш зеркалится по X,
    // поэтому верх — у box.min.x), вдоль его ширины (box.min.y → box.max.y).
    // Показывает ширину марша у верхнего торца.
    // Добавляется в группу марша, чтобы двигалась вместе со сдвигом.
    let topLine: THREE.Line | null = null
    if (stairTop) {
      const x = stairTopLineX(box, flight ?? '', direction)
      const y0 = box.min.y
      const y1 = box.max.y
      const z = box.min.z
      const lg = new THREE.BufferGeometry().setFromPoints([
        new THREE.Vector3(x, y0, z),
        new THREE.Vector3(x, y1, z),
      ])
      topLine = new THREE.Line(lg, new THREE.LineBasicMaterial({ color: 0x9b5de5 }))
      stairGroup.add(topLine)
    }

    // Зона свободного пространства перед первой ступенью (EDR-0023): полупрозрачная
    // площадка на полу перед входом в марш. После зеркалирования меша по X первый
    // шаг прямого марша — на +X, а грань входа (лицевая панель марша, свес первой
    // проступи) отстоит от sb.max.x на StepThickness. Зона начинается строго от
    // грани входа и имеет ширину ровно approachSpace: при комнате roomWidth =
    // Run + approach она упирается ровно в стену П, ничего не вылезая.
    // Добавляем в stairGroup, чтобы зона ехала вместе со сдвигом размещения.
    let approachMesh: THREE.Mesh | null = null
    if (sb) {
      const widthZ = Math.max(1, sb.max.z - sb.min.z)
      const apGeo = new THREE.PlaneGeometry(ap, widthZ)
      const apMat = new THREE.MeshBasicMaterial({
        color: 0xffd43b,
        transparent: true,
        opacity: 0.4,
        side: THREE.DoubleSide,
        depthWrite: false,
      })
      approachMesh = new THREE.Mesh(apGeo, apMat)
      approachMesh.rotation.x = -Math.PI / 2
      approachMesh.position.set(
        approachZoneCenterX(sb, flight ?? '', stepThickness, ap),
        2,
        (sb.min.z + sb.max.z) / 2,
      )
      stairGroup.add(approachMesh)
    }

    // Стены периметра (виджет В/Н/П/Л) и плита «выхода на 2-й этаж»: чисто
    // визуальные элементы контекста; добавляются в stairGroup, чтобы ехали
    // вместе со сдвигом размещения. Тумблеры меняют только .visible, поэтому
    // сцена и камера не пересоздаются при переключении.
    const contextGeos: THREE.BufferGeometry[] = []
    const contextMats: THREE.Material[] = []
    const wallMeshes: Record<WallSide, THREE.Mesh | null> = { top: null, bottom: null, right: null, left: null }
    let exitMesh: THREE.Mesh | null = null

    // Стены периметра: строятся ПО ПЕРИМЕТРУ ПОМЕЩЕНИЯ (мировые координаты,
    // как полупрозрачный периметр room_mesh), а не по габаритам лестницы.
    // В — дальняя стена позади марша, Н — ближняя (перед маршем, внутри
    // помещения), П — правая, Л — левая. Высота — из wallBounds (ввод
    // пользователя «высота марша»). Выходная стена (exitWallSide) доводится
    // до низа блока «выхода на 2-й этаж» (EXIT_BLOCK_H) — блок ложится на неё
    // заподлицо; расчёт статичен и не зависит от чекбокса (плита лишь
    // переключает .visible). Без заданных габаритов помещения стены не
    // строятся, виджет залочен.
    const exitSide = exitWallSide(flight ?? '', direction)
    if (sb && wallBounds) {
      const wallMat = new THREE.MeshStandardMaterial({
        color: 0xaab6c4,
        roughness: 0.9,
        metalness: 0.05,
        transparent: true,
        opacity: 0.55,
        side: THREE.DoubleSide,
      })
      contextMats.push(wallMat)

      const makeWall = (spec: BoxSpec, side: WallSide): THREE.Mesh => {
        const g = new THREE.BoxGeometry(spec.size[0], spec.size[1], spec.size[2])
        const m = new THREE.Mesh(g, wallMat)
        m.position.set(spec.pos[0], spec.pos[1], spec.pos[2])
        contextGeos.push(g)
        m.visible = wallsStateRef.current[side]
        scene.add(m)
        return m
      }

      for (const s of ['top', 'bottom', 'right', 'left'] as WallSide[]) {
        wallMeshes[s] = makeWall(wallBox(wallBounds, s, WALL_T, { side: exitSide, height: EXIT_BLOCK_H }), s)
      }
    }

    if (sb) {
      // Плита «выхода на 2-й этаж»: плоская, глубиной secondFloorDepth, верх
      // на уровне верха марша (sb.max.y). Направление/ось зависят от типа
      // марша (см. exitSlabBox): straight/u_shape — вдоль X, l_shape — вдоль Z.
      const slab = exitSlabBox(sb, flight ?? '', direction, secondFloorDepth, SLAB_T)
      const slabGeo = new THREE.BoxGeometry(slab.size[0], slab.size[1], slab.size[2])
      const slabMat = new THREE.MeshStandardMaterial({
        color: 0x8b98a9,
        roughness: 0.7,
        metalness: 0.1,
        side: THREE.DoubleSide,
      })
      contextGeos.push(slabGeo)
      contextMats.push(slabMat)
      exitMesh = new THREE.Mesh(slabGeo, slabMat)
      exitMesh.position.set(slab.pos[0], slab.pos[1], slab.pos[2])
      exitMesh.visible = showExitStateRef.current
      stairGroup.add(exitMesh)
      exitRef.current = exitMesh
    } else {
      exitRef.current = null
    }

    wallsRef.current = {
      top: wallMeshes.top,
      bottom: wallMeshes.bottom,
      right: wallMeshes.right,
      left: wallMeshes.left,
    }

    // Временная цвето-буквенная разметка периметра и рёбер (debug, scheme-annot.ts):
    // 4 цветные стены комнаты (В/Н/П/Л) + 4 цветных ребра подошвы марша (1В…1Л).
    // ADR-0008: X=подъём(2D +X), Z=ширина(2D +Y). В — дальняя стена (2D +Y → Z max),
    // Н — ближняя (Z min), П — правая (X max), Л — левая (X min).
    const annot: THREE.Object3D[] = []
    if (ANNOTATE) {
      const addAnnot = (
        a: THREE.Vector3,
        b: THREE.Vector3,
        color: string,
        label: string,
        parent: THREE.Object3D,
      ) => {
        const lg = new THREE.BufferGeometry().setFromPoints([a, b])
        const line = new THREE.Line(lg, new THREE.LineBasicMaterial({ color }))
        parent.add(line)
        annot.push(line)
        const spr = makeLabelSprite(label, color)
        spr.position.copy(a.clone().add(b).multiplyScalar(0.5))
        parent.add(spr)
        annot.push(spr)
      }
      if (room && room.geo.boundingBox) {
        const rb = room.geo.boundingBox
        const x0 = rb.min.x, x1 = rb.max.x, z0 = rb.min.z, z1 = rb.max.z, yT = rb.max.y
        addAnnot(new THREE.Vector3(x0, yT, z1), new THREE.Vector3(x1, yT, z1), WALLS.top.color, WALLS.top.key, scene) // В
        addAnnot(new THREE.Vector3(x0, yT, z0), new THREE.Vector3(x1, yT, z0), WALLS.bottom.color, WALLS.bottom.key, scene) // Н
        addAnnot(new THREE.Vector3(x1, yT, z0), new THREE.Vector3(x1, yT, z1), WALLS.right.color, WALLS.right.key, scene) // П
        addAnnot(new THREE.Vector3(x0, yT, z0), new THREE.Vector3(x0, yT, z1), WALLS.left.color, WALLS.left.key, scene) // Л
      }
      const sb2 = stair.geo.boundingBox
      if (sb2) {
        const x0 = sb2.min.x, x1 = sb2.max.x, z0 = sb2.min.z, z1 = sb2.max.z, yF = sb2.min.y
        addAnnot(new THREE.Vector3(x0, yF, z1), new THREE.Vector3(x1, yF, z1), edgeColor(0), '1' + WALLS.top.key, stairGroup) // 1В
        addAnnot(new THREE.Vector3(x0, yF, z0), new THREE.Vector3(x1, yF, z0), edgeColor(1), '1' + WALLS.bottom.key, stairGroup) // 1Н
        addAnnot(new THREE.Vector3(x1, yF, z0), new THREE.Vector3(x1, yF, z1), edgeColor(2), '1' + WALLS.right.key, stairGroup) // 1П
        addAnnot(new THREE.Vector3(x0, yF, z0), new THREE.Vector3(x0, yF, z1), edgeColor(3), '1' + WALLS.left.key, stairGroup) // 1Л
        if (flight === 'spiral') {
          const spr = makeLabelSprite('К', edgeColor(0))
          spr.position.set((x0 + x1) / 2, sb2.max.y + 200, (z0 + z1) / 2)
          stairGroup.add(spr)
          annot.push(spr)
        }
      }
    }

    camera.position.copy(center).add(new THREE.Vector3(radius * 1.4, radius * 1.2, radius * 1.6))
    controls.target.copy(center)
    controls.update()

    // Экономия CPU (P1-7): рендерим только при фактическом изменении, а не в
    // постоянном цикле с фиксированной частотой. OrbitControls помечает
    // needsRender при взаимодействии; в покое лишние кадры не генерируются.
    let needsRender = true
    let lastCapture = 0
    const onChange = () => {
      needsRender = true
    }
    controls.addEventListener('change', onChange)
    renderer.setAnimationLoop(() => {
      controls.update()
      if (!needsRender) return
      needsRender = false
      renderer.render(scene, camera)
      // Кадр для КП сохраняем только ПОСЛЕ фактической отрисовки и не чаще
      // 150 мс при непрерывном вращении — фиксированного PNG-цикла больше нет.
      const now = performance.now()
      if (now - lastCapture >= 150) {
        lastCapture = now
        saveLastFrame()
      }
    })

    const ro = new ResizeObserver(() => {
      const w = container.clientWidth || 600
      const h = container.clientHeight || 380
      camera.aspect = w / h
      camera.updateProjectionMatrix()
      renderer.setSize(w, h)
    })
    ro.observe(container)

    return () => {
      renderer.setAnimationLoop(null)
      ro.disconnect()
      controls.removeEventListener('change', onChange)
      controls.dispose()
      stairMat.dispose()
      stair.geo.dispose()
      if (room) {
        ;(room.mesh.material as THREE.Material).dispose()
        room.geo.dispose()
      }
      if (railing) {
        ;(railing.mesh.material as THREE.Material).dispose()
        railing.geo.dispose()
      }
      renderer.dispose()
      if (topLine) {
        topLine.geometry.dispose()
        ;(topLine.material as THREE.Material).dispose()
      }
      if (approachMesh) {
        approachMesh.geometry.dispose()
        ;(approachMesh.material as THREE.Material).dispose()
      }
      for (const g of contextGeos) g.dispose()
      for (const m of contextMats) m.dispose()
      for (const o of annot) {
        const line = o as THREE.Line
        if (line.geometry) line.geometry.dispose()
        const mat = (o as THREE.Mesh).material as THREE.Material | THREE.Material[] | undefined
        if (Array.isArray(mat)) mat.forEach((m) => m.dispose())
        else if (mat) {
          const map = (mat as THREE.SpriteMaterial).map
          if (map) map.dispose()
          mat.dispose()
        }
      }
      if (renderer.domElement.parentElement === container) {
        container.removeChild(renderer.domElement)
      }
    }
  }, [mesh, roomMesh, railingMesh, approachSpace, roomWidth, roomLength, direction, stairTop, flight, stepThickness, secondFloorDepth, heightMM])

  const SIDES: WallSide[] = ['top', 'bottom', 'right', 'left']
  const WALL_LABELS: Record<WallSide, string> = {
    top: 'Дальняя стена',
    bottom: 'Ближняя стена',
    right: 'Стена справа',
    left: 'Стена слева',
  }
  // Стены строятся по периметру помещения: без заданных габаритов (ширины и
  // длины) строить не из чего, поэтому тумблеры залочены и выводится подсказка.
  const roomSizes = (roomWidth ?? 0) > 0 && (roomLength ?? 0) > 0

  if (webglError) {
    return (
      <div className="viewer viewer--fallback" role="status">
        <p>
          3D-визуализация недоступна в этом браузере: WebGL не поддерживается
          или отключён. Геометрия, габариты и цена остаются доступны в таблицах ниже.
        </p>
      </div>
    )
  }

  return (
    <div className="viewer">
      <div className="viewer__stage" ref={containerRef} />
      <div className="viewer__controls">
        <div className="viewer__walls" role="group" aria-label="Стены">
          {SIDES.map((s) => (
            <button
              key={s}
              type="button"
              disabled={!roomSizes}
              className={`viewer__walls__seg viewer__walls__seg--${s}${walls[s] ? ' viewer__walls__seg--on' : ''}`}
              aria-pressed={walls[s]}
              title={
                roomSizes
                  ? `${WALL_LABELS[s]} (${WALLS[s].key})`
                  : 'Введите габариты помещения, чтобы показать стены'
              }
              onClick={() => setWalls((prev) => ({ ...prev, [s]: !prev[s] }))}
            >
              {WALLS[s].key}
            </button>
          ))}
        </div>
        {!roomSizes && (
          <p className="viewer__walls-lock">Введите габариты помещения (ширину и длину) в калькуляторе, чтобы показать стены.</p>
        )}
        <label className="viewer__exit">
          <input
            type="checkbox"
            checked={showExit}
            onChange={(e) => setShowExit(e.target.checked)}
            title="Плита у верхней ступени — выход на 2-й этаж"
          />
          <span>Выход на 2-й этаж</span>
        </label>
      </div>
      <p className="viewer__hint">Вращение — ЛКМ · панорама — ПКМ/средняя · зум — колесо</p>
    </div>
  )
}
