import { fetchExamples } from '../lib/api'
import { StairScene } from '../stair-scene'

export const revalidate = 900

export const metadata = {
  title: 'Примеры расчётов',
  description:
    'Готовые расчёты лестниц: прямой марш из дуба и L-образный из стали. Геометрия и цена посчитаны тем же движком, что и в калькуляторе.',
}

export default async function ExamplesPage() {
  const examples = await fetchExamples()
  const withScene = examples.filter((e) => e.result.mesh?.Vertices?.length)

  return (
    <main className="wrap section">
      <h1>Примеры расчётов</h1>
      <p className="section__head">
        Каждая модель — результат настоящего расчёта: те же формулы, тот же раскрой
        и та же цена, что и в калькуляторе. Меняйте габариты и смотрите, как
        меняется геометрия.
      </p>

      {withScene.length === 0 && (
        <div className="notice">
          Примеры генерируются из API. Если калькулятор временно недоступен, дайте
          ему минуту и обновите страницу.
        </div>
      )}

      <div className="card-grid">
        {withScene.map((e) => (
          <div className="scene" key={e.id}>
            <StairScene mesh={e.result.mesh!} material={e.material} heightMM={2800} />
            <div className="scene__caption">
              <span>{e.title}</span>
              <span>{e.caption}</span>
            </div>
            <div style={{ padding: '0 16px 16px' }}>
              <a className="btn" href="/calculator">
                Пересчитать под себя
              </a>
            </div>
          </div>
        ))}
      </div>
    </main>
  )
}
