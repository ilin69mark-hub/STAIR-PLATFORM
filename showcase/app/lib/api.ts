// Серверный клиент Go-API для витрины (этап 3).
//
// Каталог материалов и примеры лестниц читаются на сервере (RSC) с
// кэшированием: витрина не должна ждать API на каждом запросе, а данные
// каталога меняются редко. Взаимодействие пользователя (калькулятор, заказ)
// по-прежнему идёт напрямую через /api (rewrite на Go-API).

const API = process.env.STAIR_API_URL ?? 'http://localhost:8080'

export interface Material {
  code: string
  name: string
  category: string
  density_kg_m3: number
  min_thickness_mm: number
  max_thickness_mm: number
  max_width_mm: number
  max_height_mm: number
  price_per_kg_rub: number
  swatch_url: string
  finishes?: string[]
}

export interface QuoteRequest {
  width_mm: number
  height_mm: number
  flight: 'straight' | 'l_shape' | 'u_shape' | 'spiral'
  material: string
  step_height_mm?: number
  step_thickness_mm: number
  stringer_thickness_mm?: number
  riser?: boolean
  clearance_mm?: number
  railing_height_mm?: number
  comfort_step_mm?: number
  room_width_mm?: number
  room_length_mm?: number
  approach_space_mm?: number
  landing_width_mm?: number
  landing_depth_mm?: number
  lower_step_count?: number
  outer_radius_mm?: number
  railing?: string
  railing_lower?: string
  railing_landing?: string
  railing_upper?: string
  direction?: string
  spiral_direction?: string
}

export interface Mesh {
  Vertices: Array<{ X: number; Y: number; Z: number }>
  Triangles: Array<[number, number, number]>
  PartRanges?: Array<{ Solid: number; Role: string; Start: number; End: number }>
}

export interface QuoteResult {
  validation: { valid: boolean; blocking: boolean; issues?: unknown[] }
  flight?: { step_count: number; step_height_mm: number; tread_depth_mm: number; run_mm: number }
  geometry?: Record<string, unknown>
  pricing?: { total_rub?: number; pre_tax_rub?: number; material_rub?: number }
  mesh?: Mesh
  railing_mesh?: Mesh
}

async function getJSON<T>(path: string, init?: RequestInit & { revalidate?: number }): Promise<T | null> {
  try {
    const res = await fetch(`${API}${path}`, {
      ...init,
      headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
      next: { revalidate: init?.revalidate ?? 300 },
    })
    if (!res.ok) return null
    return (await res.json()) as T
  } catch {
    // Витрина не должна падать из-за недоступного API: показываем кэш/заглушку.
    return null
  }
}

/** Каталог материалов (GET /api/v1/public/materials). */
export async function fetchMaterials(): Promise<Material[]> {
  return (await getJSON<Material[]>('/api/v1/public/materials', { revalidate: 600 })) ?? []
}

/** Готовые примеры лестниц с реальным расчётом (меш + цена). */
export interface ShowcaseExample {
  id: string
  title: string
  caption: string
  material: string
  result: QuoteResult
}

export async function fetchExamples(): Promise<ShowcaseExample[]> {
  const plan: ShowcaseExample[] = [
    {
      id: 'straight-oak',
      title: 'Прямой марш, дуб',
      caption: 'Ширина 1000 мм, высота 2800 мм, перила с двух сторон',
      material: 'WOOD-OAK',
      result: {
        validation: { valid: true, blocking: false },
        flight: { step_count: 16, step_height_mm: 175, tread_depth_mm: 290, run_mm: 4640 },
        pricing: { total_rub: 0 },
        mesh: undefined,
      },
    },
    {
      id: 'l-shape-steel',
      title: 'L-образный марш, сталь',
      caption: 'Площадка 1200 мм, чёрная сталь, перила слева',
      material: 'STEEL-S235',
      result: { validation: { valid: true, blocking: false }, mesh: undefined },
    },
    {
      id: 'spiral-aluminium',
      title: 'Спираль, алюминий',
      caption: 'Наружный радиус 1500 мм, винтовой марш с центральной колонной',
      material: 'ALUM-5083',
      result: { validation: { valid: true, blocking: false }, mesh: undefined },
    },
  ]
  // Каждый пример считаем живым расчётом: витрина показывает настоящую
  // геометрию и цену, а не нарисованную картинку.
  const requests: Record<string, QuoteRequest> = {
    'straight-oak': {
      width_mm: 1000,
      height_mm: 2800,
      flight: 'straight',
      material: 'WOOD-OAK',
      step_height_mm: 175,
      step_thickness_mm: 40,
      stringer_thickness_mm: 50,
      riser: true,
      clearance_mm: 2000,
      railing_height_mm: 900,
      comfort_step_mm: 630,
      railing: 'both',
      // Помещение подобрано так, чтобы марш вписывался: забег ~4,6 м плюс
      // свободная зона 1 м перед первой ступенью.
      room_width_mm: 3600,
      room_length_mm: 6200,
    },
    'l-shape-steel': {
      width_mm: 900,
      height_mm: 2800,
      flight: 'l_shape',
      material: 'STEEL-S235',
      step_height_mm: 175,
      step_thickness_mm: 6,
      stringer_thickness_mm: 50,
      riser: true,
      clearance_mm: 2000,
      railing_height_mm: 900,
      comfort_step_mm: 630,
      landing_width_mm: 1200,
      landing_depth_mm: 1200,
      lower_step_count: 8,
      railing_lower: 'left',
      railing_landing: 'both',
      railing_upper: 'right',
      direction: 'left',
      room_width_mm: 3600,
      room_length_mm: 3600,
    },
    'spiral-aluminium': {
      width_mm: 1400,
      height_mm: 2800,
      flight: 'spiral',
      material: 'ALUM-5083',
      step_height_mm: 175,
      step_thickness_mm: 6,
      stringer_thickness_mm: 50,
      riser: true,
      clearance_mm: 2000,
      railing_height_mm: 900,
      outer_radius_mm: 1500,
      // Спираль задаёт направление закрутки в терминах «по/против часовой
      // стрелки», а не left/right (CONF-SPIRAL-RAILING).
      spiral_direction: 'ccw',
      room_width_mm: 3600,
      room_length_mm: 3600,
    },
  }
  const results = await Promise.all(
    plan.map(async (p) => {
      const req = requests[p.id]
      if (!req) return p
      const res = await getJSON<QuoteResult>('/api/v1/public/stairs:quote', {
        method: 'POST',
        body: JSON.stringify(req),
        revalidate: 900,
      })
      return res?.mesh ? { ...p, result: res } : p
    }),
  )
  return results
}
