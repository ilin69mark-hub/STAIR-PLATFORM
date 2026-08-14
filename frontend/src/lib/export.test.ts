import { describe, expect, it } from 'vitest'
import { buildCsv } from './export'

describe('buildCsv', () => {
  it('начинается с BOM и разделяет значения точкой с запятой', () => {
    const out = buildCsv(['A', 'B'], [[1, 'x']])
    expect(out.startsWith('\uFEFF')).toBe(true)
    expect(out).toBe('\uFEFFA;B\n1;x\n')
  })

  it('экранирует кавычки внутри значения', () => {
    const out = buildCsv(['C'], [['say "hi"']])
    expect(out).toContain('"say ""hi"""')
  })

  it('экранирует значения с разделителями и переводами строк', () => {
    const out = buildCsv(['C'], [['a;b']])
    expect(out).toBe('\uFEFFC\n"a;b"\n')
  })

  it('не экранирует простые значения', () => {
    const out = buildCsv(['N'], [['steel']])
    expect(out).toBe('\uFEFFN\nsteel\n')
  })
})