// Карта раскроя (MFG-0012): 2D-схема листов с размещёнными деталями.
// Строится чисто на фронтенде — SVG, данные из снапшота (Nesting).

import type { Nesting, PlacedPart, SheetLayout } from '../types'
import { fmt } from '../format'

interface Props {
  nesting: Nesting
}

const PALETTE = [
  '#2563eb',
  '#16a34a',
  '#d97706',
  '#7c3aed',
  '#dc2626',
  '#0d9488',
  '#db2777',
  '#4b5563',
  '#ca8a04',
  '#0891b2',
]

const BOX_W = 300
const BOX_H = 200

function colorFor(number: string): string {
  let h = 0
  for (let i = 0; i < number.length; i++) {
    h = (h * 31 + number.charCodeAt(i)) >>> 0
  }
  return PALETTE[h % PALETTE.length]
}

interface SheetProps {
  sheet: SheetLayout
  index: number
}

function SheetView({ sheet, index }: SheetProps) {
  const scale = Math.min(BOX_W / sheet.Length, BOX_H / sheet.Width)
  const w = sheet.Length * scale
  const h = sheet.Width * scale
  const ox = (BOX_W - w) / 2
  const oy = (BOX_H - h) / 2

  return (
    <div className="nesting-sheet">
      <svg
        className="nesting-sheet__svg"
        viewBox={`0 0 ${BOX_W} ${BOX_H}`}
        role="img"
        aria-label={`Лист ${index + 1}: ${sheet.Material} ${sheet.Thickness} мм`}
      >
        <rect x={ox} y={oy} width={w} height={h} className="nesting-sheet__bg" />
        {sheet.Placed.map((p, i) => (
          <PartRect key={`${p.PartNumber}-${i}`} part={p} scale={scale} ox={ox} oy={oy} />
        ))}
      </svg>
      <p className="nesting-sheet__caption">
        Лист {index + 1} · {sheet.Material} {Math.round(sheet.Thickness)} мм ·{' '}
        {Math.round(sheet.Length)}×{Math.round(sheet.Width)} мм · деталей: {sheet.Placed.length}
      </p>
    </div>
  )
}

interface PartRectProps {
  part: PlacedPart
  scale: number
  ox: number
  oy: number
}

function PartRect({ part, scale, ox, oy }: PartRectProps) {
  const x = ox + part.X * scale
  const y = oy + part.Y * scale
  const w = part.Length * scale
  const h = part.Width * scale
  const showLabel = w > 34 && h > 18

  return (
    <g>
      <rect
        x={x}
        y={y}
        width={w}
        height={h}
        rx={2}
        className="nesting-sheet__part"
        fill={colorFor(part.PartNumber)}
      />
      {showLabel && (
        <text
          x={x + w / 2}
          y={y + h / 2}
          className="nesting-sheet__partlabel"
          textAnchor="middle"
          dominantBaseline="central"
        >
          {part.PartNumber}
        </text>
      )}
    </g>
  )
}

export function NestingMap({ nesting }: Props) {
  if (!nesting || nesting.Sheets.length === 0) return null

  return (
    <div className="scheme">
      <div className="nesting-map">
        {nesting.Sheets.map((s, i) => (
          <SheetView key={`${s.Material}-${i}`} sheet={s} index={i} />
        ))}
      </div>
      <p className="scheme__caption">
        Деталей: {nesting.PartCount} · площадь деталей {fmt.m2(nesting.PartArea)} · отходы{' '}
        {fmt.m2(nesting.WasteArea)} · использование {fmt.pct(nesting.Utilization)}
      </p>
    </div>
  )
}
