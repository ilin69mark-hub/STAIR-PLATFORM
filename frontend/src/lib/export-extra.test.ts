import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { exportCsv } from './export'
import { makeSnapshot, manufacturingFixture, pricingFixture } from '../test/fixtures'

describe('exportCsv download paths', () => {
  let createObjectURL: ReturnType<typeof vi.fn>
  let revokeObjectURL: ReturnType<typeof vi.fn>

  beforeEach(() => {
    createObjectURL = vi.fn(() => 'blob:1')
    revokeObjectURL = vi.fn()
    vi.stubGlobal('URL', { createObjectURL, revokeObjectURL } as any)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('bom downloads', () => {
    const snap = makeSnapshot({ project_id: 'abcdefgh-1234' })
    exportCsv.bom(snap, manufacturingFixture as any)
    expect(createObjectURL).toHaveBeenCalled()
    // content is Blob, check via reading? just verify call
    const blobArg = (createObjectURL.mock.calls[0][0] as Blob)
    expect(blobArg).toBeInstanceOf(Blob)
  })

  it('parts downloads', () => {
    const snap = makeSnapshot()
    exportCsv.parts(snap, manufacturingFixture as any)
    expect(createObjectURL).toHaveBeenCalled()
  })

  it('cutList downloads', () => {
    const snap = makeSnapshot()
    exportCsv.cutList(snap, manufacturingFixture as any)
    expect(createObjectURL).toHaveBeenCalled()
  })

  it('pricing downloads', () => {
    const snap = makeSnapshot()
    exportCsv.pricing(snap, pricingFixture as any)
    expect(createObjectURL).toHaveBeenCalled()
  })

  it('no-op when mfg missing', () => {
    exportCsv.bom(makeSnapshot(), null as any)
    expect(createObjectURL).not.toHaveBeenCalled()
  })
})
