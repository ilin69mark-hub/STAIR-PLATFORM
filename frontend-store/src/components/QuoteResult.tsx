import { lazy, Suspense } from 'react'
import type { QuoteResult as QuoteResultType, QuoteSuggestion } from '@shared/types'
import { fmt } from '@shared/format'
import { materialLabel } from '@shared/config'
import { StairProfile } from '@shared/schemes/StairProfile'
import { solverOf } from './quoteView'

// Результат публичного расчёта: марш, геометрия и предварительная цена.
// Покупателю показываем схему-профиль и интерактивную 3D-модель (меш уже
// приходит в публичном ответе); производственный пакет (BOM/раскрой) остаётся
// в админке. three.js грузится лениво, чтобы не утяжелять основной бандл.

const GeometryViewer = lazy(() =>
  import('@shared/viewer/GeometryViewer').then((m) => ({ default: m.GeometryViewer })),
)

interface Props {
  quote: QuoteResultType
  onApplySuggestion?: (s: QuoteSuggestion) => void
  material?: string
}

export function QuoteResult({ quote, onApplySuggestion, material }: Props) {
  const solver = solverOf(quote)
  const geometry = quote.geometry
  const pricing = quote.pricing
  const issues = quote.validation.issues ?? []
  const spiral = quote.spiral !== undefined

  return (
    <>
      <section className="panel">
        <h2>
          Результат расчёта {quote.validation.valid ? '✅' : '⚠️'}
        </h2>
        <p className="sub">
          {quote.validation.blocking
            ? 'Расчёт остановлен: обнаружены блокирующие нарушения.'
            : 'Геометрия лестницы рассчитана.'}
        </p>

        {issues.length > 0 && (
          <div
            className={`alert ${quote.validation.blocking ? 'alert--error' : 'alert--warn'}`}
            role="alert"
          >
            {issues.map((i, idx) => (
              <div className="issue" key={idx}>
                <div>
                  <strong>{i.severity}:</strong> {i.guide ?? i.message}
                  {!i.guide && i.fix ? ` (${i.fix})` : ''}
                </div>
                {i.param && (
                  <div className="issue-param">Что поправить: {i.param}</div>
                )}
                {i.suggestions && i.suggestions.length > 0 && (
                  <div className="issue-suggestions">
                    {i.suggestions.map((s, si) => (
                      <div className="suggestion" key={si}>
                        <span>
                          {s.step_count} ступ. · h {fmt.mm(s.step_height_mm)} · проступь{' '}
                          {fmt.mm(s.tread_depth_mm)} · угол {s.angle_deg.toFixed(1)}°
                          {spiral && s.outer_radius_mm
                            ? ` · ширина ${fmt.mm(s.width_mm)} · радиус ${fmt.mm(s.outer_radius_mm)}`
                            : ''}
                        </span>
                        {onApplySuggestion && (
                          <button
                            type="button"
                            className="sp-btn"
                            onClick={() => onApplySuggestion(s)}
                          >
                            Применить эти значения
                          </button>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}

        {!quote.validation.blocking && solver.flight && (
          <div className="scheme-wrap">
            <StairProfile flight={solver.flight} />
          </div>
        )}

        {!quote.validation.blocking && quote.mesh && quote.mesh.Vertices.length > 0 && (
          <div className="scheme-3d">
            <h3 className="scheme-3d__title">3D-модель</h3>
            <Suspense fallback={<p className="muted">Загрузка 3D…</p>}>
              <GeometryViewer mesh={quote.mesh} />
            </Suspense>
          </div>
        )}
      </section>

      {!quote.validation.blocking && solver.flight && (
        <section className="panel">
          <h2>Геометрия марша</h2>
          <dl className="kv">
            <div>
              <dt>Ступеней</dt>
              <dd>{solver.flight.StepCount}</dd>
            </div>
            <div>
              <dt>Высота ступени</dt>
              <dd>{fmt.mm(solver.flight.StepHeight)}</dd>
            </div>
            <div>
              <dt>Глубина проступи</dt>
              <dd>{fmt.mm(solver.flight.TreadDepth)}</dd>
            </div>
            <div>
              <dt>Угол наклона</dt>
              <dd>{fmt.deg(solver.flight.Angle)}</dd>
            </div>
            {solver.lowerStepCount !== undefined && (
              <div>
                <dt>Нижний / верхний марш</dt>
                <dd>
                  {solver.lowerStepCount} / {solver.upperStepCount}
                </dd>
              </div>
            )}
            {solver.landingWidth !== undefined && (
              <div>
                <dt>Ширина площадки</dt>
                <dd>{fmt.mm(solver.landingWidth)}</dd>
              </div>
            )}
            {solver.outerRadius !== undefined && (
              <div>
                <dt>Наружный радиус</dt>
                <dd>{fmt.mm(solver.outerRadius)}</dd>
              </div>
            )}
            {solver.comfortStep !== undefined && (
              <div>
                <dt>Шаг комфорта</dt>
                <dd>{fmt.mm(solver.comfortStep)}</dd>
              </div>
            )}
          </dl>
        </section>
      )}

      {geometry && (
        <section className="panel">
          <h2>Габариты и объём</h2>
          <dl className="kv">
            <div>
              <dt>Деталей</dt>
              <dd>{geometry.solid_count}</dd>
            </div>
            <div>
              <dt>Объём</dt>
              <dd>{fmt.m3(geometry.volume_mm3)}</dd>
            </div>
            <div>
              <dt>Площадь поверхности</dt>
              <dd>{fmt.m2(geometry.surface_area_mm2)}</dd>
            </div>
          </dl>
        </section>
      )}

      {pricing && (
        <section className="panel">
          <div className="price-box">
            <span className="price-label">Предварительная цена</span>
            <span className="price-value">{fmt.rubMajor(pricing.final_price_rub)}</span>
          </div>
          {material && <p className="sub">Материал: {materialLabel(material)}</p>}
          <p className="sub">
            Точная стоимость зависит от согласования проекта. Цена включает материалы, обработку
            и монтаж; финальный расчёт подтвердит менеджер. Доставка рассчитывается индивидуально.
          </p>
        </section>
      )}
    </>
  )
}