// Вспомогательные форматирующие функции отображения результатов.

export const fmt = {
  mm: (v: number | undefined): string =>
    v === undefined ? '—' : `${v.toLocaleString('ru-RU')} мм`,

  mm3: (v: number | undefined): string =>
    v === undefined ? '—' : `${(v / 1e6).toLocaleString('ru-RU', { maximumFractionDigits: 2 })}×10⁶ мм³`,

  mm2: (v: number | undefined): string =>
    v === undefined ? '—' : `${(v / 1e6).toLocaleString('ru-RU', { maximumFractionDigits: 2 })}×10⁶ мм²`,

  deg: (v: number | undefined): string =>
    v === undefined ? '—' : `${(v * 180 / Math.PI).toFixed(1)}°`,

  rub: (minor: number | undefined, decimals = 2): string => {
    if (minor === undefined) return '—'
    const major = minor / 10 ** decimals
    return `${major.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽`
  },

  // Проценты (0..1 → 0..100).
  pct: (v: number | undefined): string =>
    v === undefined ? '—' : `${(v * 100).toFixed(0)}%`,
}