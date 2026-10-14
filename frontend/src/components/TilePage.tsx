// Страница плитки: все её разделы одним скроллом.
//
// Решений «плитка → вкладка → раздел» больше нет: у плитки 2–5 разделов, и
// второй клик только мешал бы. Разделы идут в порядке реестра (cabinetTiles),
// каждый — в своём <section id>, чтобы на них можно было сослаться и чтобы
// якорь работал после перезагрузки.
import { allowedTileSections, type CabinetTile } from '@shared/cabinetTiles'
import type { GrantsSubject } from '@shared/cabinetPermissions'

import { renderSection } from './admin/sections'

export function TilePage({ user, tile }: { user: GrantsSubject | null; tile: CabinetTile }) {
  const sections = allowedTileSections(user, tile)

  if (sections.length === 0) {
    return <p className="muted">В этом блоке вам ничего не выдано.</p>
  }

  return (
    <div className="tile-page">
      {sections.map((section) => (
        <section key={section.id} id={section.id} className="section">
          <h2 className="section__title">{section.title}</h2>
          <SectionBody id={section.id} user={user} />
        </section>
      ))}
    </div>
  )
}

function SectionBody({ id, user }: { id: string; user: GrantsSubject | null }) {
  const node = renderSection(id, { user })
  if (node === null) {
    // Раздел есть в реестре плиток, но панели нет: показываем честное
    // сообщение вместо пустого места, которое выглядело бы как «пусто».
    return <p className="muted">Раздел пока не реализован.</p>
  }
  return <>{node}</>
}