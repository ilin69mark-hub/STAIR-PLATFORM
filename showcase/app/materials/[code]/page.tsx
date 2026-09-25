import { materialLabelRu } from '../../lib/labels'

const CATEGORY_LABELS: Record<string, string> = {
  Steel: 'Металл',
  Aluminum: 'Металл',
  Wood: 'Дерево',
}

const FINISH_LABELS: Record<string, string> = {
  raw: 'Без покрытия',
  black: 'Чёрное',
  white: 'Белое',
  natural: 'Натуральное',
  oil: 'Масло',
  matte: 'Матовый лак',
  toned: 'Тонировка',
}

// USAGE — что этот материал делает в лестнице. Тексты отражают каталог
// MFG-0005 (плотность, диапазоны толщины) и раскрой MFG-0012.
const USAGE: Record<string, string> = {
  'STEEL-S235': 'Универсальная несущая основа: тонкая ступень 3–8 мм, большие пролёты, любые марши.',
  'STEEL-CORTEN': 'Сталь с выветривающейся патиной: сначала тёмная, потом рыжевато-коричневая. Для лофт- и студийных интерьеров.',
  'ALUM-5083': 'Лёгкий металл с высокой устойчивостью к коррозии: открытые марши, влажные зоны, балконы.',
  'WOOD-OAK': 'Классика для интерьера: выраженный рисунок, ступень 40 мм из массива или фанеры.',
  'WOOD-WALNUT': 'Тёмное плотное дерево: меньший шаг реза, чем у дуба, при этом выглядит дороже.',
  'WOOD-ASH': 'Светлая древесина высокой твёрдости: хорошо держит нагрузку и почти не темнеет.',
  'WOOD-SOFT': 'Сосна — самый бюджетный вариант: подходит для чердаков, дач и первых лестниц.',
}

// Next генерирует глобальные LayoutProps<'/...'> после dev/build; пока типы
// не сгенерированы, params описаны здесь явно.
type MaterialPageProps = { params: Promise<{ code: string }> }

export default async function MaterialPage({ params }: MaterialPageProps) {
  const { code } = await params
  const { fetchMaterials } = await import('../../lib/api')
  const materials = await fetchMaterials()
  const m = materials.find((x) => x.code === code)
  const others = materials.filter((x) => x.code !== code)

  return (
    <main className="wrap section">
      {!m ? (
        <>
          <h1>Материал не найден</h1>
          <p className="section__head">
            Такого кода нет в каталоге. Посмотрите{' '}
            <a href="/materials">полный список материалов</a>.
          </p>
        </>
      ) : (
        <>
          <p className="section__head">
            <a href="/materials">{CATEGORY_LABELS[m.category] ?? m.category}</a> · код {m.code}
          </p>
          <h1>{materialLabelRu(m.code, m.name)}</h1>
          <div className="hero">
            <div>
              <p className="hero__lead">{USAGE[code] ?? 'Материал из каталога лестниц.'}</p>
              <table className="spec-table">
                <tbody>
                  <tr>
                    <th>Плотность</th>
                    <td>{m.density_kg_m3} кг/м³</td>
                  </tr>
                  <tr>
                    <th>Толщина ступени</th>
                    <td>
                      {m.min_thickness_mm}–{m.max_thickness_mm} мм
                    </td>
                  </tr>
                  <tr>
                    <th>Габарит листа</th>
                    <td>
                      до {m.max_width_mm}×{m.max_height_mm} мм
                    </td>
                  </tr>
                  <tr>
                    <th>Цена материала</th>
                    <td className="price">{formatRub(m.price_per_kg_rub)}/кг</td>
                  </tr>
                  <tr>
                    <th>Финиши</th>
                    <td>{(m.finishes ?? []).map((f) => FINISH_LABELS[f] ?? f).join(' · ')}</td>
                  </tr>
                </tbody>
              </table>
              <p style={{ marginTop: 24 }}>
                <a className="btn" href={`/calculator?material=${m.code}`}>
                  Рассчитать из {materialLabelRu(m.code, m.name)}
                </a>
              </p>
            </div>
            <div className="scene">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={m.swatch_url}
                alt={`Текстура материала ${materialLabelRu(m.code, m.name)}`}
                style={{ width: '100%', height: 380, objectFit: 'cover', display: 'block' }}
              />
              <div className="scene__caption">
                <span>Реальная текстура</span>
                <span>используется в 3D</span>
              </div>
            </div>
          </div>
        </>
      )}

      {others.length > 0 && (
        <section className="section">
          <h2>Другие материалы</h2>
          <div className="swatches">
            {others.map((o) => (
              <a className="swatch-card" href={`/materials/${o.code}`} key={o.code}>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img className="swatch-card__img" src={o.swatch_url} alt="" loading="lazy" />
                <div className="swatch-card__body">
                  <span className="card__title">{materialLabelRu(o.code, o.name)}</span>
                  <span className="price">{formatRub(o.price_per_kg_rub)}/кг</span>
                </div>
              </a>
            ))}
          </div>
        </section>
      )}
    </main>
  )
}

function formatRub(v: number): string {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(v)
}
