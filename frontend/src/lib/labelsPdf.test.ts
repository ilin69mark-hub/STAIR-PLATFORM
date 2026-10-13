// Тесты листа наклеек PDF: вёрстка на моке jsPDF (паттерн proposal.test.ts).
import { describe, expect, it, vi } from 'vitest'

vi.mock('./fonts/notoSans', () => ({ notoSansBase64: 'AAAA' }))

const h = vi.hoisted(() => ({ docs: [] as any[] }))

vi.mock('jspdf', () => {
  class JsPDF {
    calls: Array<[string, ...any[]]> = []
    constructor() {
      h.docs.push(this)
    }
    addFileToVFS(...a: any[]) { this.calls.push(['addFileToVFS', ...a]) }
    addFont(...a: any[]) { this.calls.push(['addFont', ...a]) }
    setFont(...a: any[]) { this.calls.push(['setFont', ...a]) }
    setFontSize(...a: any[]) { this.calls.push(['setFontSize', ...a]) }
    setTextColor(...a: any[]) { this.calls.push(['setTextColor', ...a]) }
    setDrawColor(...a: any[]) { this.calls.push(['setDrawColor', ...a]) }
    setLineWidth(...a: any[]) { this.calls.push(['setLineWidth', ...a]) }
    rect(...a: any[]) { this.calls.push(['rect', ...a]) }
    addPage() { this.calls.push(['addPage']) }
    text(...a: any[]) { this.calls.push(['text', ...a]) }
    splitTextToSize(text: string) {
      return Array.isArray(text) ? text : [text]
    }
    save(name: string) { this.calls.push(['save', name]) }
  }
  return { jsPDF: JsPDF }
})

import { buildLabelsPdf, downloadLabelsPdf, stickersPerPage } from './labelsPdf'
import type { Part } from '@shared/types'

function part(n: string, where = 'ступень 1'): Part {
  return {
    Number: n, Kind: 'tread', Material: 'WOOD-OAK',
    Thickness: 40, Length: 900, Width: 290, SolidIndex: 0, Where: where,
  }
}

function callsOf(doc: any, name: string) {
  return doc.calls.filter((c: any[]) => c[0] === name)
}

describe('labelsPdf', () => {
  it('одна наклейка = одна рамка с номером', async () => {
    const { jsPDF } = await import('jspdf')
    const doc = new jsPDF()
    buildLabelsPdf(doc, [part('TRD-01'), part('TRD-02')])
    expect(callsOf(doc, 'rect')).toHaveLength(2)
    const texts = callsOf(doc, 'text').map((c: any[]) => c[1])
    expect(texts).toContain('TRD-01')
    expect(texts).toContain('TRD-02')
  })

  it('кириллица идёт через NotoSans, а не helvetica', async () => {
    const { jsPDF } = await import('jspdf')
    const doc = new jsPDF()
    buildLabelsPdf(doc, [part('TRD-01', 'ступень 7')])
    expect(callsOf(doc, 'setFont')[0]).toEqual(['setFont', 'NotoSans', 'normal'])
    const texts = callsOf(doc, 'text').flatMap((c: any[]) =>
      (Array.isArray(c[1]) ? c[1] : [c[1]]))
    expect(texts).toContain('ступень 7')
  })

  it('переполнение сетки открывает новую страницу', async () => {
    const { jsPDF } = await import('jspdf')
    const doc = new jsPDF()
    const n = stickersPerPage() + 3
    buildLabelsPdf(doc, Array.from({ length: n }, (_, i) => part(`P-${i}`)))
    expect(callsOf(doc, 'rect')).toHaveLength(n)
    expect(callsOf(doc, 'addPage')).toHaveLength(1)
  })

  it('ровно страница без остатка — без лишней страницы', async () => {
    const { jsPDF } = await import('jspdf')
    const doc = new jsPDF()
    buildLabelsPdf(doc, Array.from({ length: stickersPerPage() }, (_, i) => part(`P-${i}`)))
    expect(callsOf(doc, 'addPage')).toHaveLength(0)
  })

  it('пустой список — ошибка, а не пустой PDF', async () => {
    const { jsPDF } = await import('jspdf')
    const doc = new jsPDF()
    expect(() => buildLabelsPdf(doc, [])).toThrow()
  })

  it('download сохраняет файл с переданным именем', async () => {
    h.docs.length = 0
    await downloadLabelsPdf([part('TRD-01')], 'project-1-labels.pdf')
    expect(h.docs).toHaveLength(1)
    expect(callsOf(h.docs[0], 'save')).toEqual([['save', 'project-1-labels.pdf']])
  })
})
