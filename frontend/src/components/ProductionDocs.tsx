// Документы производства (Фаза 0): документ реза, ведомость наклеек,
// лист наклеек для печати и сборочная ведомость.
//
// Что здесь и почему четыре вещи, а не одна. На производстве четыре разных
// рабочих места, и у каждого свой файл:
//   лазер          — документ реза (DXF с сервера),
//   принтер этикеток — ведомость наклеек (CSV с сервера),
//   участок печати — лист наклеек (эта страница, печать в PDF из браузера),
//   монтажник      — сборочная ведомость (номер → куда, ниже).
//
// Лист наклеек рисуется здесь, а не на сервере, намеренно: вёрстка наклейки
// (размер шрифта, сетка на А4) — дело того, кто печатает. Сервер отдаёт
// данные (номер, место, материал, габариты), а не картинку. Разделить их
// значит не дублировать источник: поменялся номер на сервере — поменялся и
// на листе без второй правки.

import { useState } from 'react'
import { projectsApi } from '../api/projects'
import type { BomLine, Part } from '@shared/types'

interface Props {
  projectId: string
  parts: Part[]
  bom: BomLine[]
  hasCalculation: boolean
}

type Status = 'idle' | 'busy' | 'error'

// downloadFile — скачивание файла тем же способом, что и JSON-снапшот
// (fetch + Blob): переход по URL не передаёт origin-заголовок, и API
// находит не ту session-cookie (401).
async function downloadFile(url: string, filename: string): Promise<void> {
  const res = await fetch(url, {
    credentials: 'include',
    headers: { 'X-App-Origin': 'admin' },
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  const blob = await res.blob()
  const obj = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = obj
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(obj)
}

export function ProductionDocs({ projectId, parts, bom, hasCalculation }: Props) {
  const [status, setStatus] = useState<Status>('idle')
  const [message, setMessage] = useState('')

  const run = async (label: string, fn: () => Promise<void>) => {
    setStatus('busy')
    setMessage('')
    try {
      await fn()
      setStatus('idle')
    } catch {
      setStatus('error')
      setMessage(`Не удалось: ${label}`)
    }
  }

  const cutting = parts.filter((p) => !p.Purchased)
  const purchased = parts.filter((p) => p.Purchased)

  return (
    <section className="section">
      <div className="panel">
        <h2 className="panel__title">Документы производства</h2>
        <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>
          Файлы для участка: детали для лазера, наклейки на каждую деталь со стикером и местом,
          сборочная ведомость «номер → куда».
        </p>

        <div className="row" style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
          <button
            className="btn"
            disabled={!hasCalculation || status === 'busy'}
            onClick={() =>
              void run('документ реза', () =>
                downloadFile(projectsApi.cuttingUrl(projectId), `project-${projectId}-cutting.dxf`),
              )
            }
          >
            Документ реза (DXF)
          </button>
          <button
            className="btn"
            disabled={!hasCalculation || status === 'busy'}
            onClick={() =>
              void run('ведомость наклеек', () =>
                downloadFile(projectsApi.labelsUrl(projectId), `project-${projectId}-labels.csv`),
              )
            }
          >
            Ведомость наклеек (CSV)
          </button>
          <button
            className="btn"
            disabled={!hasCalculation || status === 'busy'}
            onClick={() =>
              void run('спецификация', () =>
                downloadFile(projectsApi.specUrl(projectId), `project-${projectId}-spec.csv`),
              )
            }
          >
            Спецификация (CSV)
          </button>
          <button
            className="btn"
            disabled={!hasCalculation || status === 'busy'}
            onClick={() =>
              void run('сборочный чертёж', () =>
                downloadFile(projectsApi.assemblyUrl(projectId), `project-${projectId}-assembly.svg`),
              )
            }
          >
            Сборочный чертёж (SVG)
          </button>
          <button className="btn" disabled={parts.length === 0} onClick={() => window.print()}>
            Печать листа наклеек
          </button>
        </div>
        {message && <div className="alert alert--error">{message}</div>}

        <h3 className="panel__subtitle">Спецификация — позиция → стикеры</h3>
        {bom.length === 0 ? (
          <p className="muted">Нет позиций — рассчитайте проект.</p>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Поз</th>
                <th>Деталь</th>
                <th>Кол-во</th>
                <th>Стикеры</th>
              </tr>
            </thead>
            <tbody>
              {bom.map((l) => (
                <tr key={l.Number}>
                  <td>{l.Number}</td>
                  <td>{l.Description}</td>
                  <td>{l.Quantity}</td>
                  <td>
                    <code>{(l.Numbers ?? [l.PartNumber]).join(', ')}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        <h3 className="panel__subtitle">Сборочная ведомость — номер → куда</h3>
        {parts.length === 0 ? (
          <p className="muted">Нет деталей — рассчитайте проект.</p>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Стикер</th>
                <th>Куда</th>
                <th>Материал</th>
                <th>Габарит, мм</th>
              </tr>
            </thead>
            <tbody>
              {parts.map((p) => (
                <tr key={p.Number}>
                  <td>
                    <code>{p.Number}</code>
                  </td>
                  <td>{p.Where || <span className="muted">—</span>}</td>
                  <td>{p.Material}</td>
                  <td>
                    {p.Thickness} × {p.Length} × {p.Width}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {cutting.length > 0 && purchased.length > 0 && (
          <p className="muted" style={{ fontSize: 12, marginTop: 8 }}>
            Режется {cutting.length} деталей, покупается {purchased.length}. Покупные в документ
            реза не входят: листа под них не режем.
          </p>
        )}
      </div>

      <div className="panel print-sheet">
        <h2 className="panel__title print-only">Лист наклеек — проект, расчёт</h2>
        <p className="muted print-screen" style={{ fontSize: 12 }}>
          Ниже — наклейки на все детали изделия. Печатаются кнопкой выше: одна наклейка =
          один стикер с местом. Покупные тоже с наклейками — у них есть номера.
        </p>
        <div className="stickers">
          {parts.map((p) => (
            <div className="sticker" key={p.Number}>
              <div className="sticker__number">{p.Number}</div>
              <div className="sticker__where">{p.Where || '—'}</div>
              <div className="sticker__meta">
                {p.Material} · {p.Thickness}×{p.Length}×{p.Width}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
