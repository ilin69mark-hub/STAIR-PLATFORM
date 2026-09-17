import { describe, expect, it } from 'vitest'
import { ANNOTATE, WALLS, EDGE_PALETTE, edgeColor, rectEdges, edgeLabel } from './scheme-annot'

describe('scheme-annot', () => {
  it('ANNOTATE false', () => expect(ANNOTATE).toBe(false))
  it('WALLS keys', () => {
    expect(WALLS.top.key).toBe('В')
    expect(WALLS.left.key).toBe('Л')
  })
  it('edgeColor wraps', () => {
    expect(edgeColor(0)).toBe(EDGE_PALETTE[0])
    expect(edgeColor(EDGE_PALETTE.length)).toBe(EDGE_PALETTE[0])
    expect(edgeColor(-1)).toBe(EDGE_PALETTE[EDGE_PALETTE.length - 1])
  })
  it('rectEdges 4 edges', () => {
    const edges = rectEdges([0, 0, 10, 20])
    expect(edges).toHaveLength(4)
    expect(edges[0].side).toBe('top')
    expect(edges[3].side).toBe('left')
  })
  it('edgeLabel', () => {
    expect(edgeLabel(0, 'top')).toBe('1В')
    expect(edgeLabel(1, 'left')).toBe('2Л')
  })
})
