import { describe, expect, it } from 'vitest'
import { jsPDF } from 'jspdf'

import { parsePoints, parsePolylinePath, drawSvgVector, type VectorDoc } from './svgVector'

describe('parsePolylinePath', () => {
  it('M с последующими L даёт относительные сегменты', () => {
    const { start, segs } = parsePolylinePath('M 10 20 L 30 40 L 50 60')
    expect(start).toEqual([10, 20])
    // Сегменты относительные, как того ждёт doc.lines.
    expect(segs).toEqual([
      [20, 20, 0, 0],
      [20, 20, 0, 0],
    ])
  })

  it('несколько подпутей начинают отсчёт заново', () => {
    const { start, segs } = parsePolylinePath('M 0 0 L 10 0 M 50 50 L 60 50')
    expect(start).toEqual([0, 0])
    expect(segs).toEqual([
      [10, 0, 0, 0],
      [10, 0, 0, 0],
    ])
  })

  it('пустой и мусорный путь не роняют разбор', () => {
    expect(parsePolylinePath('').segs).toEqual([])
    expect(parsePolylinePath('M 1 1').segs).toEqual([])
    expect(parsePolylinePath('совсем не путь').segs).toEqual([])
  })
})

describe('parsePoints', () => {
  it('читает пары координат с пробелами и запятыми', () => {
    expect(parsePoints('1,2 3,4')).toEqual([1, 2, 3, 4])
  })

  it('пустая строка даёт пустой массив', () => {
    expect(parsePoints('')).toEqual([])
    expect(parsePoints('   ')).toEqual([])
  })
})

/** Минимальный документ: считает вызовы, ничего не рисуя. */
function fakeDoc() {
  const calls = { lines: [] as number[][], rect: [] as number[][], line: [] as number[][], text: [] as string[] }
  const doc: VectorDoc = {
    setLineWidth: () => {},
    setDrawColor: () => {},
    setTextColor: () => {},
    setFontSize: () => {},
    line: (x1, y1, x2, y2) => {
      calls.line.push([x1, y1, x2, y2])
    },
    rect: (x, y, w, h) => {
      calls.rect.push([x, y, w, h])
    },
    text: (t) => {
      calls.text.push(t)
    },
  }
  return { doc, calls }
}

function makeSvg(inner: string, vb = '0 0 100 50'): SVGSVGElement {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
  svg.setAttribute('viewBox', vb)
  svg.innerHTML = inner
  return svg
}

describe('drawSvgVector', () => {
  it('высота вставки идёт по пропорции viewBox', () => {
    const { doc } = fakeDoc()
    // viewBox 100×50, ширина вставки 100 мм → высота 50 мм.
    const h = drawSvgVector(doc, makeSvg(''), 0, 0, 100)
    expect(h).toBeCloseTo(50, 6)
  })

  it('линия: координата от верха уходит как есть', () => {
    const { doc, calls } = fakeDoc()
    drawSvgVector(doc, makeSvg('<line x1="0" y1="0" x2="100" y2="50" class="scheme__line"/>'), 0, 0, 100)
    // Проверено на распакованном content stream настоящего jsPDF: line()
    // переворачивает ось Y сам (переданные 50 мм уходят в файл как 297 − 50).
    // Поэтому свой переворот здесь дал бы двойной переворот — чертёж вверх ногами.
    expect(calls.line).toEqual([[0, 0, 100, 50]])
  })

  it('прямоугольник ложится по левому верхнему углу', () => {
    const { doc, calls } = fakeDoc()
    drawSvgVector(doc, makeSvg('<rect x="10" y="10" width="20" height="30" class="scheme__outline"/>'), 0, 0, 100)
    expect(calls.rect).toHaveLength(1)
    const [rx, ry, rw, rh] = calls.rect[0]
    // Масштаб 1 (viewBox 100 → 100 мм), координаты от верхнего левого угла.
    expect(rx).toBeCloseTo(10, 6)
    expect(ry).toBeCloseTo(10, 6)
    expect(rw).toBeCloseTo(20, 6)
    expect(rh).toBeCloseTo(30, 6)
  })

  it('polyline рисуется отрезками по порядку точек', () => {
    const { doc, calls } = fakeDoc()
    drawSvgVector(doc, makeSvg('<polyline points="10,10 50,40 90,20" class="scheme__line"/>'), 0, 0, 100)
    expect(calls.line).toEqual([
      [10, 10, 50, 40],
      [50, 40, 90, 20],
    ])
  })

  it('path из M/L даёт ту же геометрию, что polyline', () => {
    const { doc, calls } = fakeDoc()
    drawSvgVector(doc, makeSvg('<path d="M 10 10 L 50 40 L 90 20" class="scheme__line"/>'), 0, 0, 100)
    expect(calls.line).toEqual([
      [10, 10, 50, 40],
      [50, 40, 90, 20],
    ])
  })

  it('текст попадает в документ', () => {
    const { doc, calls } = fakeDoc()
    drawSvgVector(doc, makeSvg('<text x="5" y="10" class="scheme__label">2700 мм</text>'), 0, 0, 100)
    expect(calls.text).toEqual(['2700 мм'])
  })

  it('пустой SVG не падает и даёт нулевые вызовы', () => {
    const { doc, calls } = fakeDoc()
    const h = drawSvgVector(doc, makeSvg(''), 0, 0, 100)
    expect(h).toBeCloseTo(50, 6)
    expect(calls.line).toHaveLength(0)
    expect(calls.rect).toHaveLength(0)
    expect(calls.text).toHaveLength(0)
  })

  it('SVG без viewBox берёт размеры из width/height', () => {
    const { doc } = fakeDoc()
    const svg = makeSvg('', '')
    svg.removeAttribute('viewBox')
    svg.setAttribute('width', '200')
    svg.setAttribute('height', '100')
    // 200×100 при ширине вставки 100 мм → высота 50 мм.
    expect(drawSvgVector(doc, svg, 0, 0, 100)).toBeCloseTo(50, 6)
  })
})

/**
 * Единственный тест на НАСТОЯЩЕМ jsPDF.
 *
 * Остальные проверяют моку, а весь баг S-177 был именно в расхождении моки и
 * реальности: doc.line() переворачивает ось Y сам, doc.rect() и doc.text() —
 * нет. На моке это не видно. Здесь читается распакованный content stream
 * готового PDF-файла — то есть проверяется то, что реально уйдёт клиенту.
 */
describe('настоящий jsPDF', () => {
  it('верх схемы попадает на верх страницы, а не наоборот', () => {
    const doc = new jsPDF({ unit: 'mm', compress: false })
    const svg = makeSvg('<line x1="0" y1="0" x2="100" y2="50" class="scheme__line"/>')
    drawSvgVector(doc as unknown as VectorDoc, svg, 0, 0, 100)

    const out = doc.output()
    const body = out.slice(out.indexOf('stream'), out.indexOf('endstream'))
    // В PDF координата Y отсчитывается от НИЗА страницы, поэтому верх области
    // — это максимум (297 мм = 841.89 pt), а низ — минимум.
    const ys = (body.match(/[\d.]+ [\d.]+ [ml]/g) ?? []).map((op) => Number(op.split(' ')[1]))
    expect(ys.length).toBeGreaterThanOrEqual(2)
    expect(Math.max(...ys)).toBeCloseTo(841.89, 1)
    expect(Math.min(...ys)).toBeCloseTo(700.16, 1)
  })
})