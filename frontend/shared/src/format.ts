// Вспомогательные форматирующие функции отображения результатов.

export const fmt = {
  mm: (v: number | undefined): string =>
    v === undefined ? '—' : `${v.toLocaleString('ru-RU')} мм`,

  // Площадь: вход в мм², отображение в м² (1 м² = 10⁶ мм²).
  m2: (v: number | undefined): string =>
    v === undefined ? '—' : `${(v / 1e6).toLocaleString('ru-RU', { maximumFractionDigits: 2 })} м²`,

  // Объём: вход в мм³, отображение в м³ (1 м³ = 10⁹ мм³).
  m3: (v: number | undefined): string =>
    v === undefined ? '—' : `${(v / 1e9).toLocaleString('ru-RU', { maximumFractionDigits: 3 })} м³`,

  deg: (v: number | undefined): string =>
    v === undefined ? '—' : `${(v * 180 / Math.PI).toFixed(1)}°`,

  rub: (minor: number | undefined, decimals = 2): string => {
    if (minor === undefined) return '—'
    const major = minor / 10 ** decimals
    return `${major.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽`
  },

  // Рубли как основная валюта (public quote отдаёт значения уже в рублях).
  rubMajor: (v: number | undefined): string =>
    v === undefined
      ? '—'
      : `${v.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽`,

  // Проценты (0..1 → 0..100).
  pct: (v: number | undefined): string =>
    v === undefined ? '—' : `${(v * 100).toFixed(0)}%`,
}