import { materialLabelRu } from '../lib/labels'

export const revalidate = 600

const CATEGORY_LABELS: Record<string, string> = {
  Steel: 'Металл',
  Aluminum: 'Металл',
  Wood: 'Дерево',
}

// CATEGORY_HINT — что можно делать из материала: помогает выбрать, а не просто
// показывать плотность. Тексты проверены по каталогу MFG-0005.
const CATEGORY_HINT: Record<string, string> = {
  Steel: 'Несущая основа для любой геометрии: большие забеги, спиральные и П-образные марши, минимальная толщина ступени.',
  Aluminum: 'Лёгкий и коррозионностойкий: подходит для открытых маршей и помещений с влажным воздухом.',
  Wood: 'Тёплый материал для интерьера: требует толщину ступени от 20 мм, зато ступень можно делать из массива или фанеры.',
}

export default async function MaterialsPage() {
  const { fetchMaterials } = await import('../lib/api')
  const materials = await fetchMaterials()
  const groups = materials.reduce<Record<string, typeof materials>>((acc, m) => {
    const key = m.category || 'Other'
    ;(acc[key] ??= []).push(m)
    return acc
  }, {})

  return (
    <main className="wrap section">
      <h1>Материалы</h1>
      <p className="section__head">
        Каталог из {materials.length} материалов. Для каждого — плотность,
        допустимый диапазон толщины ступени и цена за килограмм. В 3D-конструкторе
        это настоящие PBR-текстуры: сталь с бликами, дерево с рисунком волокон.
      </p>

      {materials.length === 0 && (
        <div className="notice">Каталог временно недоступен — обновите страницу.</div>
      )}

      {Object.entries(groups).map(([category, items]) => (
        <section key={category} className="section">
          <h2>{CATEGORY_LABELS[category] ?? category}</h2>
          {CATEGORY_HINT[category] && <p className="section__head">{CATEGORY_HINT[category]}</p>}
          <table className="spec-table">
            <thead>
              <tr>
                <th>Материал</th>
                <th>Плотность</th>
                <th>Толщина ступени</th>
                <th>Габариты листа</th>
                <th>Цена, ₽/кг</th>
              </tr>
            </thead>
            <tbody>
              {items.map((m) => (
                <tr key={m.code}>
                  <td>
                    <a href={`/materials/${m.code}`}>
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img
                        src={m.swatch_url}
                        alt=""
                        width={36}
                        height={36}
                        style={{ borderRadius: 6, verticalAlign: 'middle', marginRight: 8 }}
                        loading="lazy"
                      />
                      {materialLabelRu(m.code, m.name)}
                    </a>
                  </td>
                  <td>{m.density_kg_m3} кг/м³</td>
                  <td>
                    {m.min_thickness_mm}–{m.max_thickness_mm} мм
                  </td>
                  <td>
                    до {m.max_width_mm}×{m.max_height_mm} мм
                  </td>
                  <td className="price">{formatRub(m.price_per_kg_rub)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      ))}

      <div className="notice">
        Цены в каталоге — ориентировочные: они участвуют в предварительном
        расчёте вместе с раскроем, работой и НДС. Финальную смету считаем после
        замеров.
      </div>
    </main>
  )
}

function formatRub(v: number): string {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(v)
}
