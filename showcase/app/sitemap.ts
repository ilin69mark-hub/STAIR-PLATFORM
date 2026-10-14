import type { MetadataRoute } from 'next'
import { fetchMaterials } from './lib/api'

// Карта сайта для поиска: статические разделы + постраница каждого материала
// каталога. Каталог приходит с API, поэтому новые материалы попадают в карту
// без правки кода.
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const base = process.env.SITE_URL ?? 'http://localhost:5176'
  const now = new Date()
  const materials = await fetchMaterials()
  return [
    { url: `${base}/`, lastModified: now, changeFrequency: 'weekly', priority: 1 },
    { url: `${base}/materials`, lastModified: now, changeFrequency: 'weekly', priority: 0.8 },
    { url: `${base}/examples`, lastModified: now, changeFrequency: 'weekly', priority: 0.7 },
    { url: `${base}/calculator`, lastModified: now, changeFrequency: 'monthly', priority: 0.9 },
    ...materials.map((m) => ({
      url: `${base}/materials/${m.code}`,
      lastModified: now,
      changeFrequency: 'monthly' as const,
      priority: 0.6,
    })),
  ]
}
