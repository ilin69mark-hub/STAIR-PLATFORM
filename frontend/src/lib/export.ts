// Экспорт результатов в CSV (FE-0011): BOM, детали, раскрой, цены.
// Генерация полностью клиентская — через Blob + скачивание файла.

import type { Manufacturing, Pricing, Snapshot } from '../api/types'

const RUB_DECIMALS = 2

function cell(v: string | number): string {
  const s = String(v)
  return /[",;\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
}

function csv(headers: string[], rows: Array<Array<string | number>>): string {
  const lines = [headers.map(cell).join(';'), ...rows.map((r) => r.map(cell).join(';'))]
  return `\uFEFF${lines.join('\n')}\n`
}

function download(filename: string, content: string) {
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

function slug(projectId: string): string {
  return projectId.slice(0, 8)
}

export const exportCsv = {
  bom(snapshot: Snapshot, mfg: Manufacturing) {
    if (!mfg) return
    download(
      `stair-${slug(snapshot.project_id)}-bom.csv`,
      csv(
        ['№', 'Деталь', 'Описание', 'Материал', 'Толщина, мм', 'Кол-во', 'Длина, мм', 'Ширина, мм'],
        mfg.BOM.map((l) => [
          l.Number,
          l.PartNumber,
          l.Description,
          l.Material,
          l.Thickness,
          l.Quantity,
          l.Length,
          l.Width,
        ]),
      ),
    )
  },

  parts(snapshot: Snapshot, mfg: Manufacturing) {
    if (!mfg) return
    download(
      `stair-${slug(snapshot.project_id)}-parts.csv`,
      csv(
        ['№', 'Тип', 'Материал', 'Толщина, мм', 'Длина, мм', 'Ширина, мм'],
        mfg.Parts.map((p) => [
          p.Number,
          p.Kind,
          p.Material,
          p.Thickness,
          p.Length,
          p.Width,
        ]),
      ),
    )
  },

  cutList(snapshot: Snapshot, mfg: Manufacturing) {
    if (!mfg) return
    const rows: Array<Array<string | number>> = []
    for (const sheet of mfg.Nesting.Sheets) {
      for (const part of sheet.Placed) {
        rows.push([
          part.PartNumber,
          sheet.Material,
          sheet.Thickness,
          part.Length,
          part.Width,
          part.X,
          part.Y,
        ])
      }
    }
    download(
      `stair-${slug(snapshot.project_id)}-cutlist.csv`,
      csv(
        ['Деталь', 'Лист (материал)', 'Толщина, мм', 'Длина, мм', 'Ширина, мм', 'X, мм', 'Y, мм'],
        rows,
      ),
    )
  },

  pricing(snapshot: Snapshot, pricing: Pricing) {
    if (!pricing) return
    const dec = pricing.Currency?.Decimals ?? RUB_DECIMALS
    const fmtRub = (v: number | undefined) => (v === undefined ? '' : (v / 10 ** dec).toFixed(dec))
    const rows: Array<Array<string | number>> = [
      ['Материалы', fmtRub(pricing.Material)],
      ['Обработка (станки)', fmtRub(pricing.Machine)],
      ['Труд', fmtRub(pricing.Labor)],
      ['Накладные расходы', fmtRub(pricing.Overhead)],
      ['Производственная себестоимость', fmtRub(pricing.ProductionCost)],
      ['Маржа', fmtRub(pricing.Margin)],
      ['Скидка', fmtRub(pricing.Discount)],
      ['До налога', fmtRub(pricing.PreTax)],
      ['НДС', fmtRub(pricing.Tax)],
      ['Итоговая цена', fmtRub(pricing.FinalPrice)],
    ]
    if (pricing.Lines?.length) {
      for (const l of pricing.Lines) {
        rows.push([`  ${l.Name} (${l.Category})`, (l.Amount / 10 ** dec).toFixed(dec)])
      }
    }
    download(
      `stair-${slug(snapshot.project_id)}-pricing.csv`,
      csv(['Статья', `Сумма, ${pricing.Currency?.Code ?? 'RUB'}`], rows),
    )
  },
}