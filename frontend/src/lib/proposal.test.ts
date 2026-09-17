// Тесты генератора КП: jspdf/three/React/DOM мокаются полностью.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const h = vi.hoisted(() => {
  const state = {
    schematic: null as any,
    failScene: false,
    docs: [] as any[],
  }
  return state
})

vi.mock('./fonts/notoSans', () => ({ notoSansBase64: 'AAAA' }))

vi.mock('jspdf', () => {
  class JsPDF {
    calls: Array<[string, ...any[]]> = []
    page = 1
    __splitMany: string | null = null
    opts?: any
    constructor(opts?: any) {
      this.opts = opts
      h.docs.push(this)
    }
    addFileToVFS(...a: any[]) { this.calls.push(['addFileToVFS', ...a]) }
    addFont(...a: any[]) { this.calls.push(['addFont', ...a]) }
    setFont(...a: any[]) { this.calls.push(['setFont', ...a]) }
    setFontSize(...a: any[]) { this.calls.push(['setFontSize', ...a]) }
    setTextColor(...a: any[]) { this.calls.push(['setTextColor', ...a]) }
    setDrawColor(...a: any[]) { this.calls.push(['setDrawColor', ...a]) }
    setFillColor(...a: any[]) { this.calls.push(['setFillColor', ...a]) }
    setLineWidth(...a: any[]) { this.calls.push(['setLineWidth', ...a]) }
    line(...a: any[]) { this.calls.push(['line', ...a]) }
    rect(...a: any[]) { this.calls.push(['rect', ...a]) }
    roundedRect(...a: any[]) { this.calls.push(['roundedRect', ...a]) }
    addImage(...a: any[]) { this.calls.push(['addImage', ...a]) }
    addPage() { this.page++; this.calls.push(['addPage']) }
    setPage(n: number) { this.page = n }
    text(...a: any[]) { this.calls.push(['text', ...a]) }
    getTextWidth() { return 10 }
    getNumberOfPages() { return 3 }
    splitTextToSize(text: string) {
      if (typeof text === 'string' && text.includes('MANYDESC')) return Array(60).fill('line')
      if (typeof text === 'string' && text.includes('MANYNAME')) return Array(60).fill('line')
      return Array.isArray(text) ? text : [text]
    }
    save(name: string) { this.calls.push(['save', name]) }
  }
  return { jsPDF: JsPDF }
})

vi.mock('react', () => ({ createElement: () => null }))
vi.mock('react-dom/client', () => ({ createRoot: () => ({ render: vi.fn(), unmount: vi.fn() }) }))
vi.mock('@shared/schemes/StairProfile', () => ({ StairProfile: () => null }))
vi.mock('@shared/schemes/StairPlan', () => ({ StairPlan: () => null }))
vi.mock('@shared/viewer/projection', () => ({
  toThreePositions: (v: number[]) => new Float32Array(v.length * 3),
}))
vi.mock('../components/snapshotView', () => ({ schematicOf: () => h.schematic }))

vi.mock('three', () => {
  class Vector3 {
    x = 0; y = 0; z = 0
    set(x: number, y: number, z: number) { this.x = x; this.y = y; this.z = z; return this }
  }
  class BufferAttribute {
    needsUpdate = false
    data: any
    itemSize: number
    constructor(data: any, itemSize: number) { this.data = data; this.itemSize = itemSize }
    get count() { return 2 }
    setX() {} getX() { return 0 }
  }
  class BufferGeometry {
    attributes: any = { position: { count: 2, setX() {}, getX() { return 0 }, needsUpdate: false } }
    boundingBox: any = { getCenter(c: any) { c.x = 0; c.y = 0; c.z = 0 }, getSize(s: any) { s.x = 100; s.y = 100; s.z = 100 } }
    setAttribute(name: string, attr: any) { this.attributes[name] = attr; return this }
    setIndex() { return this }
    computeVertexNormals() {}
    computeBoundingBox() {}
  }
  class Scene {
    background: any = null
    constructor() { if (h.failScene) throw new Error('scene boom') }
    add() {}
    traverse(cb: any) { cb({ geometry: undefined, material: undefined }) }
  }
  class PerspectiveCamera {
    position = { set() {} }
    lookAt() {}
    updateMatrixWorld() {}
  }
  class WebGLRenderer {
    domElement = { toDataURL: () => 'data:image/png;base64,AAA' }
    setSize() {}
    setPixelRatio() {}
    render() {}
    dispose() {}
  }
  class Mesh {
    geometry: any
    material: any
    constructor(geometry: any, material: any) { this.geometry = geometry; this.material = material }
  }
  class MeshStandardMaterial {}
  class HemisphereLight {}
  class DirectionalLight { position = { set() {} } }
  class Color {}
  return {
    Scene, PerspectiveCamera, WebGLRenderer, Mesh, MeshStandardMaterial,
    HemisphereLight, DirectionalLight, Color, BufferAttribute, BufferGeometry,
    Vector3, DoubleSide: 2,
  }
})

import { generateProposalPdf, flightLabel, rub } from './proposal'

class FakeImage {
  onload: (() => void) | null = null
  onerror: (() => void) | null = null
  width = 100
  height = 50
  set src(_v: string) { queueMicrotask(() => this.onload && this.onload()) }
}

function snap(over: any = {}): any {
  return { project_id: 'p-1', ...over }
}

function project(name = 'Проект', description = 'desc'): any {
  return { id: 'p-1', name, description, status: 'draft' }
}

function mesh(vertices: number[] = [0, 0, 0]) {
  return { Vertices: vertices, Triangles: [[0, 0, 0]] }
}

beforeEach(() => {
  h.schematic = null
  h.failScene = false
  h.docs.length = 0
  vi.stubGlobal('Image', FakeImage as any)
  vi.stubGlobal('requestAnimationFrame', (cb: any) => setTimeout(cb, 0) as any)
  HTMLCanvasElement.prototype.getContext = vi.fn(() => ({ fillStyle: '', fillRect: vi.fn(), drawImage: vi.fn() })) as any
  HTMLCanvasElement.prototype.toDataURL = vi.fn(() => 'data:image/png;base64,CANVAS') as any
})

afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
  delete (window as any).__stairLast3D
})

describe('rub / flightLabel', () => {
  it('formats rubles and handles missing values', () => {
    expect(rub(undefined)).toBe('—')
    expect(rub(123456)).toContain('₽')
    expect(rub(100, 0)).toContain('₽')
  })

  it('labels every flight type', () => {
    expect(flightLabel(snap({ spiral: {} }))).toBe('Винтовая')
    expect(flightLabel(snap({ ushape: {} }))).toBe('П-образная')
    expect(flightLabel(snap({ lshape: {} }))).toBe('Г-образная')
    expect(flightLabel(snap())).toBe('Прямой марш')
  })
})

describe('generateProposalPdf', () => {
  it('renders a straight flight with room/railing meshes and saved 3D canvas', async () => {
    h.schematic = { kind: 'straight', flight: { StepCount: 10 } }
    ;(window as any).__stairLast3D = 'data:image/png;base64,STORED'
    const snapshot = snap({
      flight: { StepCount: 12, StepHeight: 180, TreadDepth: 280, Angle: 0.5, Run: 3000 },
      validation: { Valid: true },
      pricing: { FinalPrice: 123400, Currency: { Decimals: 2, Code: 'RUB' } },
      mesh: mesh(),
      room_mesh: mesh(),
      railing_mesh: mesh(),
    })
    await generateProposalPdf(project('MANYNAME'), snapshot)
    const doc = h.docs.at(-1)
    expect(doc.calls.some((c: any[]) => c[0] === 'save')).toBe(true)
    expect(doc.calls.some((c: any[]) => c[0] === 'addImage')).toBe(true)
    expect(doc.calls.filter((c: any[]) => c[0] === 'addPage').length).toBeGreaterThan(0)
  })

  it('falls back to offscreen SVG rendering (plan kind) and offscreen 3D', async () => {
    h.schematic = { kind: 'lshape', flight: {}, solver: {} }
    const snapshot = snap({
      lshape: { StepCount: 8, LowerStepCount: 3 },
      validation: { Valid: false },
      mesh: mesh(),
    })
    await generateProposalPdf(project('', ''), snapshot)
    expect(h.docs.at(-1).calls.some((c: any[]) => c[0] === 'save')).toBe(true)
  })

  it('once more with straight offscreen SVG', async () => {
    h.schematic = { kind: 'straight', flight: { StepCount: 5 }, railing: {} }
    await generateProposalPdf(project(), snap({ mesh: mesh() }))
    expect(h.docs.at(-1).calls.some((c: any[]) => c[0] === 'save')).toBe(true)
  })

  it('handles ushape, spiral and plain flights', async () => {
    await generateProposalPdf(project(), snap({ ushape: { StepCount: 6 } }))
    await generateProposalPdf(project(), snap({ spiral: { StepCount: 7 } }))
    await generateProposalPdf(project(), snap())
    expect(h.docs.length).toBe(3)
  })

  it('paginates when text and params overflow', async () => {
    await generateProposalPdf(project('Проект', 'MANYDESC'), snap({
      flight: { StepCount: 3, StepHeight: 100, TreadDepth: 100, Angle: 0.1, Run: 100 },
      mesh: mesh(),
    }))
    expect(h.docs.at(-1).calls.filter((c: any[]) => c[0] === 'addPage').length).toBeGreaterThan(1)
  })

  it('falls back to canvas capture when offscreen 3D throws', async () => {
    h.failScene = true
    ;(window as any).__stairLast3D = 'data:image/png;base64,STORED'
    await generateProposalPdf(project(), snap({ mesh: mesh() }))
    expect(h.docs.at(-1).calls.some((c: any[]) => c[0] === 'addImage')).toBe(true)
  })

  it('reports unavailable 3D when mesh is empty', async () => {
    await generateProposalPdf(project(), snap({ mesh: { Vertices: [], Triangles: [] } }))
    const texts = h.docs.at(-1).calls.filter((c: any[]) => c[0] === 'text').map((c: any[]) => c[1])
    expect(texts.flat().join(' ')).toContain('3D-модель недоступна')
  })

  it('captures an inline SVG drawing when present', async () => {
    const draw = document.createElement('div')
    draw.className = 'draw'
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
    draw.appendChild(svg)
    document.body.appendChild(draw)
    await generateProposalPdf(project(), snap({ mesh: mesh() }))
    expect(h.docs.at(-1).calls.some((c: any[]) => c[0] === 'addImage')).toBe(true)
  })

  it('reads 3D capture from sessionStorage', async () => {
    sessionStorage.setItem('stairLast3D', 'data:image/png;base64,SESS')
    await generateProposalPdf(project(), snap({ mesh: mesh() }))
    expect(h.docs.at(-1).calls.some((c: any[]) => c[0] === 'addImage')).toBe(true)
  })

  it('saves without pricing using the fallback file name', async () => {
    await generateProposalPdf(project('', ''), snap({ mesh: mesh(), validation: {} }))
    const saveCall = h.docs.at(-1).calls.find((c: any[]) => c[0] === 'save')
    expect(saveCall[1]).toBe('project-kp.pdf')
  })
})
