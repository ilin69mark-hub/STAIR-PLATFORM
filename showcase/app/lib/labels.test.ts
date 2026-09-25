import { describe, expect, it } from 'vitest'
import { materialLabelRu } from './labels'

describe('materialLabelRu', () => {
  it('даёт русскую подпись для кодов каталога', () => {
    expect(materialLabelRu('STEEL-S235', 'Structural Steel S235')).toBe('Сталь S235')
    expect(materialLabelRu('WOOD-WALNUT', 'American Walnut')).toBe('Орех')
    expect(materialLabelRu('STEEL-CORTEN', 'Weathering Steel')).toBe('Кортэн')
  })

  it('падает на имя с API, если кода нет в словаре', () => {
    // Новый материал в каталоге не должен ломать витрину до обновления словаря.
    expect(materialLabelRu('WOOD-BAMBOO', 'Bamboo')).toBe('Bamboo')
  })

  it('покрывает все семь кодов текущего каталога', () => {
    const codes = [
      'STEEL-S235',
      'STEEL-CORTEN',
      'ALUM-5083',
      'WOOD-OAK',
      'WOOD-WALNUT',
      'WOOD-ASH',
      'WOOD-SOFT',
    ]
    for (const c of codes) {
      expect(materialLabelRu(c, c)).not.toBe(c)
    }
  })
})
