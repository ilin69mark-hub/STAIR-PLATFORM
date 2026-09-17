// Коммерческое предложение — генерация PDF (клиентский, без себестоимости/раскроя)
// Содержит: шапка, описание, параметры, итоговую цену, 2D и 3D скриншоты.

import { jsPDF } from 'jspdf'
import { notoSansBase64 } from './fonts/notoSans'
import type { Snapshot, Project } from '@shared/types'

const FONT_NAME = 'NotoSans'

function ensureFont(doc: jsPDF) {
  // Статический импорт — без ленивой загрузки, чанк не нужен (гарантия в проде)
  // Каждый new jsPDF() — свой VFS/словарь
  try {
    doc.addFileToVFS('NotoSans-Regular.ttf', notoSansBase64)
  } catch {}
  try {
    // @ts-ignore — Identity-H для кириллицы и ₽
    doc.addFont('NotoSans-Regular.ttf', FONT_NAME, 'normal', 'Identity-H' as any)
  } catch {}
}

export function rub(minor: number | undefined, decimals = 2): string {
  if (minor === undefined || minor === null) return '—'
  const major = minor / 10 ** decimals
  return `${major.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽`
}

export function flightLabel(s: Snapshot): string {
  if (s.spiral) return 'Винтовая'
  if (s.ushape) return 'П-образная'
  if (s.lshape) return 'Г-образная'
  return 'Прямой марш'
}

function tryCaptureCanvas(): string | null {
  // Вариант А: сначала твой ракурс из GeometryViewer (сохранён при каждом кадре)
  try {
    const stored = (window as any).__stairLast3D as string | undefined
    if (stored && stored.startsWith('data:image/png')) return stored
    const sess = sessionStorage.getItem('stairLast3D')
    if (sess && sess.startsWith('data:image/png')) return sess
  } catch {}
  try {
    const canvas = document.querySelector<HTMLCanvasElement>('.viewer__stage canvas')
    if (!canvas) return null
    return canvas.toDataURL('image/png')
  } catch {
    return null
  }
}

function svgElementToDataUrl(svgEl: SVGSVGElement): string | null {
  try {
    const clone = svgEl.cloneNode(true) as SVGSVGElement
    if (!clone.getAttribute('width')) clone.setAttribute('width', String(svgEl.clientWidth || 620))
    if (!clone.getAttribute('height')) clone.setAttribute('height', String(svgEl.clientHeight || 340))
    const serialized = new XMLSerializer().serializeToString(clone)
    const encoded = encodeURIComponent(serialized)
    return `data:image/svg+xml;charset=utf-8,${encoded}`
  } catch {
    return null
  }
}

function tryCaptureSvg(): string | null {
  try {
    const svg = document.querySelector<SVGSVGElement>('.draw svg')
    if (!svg) return null
    return svgElementToDataUrl(svg)
  } catch {
    return null
  }
}

// Офлайн-рендер 2D чертежа без зависимости от активного таба
async function renderOffscreenSvg(snapshot: Snapshot): Promise<string | null> {
  try {
    const { schematicOf } = await import('../components/snapshotView')
    const sch = schematicOf(snapshot)
    if (!sch) return null
    const isStraight = sch.kind === 'straight' && sch.flight
    const [profileMod, planMod] = await Promise.all([
      import('@shared/schemes/StairProfile'),
      import('@shared/schemes/StairPlan'),
    ])
    const React = await import('react')
    const ReactDOM = await import('react-dom/client')
    const container = document.createElement('div')
    container.style.position = 'absolute'
    container.style.left = '-10000px'
    container.style.top = '0'
    container.style.width = '800px'
    container.style.height = '600px'
    document.body.appendChild(container)
    const root = ReactDOM.createRoot(container)
    const el = isStraight
      ? React.createElement((profileMod as any).StairProfile, { flight: sch.flight as any, railing: sch.railing } as any)
      : React.createElement((planMod as any).StairPlan, { flight: sch.flight as any, kind: sch.kind, solver: sch.solver } as any)
    root.render(el)
    // Два rAF + таймаут — гарантируем, что React успел отрендерить SVG
    await new Promise<void>((r) => requestAnimationFrame(() => requestAnimationFrame(() => r())))
    await new Promise((r) => setTimeout(r, 80))
    const svg = container.querySelector('svg') as SVGSVGElement | null
    const dataUrl = svg ? svgElementToDataUrl(svg) : null
    root.unmount()
    container.remove()
    return dataUrl
  } catch {
    return null
  }
}

// Офлайн-рендер 3D без активной вкладки — один кадр offscreen WebGL
async function renderOffscreen3D(snapshot: Snapshot): Promise<string | null> {
  if (!snapshot.mesh || snapshot.mesh.Vertices.length === 0) return null
  let container: HTMLDivElement | null = null
  let renderer: any = null
  try {
    const THREE: any = await import('three')
    const { toThreePositions } = await import('@shared/viewer/projection')
    const width = 1200, height = 800
    container = document.createElement('div')
    container.style.position = 'absolute'
    container.style.left = '-10000px'
    container.style.top = '0'
    container.style.width = width + 'px'
    container.style.height = height + 'px'
    document.body.appendChild(container)
    const scene = new THREE.Scene()
    scene.background = new THREE.Color('#f7f9fc')
    const camera = new THREE.PerspectiveCamera(45, width / height, 1, 100000)
    renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true, alpha: false } as any)
    renderer.setSize(width, height)
    renderer.setPixelRatio(1)
    container.appendChild(renderer.domElement)
    scene.add(new THREE.HemisphereLight(0xffffff, 0xbfc8d8, 1))
    const dir = new THREE.DirectionalLight(0xffffff, 1.4)
    dir.position.set(2000, 4000, 3000)
    scene.add(dir)
    const dir2 = new THREE.DirectionalLight(0xffffff, 0.5)
    dir2.position.set(-2000, -1000, -3000)
    scene.add(dir2)
    const makeMesh = (api: any, material: any) => {
      const positions = new Float32Array(toThreePositions(api.Vertices))
      const indices = new Uint32Array(api.Triangles.length * 3)
      api.Triangles.forEach((t: number[], i: number) => {
        indices[i * 3] = t[0]; indices[i * 3 + 1] = t[1]; indices[i * 3 + 2] = t[2]
      })
      const g = new THREE.BufferGeometry()
      g.setAttribute('position', new THREE.BufferAttribute(positions, 3))
      g.setIndex(new THREE.BufferAttribute(indices, 1))
      g.computeVertexNormals()
      g.computeBoundingBox()
      const m = new THREE.Mesh(g, material)
      return { mesh: m, geo: g }
    }
    const stairMat = new THREE.MeshStandardMaterial({ color: 0x4f8df7, roughness: 0.55, metalness: 0.12, side: THREE.DoubleSide } as any)
    const stair = makeMesh(snapshot.mesh, stairMat)
    // зеркалим прямой марш как в GeometryViewer
    const flightKind = snapshot.spiral ? 'spiral' : snapshot.ushape ? 'ushape' : snapshot.lshape ? 'lshape' : 'straight'
    if (flightKind === 'straight') {
      const pos = stair.geo.attributes.position as any
      for (let i = 0; i < pos.count; i++) pos.setX(i, -pos.getX(i))
      pos.needsUpdate = true
      stair.geo.computeBoundingBox()
    }
    scene.add(stair.mesh)
    if (snapshot.room_mesh && snapshot.room_mesh.Vertices.length > 0) {
      const roomMat = new THREE.MeshStandardMaterial({ color: 0xffa94d, roughness: 0.9, transparent: true, opacity: 0.32, side: THREE.DoubleSide } as any)
      const room = makeMesh(snapshot.room_mesh, roomMat)
      scene.add(room.mesh)
    }
    if (snapshot.railing_mesh && snapshot.railing_mesh.Vertices.length > 0) {
      const railMat = new THREE.MeshStandardMaterial({ color: 0x9aa7b8, roughness: 0.5, side: THREE.DoubleSide } as any)
      const positions = new Float32Array(toThreePositions(snapshot.railing_mesh.Vertices))
      const indices = new Uint32Array(snapshot.railing_mesh.Triangles.length * 3)
      snapshot.railing_mesh.Triangles.forEach((t: number[], i: number) => { indices[i*3]=t[0]; indices[i*3+1]=t[1]; indices[i*3+2]=t[2] })
      const g = new THREE.BufferGeometry()
      g.setAttribute('position', new THREE.BufferAttribute(positions,3))
      g.setIndex(new THREE.BufferAttribute(indices,1))
      g.computeVertexNormals()
      g.computeBoundingBox()
      if (flightKind === 'straight') {
        const p = g.attributes.position as any
        for (let i=0;i<p.count;i++) p.setX(i, -p.getX(i))
        p.needsUpdate=true
      }
      scene.add(new THREE.Mesh(g, railMat))
    }
    // камера — вид как в GeometryViewer (изометрия)
    const box = stair.geo.boundingBox!
    const center = new THREE.Vector3(); box.getCenter(center)
    const size = new THREE.Vector3(); box.getSize(size)
    const maxDim = Math.max(size.x, size.y, size.z, 1000)
    camera.position.set(center.x + maxDim * 1.2, center.y - maxDim * 1.1, center.z + maxDim * 1.3)
    camera.lookAt(center)
    camera.updateMatrixWorld()
    renderer.render(scene, camera)
    const dataUrl = renderer.domElement.toDataURL('image/png')
    renderer.dispose()
    scene.traverse((obj: any) => { if (obj.geometry) obj.geometry.dispose(); if (obj.material) obj.material.dispose() })
    if (container && container.parentNode) container.remove()
    return dataUrl
  } catch {
    try { if (renderer) renderer.dispose() } catch {}
    try { if (container && container.parentNode) container.remove() } catch {}
    return tryCaptureCanvas()
  }
}

async function getImageSize(dataUrl: string): Promise<{ w: number; h: number } | null> {
  return new Promise((resolve) => {
    const img = new Image()
    img.onload = () => resolve({ w: img.width, h: img.height })
    img.onerror = () => resolve(null)
    img.src = dataUrl
  })
}

async function svgDataUrlToPng(svgDataUrl: string, width = 1200, height = 600): Promise<string | null> {
  return new Promise((resolve) => {
    const img = new Image()
    img.onload = () => {
      try {
        const canvas = document.createElement('canvas')
        canvas.width = width
        canvas.height = height
        const ctx = canvas.getContext('2d')
        if (!ctx) { resolve(null); return }
        ctx.fillStyle = '#ffffff'
        ctx.fillRect(0, 0, width, height)
        // contain
        const scale = Math.min(width / img.width, height / img.height)
        const w = img.width * scale
        const h = img.height * scale
        const x = (width - w) / 2
        const y = (height - h) / 2
        ctx.drawImage(img, x, y, w, h)
        resolve(canvas.toDataURL('image/png'))
      } catch { resolve(null) }
    }
    img.onerror = () => resolve(null)
    img.src = svgDataUrl
  })
}

export async function generateProposalPdf(project: Project, snapshot: Snapshot): Promise<void> {
  const doc = new jsPDF({ unit: 'mm', format: 'a4', orientation: 'portrait' })
  ensureFont(doc)
  doc.setFont(FONT_NAME, 'normal')

  const pageW = 210
  const margin = 14
  const contentW = pageW - margin * 2
  let y = 14

  // Заголовок
  doc.setFontSize(22)
  doc.setTextColor(20, 30, 60)
  doc.text('Коммерческое предложение', margin, y)
  y += 8
  doc.setDrawColor(79, 141, 247)
  doc.setLineWidth(0.6)
  doc.line(margin, y, pageW - margin, y)
  y += 7

  // Дата
  const dateStr = new Date().toLocaleDateString('ru-RU', { day: '2-digit', month: 'long', year: 'numeric' })
  doc.setFontSize(8)
  doc.setTextColor(110, 120, 140)
  doc.text(`Дата: ${dateStr}`, margin, y)
  y += 6

  // Название проекта
  doc.setFontSize(14)
  doc.setTextColor(20, 30, 60)
  const nameLines = doc.splitTextToSize(project.name || 'Проект', contentW)
  doc.text(nameLines, margin, y)
  y += nameLines.length * 6 + 2

  // Описание
  doc.setFontSize(9)
  doc.setTextColor(70, 70, 70)
  const desc = (project.description && project.description.trim()) ? project.description.trim() : '—'
  const descLines = doc.splitTextToSize(`Описание: ${desc}`, contentW)
  doc.text(descLines, margin, y)
  y += descLines.length * 4 + 6

  // Параметры — таблица
  doc.setFontSize(10)
  doc.setTextColor(20, 30, 60)
  doc.text('Параметры лестницы', margin, y)
  y += 5
  doc.setFontSize(8)
  doc.setTextColor(50, 50, 50)

  const params: Array<[string, string]> = []
  params.push(['Тип марша', flightLabel(snapshot)])
  if (snapshot.flight) {
    params.push(['Ступеней', String(snapshot.flight.StepCount)])
    params.push(['Высота ступени', `${snapshot.flight.StepHeight.toLocaleString('ru-RU')} мм`])
    params.push(['Глубина проступи', `${snapshot.flight.TreadDepth.toLocaleString('ru-RU')} мм`])
    params.push(['Угол наклона', `${(snapshot.flight.Angle * 180 / Math.PI).toFixed(1)}°`])
    params.push(['Длина марша', `${snapshot.flight.Run.toLocaleString('ru-RU')} мм`])
  } else if (snapshot.lshape) {
    params.push(['Ступеней всего', String(snapshot.lshape.StepCount)])
    params.push(['Нижний марш', String(snapshot.lshape.LowerStepCount)])
  } else if (snapshot.ushape) {
    params.push(['Ступеней всего', String(snapshot.ushape.StepCount)])
  } else if (snapshot.spiral) {
    params.push(['Ступеней', String(snapshot.spiral.StepCount)])
  }
  if (snapshot.validation?.Valid !== undefined) {
    params.push(['Статус расчёта', snapshot.validation.Valid ? 'Корректен' : 'Есть замечания'])
  }

  const col1 = 48
  const rowH = 5.2
  // фон шапки таблицы
  doc.setFillColor(244, 246, 250)
  doc.rect(margin, y - 3.5, contentW, 6, 'F')
  doc.setFontSize(7)
  doc.setTextColor(110, 120, 140)
  doc.text('ПАРАМЕТР', margin + 2, y)
  doc.text('ЗНАЧЕНИЕ', margin + col1 + 2, y)
  y += 4.5
  doc.setFontSize(8)
  for (const [k, v] of params) {
    if (y > 272) { doc.addPage(); y = 14; doc.setFont(FONT_NAME, 'normal') }
    doc.setDrawColor(232, 235, 240)
    doc.line(margin, y + 1.2, pageW - margin, y + 1.2)
    doc.setTextColor(90, 90, 90)
    doc.text(k, margin + 2, y)
    doc.setTextColor(20, 30, 60)
    doc.text(String(v), margin + col1 + 2, y)
    y += rowH
  }
  y += 4

  // Стоимость — акцентный блок
  if (y > 240) { doc.addPage(); y = 14; doc.setFont(FONT_NAME, 'normal') }
  doc.setFillColor(79, 141, 247)
  doc.roundedRect(margin, y, contentW, 18, 2, 2, 'F')
  doc.setTextColor(255, 255, 255)
  doc.setFontSize(9)
  doc.text('Итоговая стоимость', margin + 5, y + 7)
  doc.setFontSize(16)
  const priceStr = snapshot.pricing ? rub(snapshot.pricing.FinalPrice, snapshot.pricing.Currency.Decimals) : '—'
  doc.text(priceStr, margin + 5, y + 13)
  doc.setFontSize(7)
  doc.setTextColor(220, 230, 255)
  const cur = snapshot.pricing?.Currency.Code || 'RUB'
  doc.text(cur, pageW - margin - doc.getTextWidth(cur) - 5, y + 13)
  y += 26

  // Чертежи — расслабленно, на отдельной странице без сжатия (оба помещаются)
  doc.addPage()
  y = 14
  // 2D — на всю ширину, без рамки и contain
  doc.setTextColor(20, 30, 60)
  doc.setFontSize(10)
  doc.text('Чертёж (вид сверху / профиль)', margin, y)
  y += 6
  {
    let svgRaw = tryCaptureSvg()
    if (!svgRaw) svgRaw = await renderOffscreenSvg(snapshot)
    if (svgRaw) {
      let png: string | null = null
      if (svgRaw.startsWith('data:image/svg+xml')) {
        png = await svgDataUrlToPng(svgRaw, 2400, 1240)
      } else {
        png = svgRaw.startsWith('data:image/png') ? svgRaw : null
      }
      if (png) {
        // без сжатия: ширина = contentW, высота по пропорции 2400×1240 (~94мм)
        const h = (contentW * 1240) / 2400
        doc.addImage(png, 'PNG', margin, y, contentW, h)
        y += h + 8
      } else {
        doc.setFontSize(8); doc.setTextColor(140,140,140)
        doc.text('Не удалось сформировать изображение чертежа.', margin, y); y += 6
      }
    } else {
      doc.setFontSize(8); doc.setTextColor(140,140,140)
      doc.text('Чертёж недоступен для этого расчёта.', margin, y, { maxWidth: contentW })
      y += 10
    }
  }

  // 3D — твой ракурс (вариант А), как на экране, без растяжения
  doc.setTextColor(20, 30, 60)
  doc.setFontSize(10)
  doc.text('3D-модель', margin, y)
  y += 6
  {
    let canvasRaw: string | null = tryCaptureCanvas()
    if (!canvasRaw) canvasRaw = await renderOffscreen3D(snapshot)
    if (canvasRaw) {
      const size = await getImageSize(canvasRaw)
      // Сохраняем пропорции как на экране (обычно ~880×380 → 2.31), не тянем
      const h = size ? (contentW * size.h) / size.w : (contentW * 380) / 880
      // Ограничиваем чтобы не вылезти за страницу, но без растяжения
      const maxH = 285 - y
      const finalH = Math.min(h, maxH)
      const finalW = size ? (finalH * size.w) / size.h : contentW
      const x = margin + (contentW - finalW) / 2
      doc.addImage(canvasRaw, 'PNG', x, y, finalW, finalH)
      y += finalH + 6
    } else {
      doc.setFontSize(8); doc.setTextColor(140,140,140)
      doc.text('3D-модель недоступна для этого расчёта.', margin, y, { maxWidth: contentW })
      y += 10
    }
  }

  // Футер на последней странице — мелкий дисклеймер
  doc.setFontSize(6)
  doc.setTextColor(150, 160, 180)
  const foot = 'Коммерческое предложение сформировано автоматически на основе расчёта. Стоимость ориентировочная.'
  doc.text(foot, margin, 287)

  // Нумерация уже не нужна — jsPDF нумерует по страницам, но добавим на каждую
  const totalPages = doc.getNumberOfPages()
  for (let i = 1; i <= totalPages; i++) {
    doc.setPage(i)
    doc.setFontSize(6)
    doc.setTextColor(150, 160, 180)
    doc.text(`${i} / ${totalPages}`, pageW - margin - 10, 287)
  }

  const safeName = (project.name || 'project').replace(/[^\w\-а-яА-Яёё]+/g, '_').slice(0, 40) || 'project'
  doc.save(`${safeName}-kp.pdf`)
}
