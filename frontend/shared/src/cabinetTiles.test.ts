// Приветственный экран: какие плитки и разделы видит сотрудник.
import { describe, expect, it } from 'vitest'

import {
  CABINET_TILES,
  DEFERRED_CAPABILITIES,
  allowedTileSections,
  allowedTiles,
  findTile,
  firstTileId,
} from './cabinetTiles'
import { P, type GrantsSubject } from './cabinetPermissions'

function withGrants(...caps: (keyof typeof P)[]): GrantsSubject {
  return {
    grants: caps.map((c) => ({ capability: P[c], level: 'read' as const })),
  }
}

describe('реестр плиток', () => {
  it('id плиток и разделов уникальны', () => {
    const tiles = new Set(CABINET_TILES.map((t) => t.id))
    expect(tiles.size).toBe(CABINET_TILES.length)
    const sections = CABINET_TILES.flatMap((t) => t.sections.map((s) => s.id))
    expect(new Set(sections).size).toBe(sections.length)
  })

  it('каждый раздел помечен нужной метрикой и у каждой есть заголовок', () => {
    for (const tile of CABINET_TILES) {
      expect(tile.title.length).toBeGreaterThan(0)
      expect(tile.hint.length).toBeGreaterThan(0)
      for (const s of tile.sections) {
        expect(s.title.length).toBeGreaterThan(0)
        expect(s.needs !== undefined).toBe(true)
      }
    }
  })

  it('проекты — первая плитка и доступна всем', () => {
    expect(CABINET_TILES[0].id).toBe('projects')
    expect(CABINET_TILES[0].sections[0].needs).toBeNull()
    expect(allowedTiles(withGrants()).map((t) => t.id)).toEqual(['projects'])
  })

  it('каждая метрика с данными открывает какой-то раздел', () => {
    // Метрика-действие (кнопка) раздела не открывает: «Оформить возврат» и
    // «Выдать ключ» живут кнопками внутри чужих разделов, а часть действий
    // (передать проект, удалить сотрудника) появится позже — они перечислены
    // в DEFERRED_CAPABILITIES. Данные, наоборот, обязаны иметь раздел: иначе
    // владелец выдал бы полномочие, которого на экране нигде нет.
    const used = new Set(
      CABINET_TILES.flatMap((t) => t.sections.map((s) => s.needs).filter(Boolean)),
    )
    const actions: string[] = []
    for (const cap of Object.values(P)) {
      if (used.has(cap as never)) continue
      actions.push(cap)
    }
    for (const cap of actions) {
      expect(DEFERRED_CAPABILITIES.has(cap as never), `метрика ${cap} не открывает раздел и не числится отложенной`).toBe(true)
    }
  })
})

describe('видимость плиток по метрикам', () => {
  it('владелец видит все плитки', () => {
    expect(allowedTiles(withGrants(...(Object.keys(P) as (keyof typeof P)[]))).map((t) => t.id)).toEqual([
      'projects', 'site', 'pricing', 'finance', 'production', 'traffic', 'staff',
    ])
  })

  it('менеджер с одной метрикой видит одну плитку плюс проекты', () => {
    expect(allowedTiles(withGrants('financeSales')).map((t) => t.id)).toEqual(['projects', 'finance'])
  })

  it('руководитель продаж не видит то, что не выдавали', () => {
    const sales = withGrants('staffView', 'financeSales', 'trafficFunnel', 'priceMaterials')
    // Нет site.* (витрина) и production.* (производство) — плиток нет.
    expect(allowedTiles(sales).map((t) => t.id)).toEqual([
      'projects', 'pricing', 'finance', 'traffic', 'staff',
    ])
  })

  it('публичный каталог не делает плитку «Сайт и витрина» всеобщей', () => {
    // Каталог материалов открыт всем (needs: null), но контакты, реквизиты,
    // соцсети, SEO и отзывы — за метриками. Без этого правила плитка
    // появлялась бы у каждого, и человек с одной метрикой «заказы» увидел
    // бы блок, в котором ему всё равно ничего не открыть.
    expect(allowedTiles(withGrants('financeSales')).map((t) => t.id)).not.toContain('site')
    expect(allowedTiles(withGrants('siteSEO')).map((t) => t.id)).toContain('site')
  })

  it('плитка есть по одной метрике, а лишние разделы внутри скрыты', () => {
    const u = withGrants('siteSEO')
    const tile = findTile(u, 'site')
    expect(tile).not.toBeNull()
    expect(allowedTileSections(u, tile!).map((s) => s.id)).toEqual(['site-seo'])
  })

  it('без пользователя не открывается ни одна плитка', () => {
    expect(allowedTiles(null)).toEqual([])
    expect(findTile(null, 'staff')).toBeNull()
    expect(firstTileId(null)).toBeNull()
  })

  it('первая доступная плитка — та, что реально доступна', () => {
    expect(firstTileId(withGrants('financeSales'))).toBe('projects')
    expect(firstTileId(null)).toBeNull()
  })

  it('закрытая плитка не открывается по прямой ссылке', () => {
    // Раньше можно было попасть во вкладку по состоянию, даже если права
    // не хватало: активная вкладка просто откатывалась. Здесь findTile
    // возвращает null — значит, экран не откроется.
    expect(findTile(withGrants('financeSales'), 'staff')).toBeNull()
  })
})
