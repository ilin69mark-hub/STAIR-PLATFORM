import { lazy, Suspense } from 'react'
import type { QuoteResult as QuoteResultType, QuoteSuggestion, Variation } from '@shared/types'
import { fmt } from '@shared/format'
import { materialLabel } from '@shared/config'
import { VariationPicker } from '@shared/components/VariationPicker'
import { elementLabel, severityLabel } from '@shared/validationText'
import { solverOf } from './quoteView'
import { ErrorBoundary } from '../ErrorBoundary'

// Результат публичного расчёта: марш, геометрия и предварительная цена.
// Покупателю показываем интерактивную 3D-модель (меш уже приходит в публичном
// ответе); производственный пакет (BOM/раскрой) остаётся в админке.
// three.js грузится лениво, чтобы не утяжелять основной бандл.

const GeometryViewer = lazy(() =>
  import('@shared/viewer/GeometryViewer').then((m) => ({ default: m.GeometryViewer })),
)

interface Props {
  quote: QuoteResultType
  onApplySuggestion?: (s: QuoteSuggestion) => void
  onApplyVariation?: (v: Variation) => void
  material?: string
  // Свободное пространство перед первой ступенью (мм), введено в калькуляторе
  // (дефолт 1000). Передаём в solverOf, чтобы не зависеть от сдвига модели.
  approachSpaceMM?: string
  // Персистентные вариации (A/B/C): список держит Конструктор между
  // пересчётами, поэтому активный вариант можно менять местами с текущим и
  // возвращаться обратно. Передаются как пропсы — в админке VariantPanel
  // использует собственный активный выбор (activeVariantId).
  variations?: Variation[] | null
  activeVariationId?: string | null
  // Высота марша из ввода пользователя (поле «Высота», мм): задаёт высоту
  // стен периметра в 3D-вьювере.
  heightMM?: number
}

export function QuoteResult({
  quote,
  onApplySuggestion,
  onApplyVariation,
  material,
  approachSpaceMM,
  variations,
  activeVariationId,
  heightMM,
}: Props) {
  const solver = solverOf(quote, approachSpaceMM != null && approachSpaceMM.trim() !== '' ? Number(approachSpaceMM) : undefined)
  const geometry = quote.geometry
  const pricing = quote.pricing
  const issues = quote.validation.issues ?? []
  const spiral = quote.spiral !== undefined
  // Вариации (A/B/C, напр. невписываемость в помещение) могут относиться к
  // нескольким issue с одинаковым набором — показываем только для первого,
  // если Конструктор не передал персистентный список.
  const firstVar = issues.find((i) => i.variations && i.variations.length > 0)
  // Фиолетовая линия верха марша на 3D: суммарный подъём марша.
  const stairTop =
    solver.flight && solver.kind !== 'spiral'
      ? { rise: solver.flight.StepHeight * solver.flight.StepCount }
      : undefined

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
                  <strong>{severityLabel(i.severity)}:</strong> {i.guide ?? i.message}
                  {!i.guide && i.fix ? ` (${i.fix})` : ''}
                </div>
                {i.param && (
                  <div className="issue-param">Что поправить: {elementLabel(i.param)}</div>
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

        {(variations ?? firstVar?.variations) && onApplyVariation && (
          <div className="variations">
            <h3 className="panel__sub">Варианты решения (выберите подходящий)</h3>
            <VariationPicker
              variations={variations ?? firstVar!.variations!}
              onApply={onApplyVariation}
              activeId={activeVariationId ?? undefined}
            />
          </div>
        )}

        {!quote.validation.blocking && quote.mesh?.Vertices?.length && quote.mesh?.Triangles && (
          <div className="scheme-3d">
            <h3 className="scheme-3d__title">3D-модель</h3>
            <ErrorBoundary
              fallback={
                <div className="alert alert--error" role="alert">
                  <p>Не удалось загрузить 3D-модель. Попробуйте перезагрузить страницу.</p>
                </div>
              }
            >
              <Suspense fallback={<p className="muted">Загрузка 3D…</p>}>
                <GeometryViewer
                mesh={quote.mesh}
                roomMesh={quote.room_mesh}
                railingMesh={quote.railing_mesh}
                stairTop={stairTop}
                flight={solver.kind}
                direction={solver.direction}
                roomWidth={solver.roomWidth}
                roomLength={solver.roomLength}
                approachSpace={solver.approachSpace}
                heightMM={heightMM}
              />
              </Suspense>
            </ErrorBoundary>
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