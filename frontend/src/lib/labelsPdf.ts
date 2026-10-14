// Лист наклеек в PDF (S-191): настоящий файл, а не «печать из браузера».
//
// Раньше был только CSV и печать через window.print(): PDF получался лишь
// если монтажщик вручную выбирал «Сохранить как PDF» в диалоге. Теперь
// кнопка «Наклейки (PDF)» отдаёт готовый файл со всеми стикерами.
//
// Шрифт — тот же NotoSans (кириллица), что и в КП (proposal.ts): у jsPDF
// из коробки кириллицы нет, без Identity-H место «ступень 7» стало бы
// мусором. ensureFont продублирован, а не импортирован: proposal.ts его
// не экспортирует, а тянуть весь генератор КП ради 8 строк — хуже.
//
// Чанки: jspdf и шрифт (~759 КБ) грузятся лениво только по кнопке,
// как в proposal.ts (S4-1, P2-10), — иначе лежат в стартовом бандле.
import type { jsPDF } from 'jspdf'
import type { Part } from '@shared/types'

const FONT_NAME = 'NotoSans'

function ensureLabelsFont(doc: jsPDF, notoSansBase64: string) {
  try {
    doc.addFileToVFS('NotoSans-Regular.ttf', notoSansBase64)
  } catch {}
  try {
    // Identity-H — кодировка для кириллицы (место детали — по-русски).
    doc.addFont('NotoSans-Regular.ttf', FONT_NAME, 'normal', 'Identity-H')
  } catch {}
}

// Раскладка А4: поля 12 мм, сетка 2×5. Наклейка — рамка для вырезания:
// номер крупно, место под ним, внизу материал и габариты. Одна наклейка =
// один стикер с местом (покупные тоже: у них есть номера).
const PAGE_W = 210
const MARGIN = 12
const COLS = 2
const ROWS = 5
const GAP = 4
const CELL_W = (PAGE_W - 2 * MARGIN - GAP) / COLS
const CELL_H = 51.4

export function stickersPerPage(): number {
  return COLS * ROWS
}

// buildLabelsPdf — чистая вёрстка на переданном документе (для тестов
// документ мокается, как в proposal.test.ts). Пустой список — ошибка,
// а не пустой PDF: кнопки в UI и так гаснут, но молчаливый пустой файл
// хуже громкой ошибки.
export function buildLabelsPdf(doc: jsPDF, parts: Part[]): void {
  if (parts.length === 0) throw new Error('нет деталей для наклеек')
  doc.setFont(FONT_NAME, 'normal')
  parts.forEach((p, i) => {
    if (i > 0 && i % stickersPerPage() === 0) doc.addPage()
    const slot = i % stickersPerPage()
    const col = slot % COLS
    const row = Math.floor(slot / COLS)
    const x = MARGIN + col * (CELL_W + GAP)
    const y = MARGIN + row * (CELL_H + GAP)
    doc.setDrawColor(0)
    doc.setLineWidth(0.4)
    doc.rect(x, y, CELL_W, CELL_H)
    doc.setTextColor(0)
    doc.setFontSize(18)
    doc.text(p.Number, x + 4, y + 12)
    doc.setFontSize(11)
    const where = doc.splitTextToSize(p.Where || '—', CELL_W - 8).slice(0, 2)
    doc.text(where, x + 4, y + 22)
    doc.setFontSize(9)
    doc.setTextColor(80)
    doc.text(`${p.Material} · ${p.Thickness}×${p.Length}×${p.Width}`, x + 4, y + CELL_H - 4)
  })
}

// downloadLabelsPdf — ленивая загрузка jsPDF и шрифта, сборка и сохранение.
export async function downloadLabelsPdf(parts: Part[], filename: string): Promise<void> {
  const { jsPDF } = await import('jspdf')
  const { notoSansBase64 } = await import('./fonts/notoSans')
  const doc = new jsPDF({ unit: 'mm', format: 'a4' })
  ensureLabelsFont(doc, notoSansBase64)
  buildLabelsPdf(doc, parts)
  doc.save(filename)
}
