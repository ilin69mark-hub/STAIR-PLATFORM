// Приветственный экран: плитки по блокам бизнеса.
//
// Человек видит не устройство системы («вкладка администрирования», «вкладка
// магазина»), а то, за что отвечает. Плитка появляется, если у него есть хотя
// бы одна метрика внутри неё (allowedTiles), а внутри неё разделы открываются
// по своим метрикам — то есть «не выдал» означает «не отображается».
//
// Цифры на плитках намеренно не показаны: ради них пришлось бы грузить данные
// всех блоков сразу, а половина блоков отдала бы 403 человеку без прав.
// Считается, что одна кнопка без цифр полезнее шести попыток загрузки.
import { CABINET_TILES, allowedTileSections, allowedTiles } from '@shared/cabinetTiles'
import type { GrantsSubject } from '@shared/cabinetPermissions'
import type { CabinetTile } from '@shared/cabinetTiles'

export function HomeScreen({
  user,
  onOpenTile,
}: {
  user: GrantsSubject | null
  onOpenTile: (tile: CabinetTile) => void
}) {
  const tiles = allowedTiles(user)

  if (tiles.length === 0) {
    return (
      <div className="page">
        <p className="muted">
          Вам не выдан ни один раздел. Попросите владельца компании выдать права в
          разделе «Команда».
        </p>
      </div>
    )
  }

  return (
    <div className="tiles">
      {tiles.map((tile) => (
        <button
          key={tile.id}
          type="button"
          className="tile"
          onClick={() => onOpenTile(tile)}
        >
          <span className="tile__title">{tile.title}</span>
          <span className="tile__hint">{tile.hint}</span>
          <span className="tile__count">
            {allowedTileSections(user, tile).length}{' '}
            {plural(allowedTileSections(user, tile).length, 'раздел', 'раздела', 'разделов')}
          </span>
        </button>
      ))}
    </div>
  )
}

/** Склонение: 1 раздел, 2 раздела, 5 разделов. */
function plural(n: number, one: string, few: string, many: string): string {
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return one
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few
  return many
}

export { CABINET_TILES }