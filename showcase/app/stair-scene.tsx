'use client'

// Клиентский остров 3D для витрины: тонкая обёртка над общим вьювером
// (frontend/shared/src/viewer/GeometryViewer), который уже умеет PBR,
// студийный свет, HDRI и выбор деталей. Серверная страница отдаёт готовый меш,
// браузер только рендерит.
import dynamic from 'next/dynamic'
import type { Mesh } from './lib/api'

const GeometryViewer = dynamic(
  () => import('@shared/viewer/GeometryViewer').then((m) => ({ default: m.GeometryViewer })),
  { ssr: false, loading: () => <div className="scene__loading">Загрузка 3D…</div> },
)

export function StairScene({
  mesh,
  material,
  flight = 'straight',
  heightMM,
}: {
  mesh: Mesh
  material: string
  flight?: 'straight' | 'l_shape' | 'u_shape' | 'spiral'
  heightMM?: number
}) {
  if (!mesh?.Vertices?.length) {
    return <div className="scene__loading">3D-модель готовится…</div>
  }
  return (
    <GeometryViewer
      mesh={mesh as never}
      materialCode={material}
      environmentHDRI="/static-assets/hdri/studio_small_08_1k.hdr"
      railingMetal
      flight={flight}
      heightMM={heightMM}
    />
  )
}
