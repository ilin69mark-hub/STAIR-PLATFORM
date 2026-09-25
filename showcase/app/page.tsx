import Link from 'next/link'
import { fetchExamples, fetchMaterials } from './lib/api'
import { materialLabelRu } from './lib/labels'
import { StairScene } from './stair-scene'
import { ServicesPay } from './services-pay'

export const revalidate = 900

const STEPS = [
  {
    title: 'Задаёте габариты',
    text: 'Ширина марша, высота подъёма и помещение — конструктор сразу подсказывает, вписывается ли лестница.',
  },
  {
    title: 'Выбираете материал',
    text: 'Сталь, алюминий или дерево: толщина подтягивается под допуск материала, раскрой считается по листам.',
  },
  {
    title: 'Смотрите 3D и цену',
    text: 'Реальная геометрия марша в 3D и предварительная цена по вашему материалу — без звонков и ожидания.',
  },
  {
    title: 'Уточняем на замерах',
    text: 'Инженер приедет с образцами: меряем, считаем финальную смету и собираем по вашим размерам.',
  },
]

const FAQ = [
  {
    q: 'Сколько стоит лестница в среднем?',
    a: 'Стальной прямой марш 900×2700 мм выходит в десятки тысяч рублей, деревянный — в сотни тысяч, марши с площадкой и нестандартными размерами дороже. Точную цифру даёт калькулятор: он считает материал, раскрой, работу и НДС.',
  },
  {
    q: 'Какие размеры считаются допустимыми?',
    a: 'Высота ступени 150–220 мм, проступь 260–340 мм, ширина марша от 900 мм. Если ваш проект выходит за границы, конструктор предлагает ближайший рабочий вариант одной кнопкой.',
  },
  {
    q: 'Из каких материалов делаете?',
    a: 'Сталь S235 и кортен, алюминий 5083, дуб, орех, ясень и сосна. Для каждого — свой диапазон толщины, своя скорость обработки и своя цена за килограмм.',
  },
  {
    q: 'Можно ли изменить геометрию после расчёта?',
    a: 'Да. В 3D можно выбрать ступень или площадку и потянуть: по вертикали меняется высота марша, по горизонтали — проступь и забег. Каждое действие пересчитывает цену.',
  },
  {
    q: 'Что входит в предварительную цену?',
    a: 'Материал с учётом раскроя листов, работа станка и ручная доводка, накладные, маржа, скидка и НДС. Доставка и монтаж считаются отдельно после замеров.',
  },
]

export default async function Home() {
  const [materials, examples] = await Promise.all([fetchMaterials(), fetchExamples()])
  const withScene = examples.filter((e) => e.result.mesh?.Vertices?.length)

  return (
    <main>
      <section className="wrap hero">
        <div>
          <h1>Лестница под ваш проём — расчёт, 3D и цена за минуту</h1>
          <p className="hero__lead">
            Проектируем прямые, L-образные и П-образные марши.
            Движок считает геометрию, раскрой листов и себестоимость — вы видите
            честную предварительную цену и реальную 3D-модель.
          </p>
          <div className="hero__actions">
            <Link href="/calculator" className="btn">
              Рассчитать лестницу
            </Link>
            <Link href="/materials" className="btn btn--ghost">
              Смотреть материалы
            </Link>
          </div>
          <ul className="hero__points">
            <li>7 материалов в каталоге</li>
            <li>Раскрой по листам, а не «на глаз»</li>
            <li>Цена с НДС и маржой</li>
          </ul>
        </div>
        <div className="scene">
          {withScene[0] ? (
            <>
              <StairScene
                mesh={withScene[0].result.mesh!}
                material={withScene[0].material}
                heightMM={2800}
              />
              <div className="scene__caption">
                <span>{withScene[0].title}</span>
                <span>{withScene[0].caption}</span>
              </div>
            </>
          ) : (
            <div className="scene__loading">3D-модель недоступна: калькулятор API не отвечает</div>
          )}
        </div>
      </section>

      <section className="wrap section">
        <h2>Как это работает</h2>
        <p className="section__head">
          От габаритов до готового заказа — четыре шага, без чертежей в
          редакторе и без «приходите в офис за ценой».
        </p>
        <div className="steps">
          {STEPS.map((s) => (
            <div className="step" key={s.title}>
              <h3>{s.title}</h3>
              <p className="card__meta">{s.text}</p>
            </div>
          ))}
        </div>
      </section>

      {withScene.length > 0 && (
        <section className="wrap section">
          <h2>Примеры готовых расчётов</h2>
          <p className="section__head">
            Это не картинки: геометрия и цена считаются тем же движком, что и в
            калькуляторе.
          </p>
          <div className="card-grid">
            {withScene.map((e) => (
              <div className="scene" key={e.id}>
                <StairScene mesh={e.result.mesh!} material={e.material} heightMM={2800} />
                <div className="scene__caption">
                  <span>{e.title}</span>
                  <span>{e.caption}</span>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="wrap section">
        <h2>Материалы</h2>
        <p className="section__head">
          Каталог из {materials.length} материалов с плотностью, допустимой
          толщиной и ценой за килограмм. В 3D это настоящие PBR-текстуры.
        </p>
        {materials.length === 0 ? (
          <div className="notice">
            Каталог материалов временно недоступен — обновите страницу через
            минуту.
          </div>
        ) : (
          <div className="swatches">
            {materials.map((m) => (
              <Link className="swatch-card" href={`/materials/${m.code}`} key={m.code}>
                {/* Текстура отдаётся Go-API через /static-assets; обычный img,
                    чтобы не тянуть оптимизатор ради 7 картинок. */}
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  className="swatch-card__img"
                  src={m.swatch_url}
                  alt={`Текстура: ${materialLabelRu(m.code, m.name_ru, m.name)}`}
                  loading="lazy"
                />
                <div className="swatch-card__body">
                  <span className="card__title">{materialLabelRu(m.code, m.name_ru, m.name)}</span>
                  <span className="price">{formatRub(m.price_per_kg_rub)}/кг</span>
                  <span className="card__meta">
                    Толщина {m.min_thickness_mm}–{m.max_thickness_mm} мм
                  </span>
                </div>
              </Link>
            ))}
          </div>
        )}
      </section>

      <section className="wrap section">
        <h2>Инженерные услуги</h2>
        <p className="section__head">
          Замер и проект — фиксированная цена, оплата онлайн после входа. Стоимость
          самой лестности считаем по результатам замера.
        </p>
        <div id="services">
          <ServicesPay />
        </div>
      </section>

      <section className="wrap section" id="faq">
        <h2>Частые вопросы</h2>
        <div className="faq">
          {FAQ.map((f) => (
            <details key={f.q}>
              <summary>{f.q}</summary>
              <p className="card__meta">{f.a}</p>
            </details>
          ))}
        </div>
      </section>

      <section className="wrap">
        <div className="cta">
          <h2>Посчитайте свою лестницу</h2>
          <p>
            Введите габариты — и через минуту увидите 3D-модель, состав
            материалов и предварительную цену.
          </p>
          <Link href="/calculator" className="btn">
            Открыть калькулятор
          </Link>
        </div>
      </section>
    </main>
  )
}

function formatRub(v: number): string {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(v) + ' ₽'
}
