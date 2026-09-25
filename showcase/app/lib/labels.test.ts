import { describe, expect, it } from 'vitest'
import { materialLabelRu } from './labels'

describe('materialLabelRu', () => {
  it('берёт русское имя из API', () => {
    expect(materialLabelRu('WOOD-WALNUT', 'Орех', 'American Walnut')).toBe('Орех')
    expect(materialLabelRu('STEEL-CORTEN', 'Кортэн', 'Weathering Steel')).toBe('Кортэн')
  })

  it('падает на словарь фронта, если у кода нет name_ru', () => {
    expect(materialLabelRu('WOOD-OAK', undefined, 'Oak Wood')).toBe('Дуб')
  })

  it('и на техническое имя, если кода нет нигде', () => {
    expect(materialLabelRu('WOOD-BAMBOO', undefined, 'Bamboo')).toBe('Bamboo')
  })

  it('пустое имя из API не превращается в пробелы', () => {
    expect(materialLabelRu('WOOD-OAK', '   ', 'Oak Wood')).toBe('Дуб')
  })
})
