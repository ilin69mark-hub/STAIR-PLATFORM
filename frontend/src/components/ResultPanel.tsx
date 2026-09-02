import { lazy, Suspense, useState } from 'react'
import type { Pricing, Snapshot, Variation } from '@shared/types'
import { fmt } from '@shared/format'
import { exportCsv } from '../lib/export'
import { StairProfile } from '@shared/schemes/StairProfile'
import { StairPlan } from '@shared/schemes/StairPlan'
import { NestingMap } from '@shared/schemes/NestingMap'
import { VariationPicker } from '@shared/components/VariationPicker'
import { schematicOf } from './snapshotView'

const GeometryViewer = lazy(() =>
  import('@shared/viewer/GeometryViewer').then((m) => ({ default: m.GeometryViewer })),
)

interface Props {
  snapshot: Snapshot
  // onApplyVariation — применить выбранный вариант (A/B/C) как превью.
  onApplyVariation?: (v: Variation) => void
  activeVariantId?: string
}

export function ResultPanel({ snapshot, onApplyVariation, activeVariantId }: Props) {
  const s = snapshot
  const stopped = !s.manufacturing || !s.pricing

  return (
    <div className="results">
      <section className="panel">
        <h2 className="panel__title">
          Результат расчёта {s.validation.Valid ? '✅' : '⚠️'}
        </h2>
        <p className="muted">
          {s.validation.Blocking
            ? 'Конвейер остановлен: обнаружены блокирующие нарушения.'
            : 'Конвейер выполнен полностью.'}
        </p>
      </section>

      <ValidationPanel
        issues={s.validation.Issues}
        onApplyVariation={onApplyVariation}
        activeVariantId={activeVariantId}
      />

      {!stopped && (
        <>
          {s.lshape ? (
            <LShapePanel snapshot={s} />
          ) : s.ushape ? (
            <UShapePanel snapshot={s} />
          ) : s.spiral ? (
            <SpiralPanel snapshot={s} />
          ) : (
            <FlightPanel snapshot={s} />
          )}
          <GeometryPanel snapshot={s} />
          <ManufacturingPanel snapshot={s} />
          <PricingPanel snapshot={s} pricing={s.pricing!} />
        </>
      )}
    </div>
  )
}

// SolverDrawings — чертёжная секция результата Solver: вкладки «Профиль»
// (боковой вид), «План» (вид сверху, все типы маршей) и «3D» (меш, если есть).
function SolverDrawings({ snapshot }: { snapshot: Snapshot }) {
  const [tab, setTab] = useState<'profile' | 'plan' | 'threed'>('profile')
  const sch = schematicOf(snapshot)
  if (!sch) return null

  // Фиолетовая линия верха марша на 3D: суммарный подъём марша.
  const stairTop =
    sch.flight && sch.kind !== 'spiral'
      ? { rise: sch.flight.StepHeight * sch.flight.StepCount }
      : undefined

  // Профиль (вид сбоку) осмыслен только для прямого марша; L/U/спираль
  // показываем планом по умолчанию.
  const activeTab = sch.kind === 'straight' ? tab : tab === 'profile' ? 'plan' : tab
  const tabs: Array<['profile' | 'plan' | 'threed', string]> =
    sch.kind === 'straight'
      ? [['profile', 'Профиль'], ['plan', 'План'], ['threed', '3D']]
      : [['plan', 'План'], ['threed', '3D']]

  return (
    <div className="draw">
      <div className="draw__tabs" role="tablist">
        {tabs.map(([key, label]) => (
          <button
            key={key}
            type="button"
            role="tab"
            aria-selected={activeTab === key}
            className={`draw__tab${activeTab === key ? ' draw__tab--active' : ''}`}
            onClick={() => setTab(key)}
          >
            {label}
          </button>
        ))}
      </div>
      {activeTab === 'profile' && (
        <StairProfile flight={sch.flight} railing={sch.railing} />
      )}
      {activeTab === 'plan' && (
        <StairPlan flight={sch.flight} kind={sch.kind} solver={sch.solver} />
      )}
      {activeTab === 'threed' && (
        snapshot.mesh ? (
          <Suspense fallback={<p className="muted">Загрузка 3D…</p>}>
            <GeometryViewer
              mesh={snapshot.mesh}
              roomMesh={snapshot.room_mesh}
              railingMesh={snapshot.railing_mesh}
              stairTop={stairTop}
              flight={sch?.kind}
              direction={sch?.solver?.direction}
              roomWidth={sch?.solver?.roomWidth}
              roomLength={sch?.solver?.roomLength}
              approachSpace={sch?.solver?.approachSpace}
            />
          </Suspense>
        ) : (
          <p className="muted">Модель 3D недоступна для этого снапшота.</p>
        )
      )}
    </div>
  )
}

function ValidationPanel({
  issues,
  onApplyVariation,
  activeVariantId,
}: {
  issues: Snapshot['validation']['Issues']
  onApplyVariation?: (v: Variation) => void
  activeVariantId?: string
}) {
  if (!issues || issues.length === 0) {
    return (
      <section className="panel">
        <h2 className="panel__title">Валидация</h2>
        <p className="ok-text">Нарушений не обнаружено.</p>
      </section>
    )
  }
  // Готовые варианты (advisor) и вариации (A/B/C) могут относиться сразу к
  // нескольким issue с одинаковым набором — чтобы не дублировать списки,
  // показываем их только для ПЕРВОГО подходящего issue.
  const firstSugg = issues.find((i) => i.Suggestions && i.Suggestions.length > 0)
  const firstVar = issues.find((i) => i.Variations && i.Variations.length > 0)
  return (
    <section className="panel">
      <h2 className="panel__title">Валидация</h2>
      <table className="table">
        <thead>
          <tr>
            <th>Код</th>
            <th>Severity</th>
            <th>Элемент</th>
            <th>Сообщение</th>
            <th>Что поправить</th>
            <th>Рекомендация</th>
          </tr>
        </thead>
        <tbody>
          {issues.map((i, idx) => (
            <tr key={idx} className={i.Severity === 'error' ? 'row--error' : ''}>
              <td>{i.Code}</td>
              <td>{i.Severity}</td>
              <td>{i.Element}</td>
              <td>{i.Guide ?? i.Message}</td>
              <td>{i.Param ?? '—'}</td>
              <td>{i.Fix ?? '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {firstSugg && (
        <h3 className="panel__sub">Подходящие варианты конфигурации</h3>
      )}
      {firstSugg && (
        <div className="issue-suggestions">
          {firstSugg.Suggestions!.map((s, si) => (
            <div className="suggestion" key={si}>
              <span>
                {s.StepCount} ступ. · h {fmt.mm(s.StepHeightMm)} · проступь{' '}
                {fmt.mm(s.TreadDepthMm)} · угол {s.AngleDeg.toFixed(1)}°
              </span>
            </div>
          ))}
        </div>
      )}
      {firstVar && onApplyVariation && (
        <div className="variations">
          <h3 className="panel__sub">Варианты решения (выберите подходящий)</h3>
          <VariationPicker
            variations={firstVar.Variations!}
            onApply={onApplyVariation}
            activeId={activeVariantId}
          />
        </div>
      )}
    </section>
  )
}

function FlightPanel({ snapshot }: { snapshot: Snapshot }) {
  const f = snapshot.flight!
  return (
    <section className="panel">
      <h2 className="panel__title">Марш (Solver)</h2>
      <dl className="kv">
        <div>
          <dt>Ступеней</dt>
          <dd>{f.StepCount}</dd>
        </div>
        <div>
          <dt>Высота ступени</dt>
          <dd>{fmt.mm(f.StepHeight)}</dd>
        </div>
        <div>
          <dt>Глубина проступи</dt>
          <dd>{fmt.mm(f.TreadDepth)}</dd>
        </div>
        <div>
          <dt>Горизонтальная проекция</dt>
          <dd>{fmt.mm(f.Run)}</dd>
        </div>
        <div>
          <dt>Длина косоура</dt>
          <dd>{fmt.mm(f.Stringer)}</dd>
        </div>
        <div>
          <dt>Угол наклона</dt>
          <dd>{fmt.deg(f.Angle)}</dd>
        </div>
      </dl>
      <SolverDrawings snapshot={snapshot} />
    </section>
  )
}

// LShapePanel — результат Solver L-образной лестницы (EDR-0005):
// два марша, площадка между ними на высоте H1.
function LShapePanel({ snapshot }: { snapshot: Snapshot }) {
  const l = snapshot.lshape!
  return (
    <section className="panel">
      <h2 className="panel__title">L-образный марш (Solver)</h2>
      <dl className="kv">
        <div>
          <dt>Ступеней всего</dt>
          <dd>{l.StepCount}</dd>
        </div>
        <div>
          <dt>Нижний / верхний марш</dt>
          <dd>
            {l.LowerStepCount} / {l.UpperStepCount}
          </dd>
        </div>
        <div>
          <dt>Высота ступени</dt>
          <dd>{fmt.mm(l.StepHeight)}</dd>
        </div>
        <div>
          <dt>Глубина проступи</dt>
          <dd>{fmt.mm(l.TreadDepth)}</dd>
        </div>
        <div>
          <dt>Угол наклона</dt>
          <dd>{fmt.deg(l.Angle)}</dd>
        </div>
        <div>
          <dt>Площадка на высоте H1</dt>
          <dd>{fmt.mm(l.LowerHeight)}</dd>
        </div>
        <div>
          <dt>Ширина площадки Wp</dt>
          <dd>{fmt.mm(l.LandingWidth)}</dd>
        </div>
        {l.LandingDepth !== undefined && (
          <div>
            <dt>Глубина площадки</dt>
            <dd>{fmt.mm(l.LandingDepth)}</dd>
          </div>
        )}
        {l.RoomWidth !== undefined && l.RoomLength !== undefined && (
          <div>
            <dt>Помещение (X × Y)</dt>
            <dd>
              {fmt.mm(l.RoomWidth)} × {fmt.mm(l.RoomLength)}
            </dd>
          </div>
        )}
        <div>
          <dt>Нижний марш (L1 × R1)</dt>
          <dd>
            {fmt.mm(l.LowerRun)} × {fmt.mm(l.LowerStringer)}
          </dd>
        </div>
        <div>
          <dt>Верхний марш (L2 × R2)</dt>
          <dd>
            {fmt.mm(l.UpperRun)} × {fmt.mm(l.UpperStringer)}
          </dd>
        </div>
        <div>
          <dt>Общая высота H</dt>
          <dd>{fmt.mm(l.UpperHeight)}</dd>
        </div>
      </dl>
      <SolverDrawings snapshot={snapshot} />
    </section>
  )
}
// UShapePanel — результат Solver П-образной лестницы (EDR-0006):
// два параллельных марша, площадка между ними на высоте H1.
function UShapePanel({ snapshot }: { snapshot: Snapshot }) {
  const u = snapshot.ushape!
  return (
    <section className="panel">
      <h2 className="panel__title">П-образный марш (Solver)</h2>
      <dl className="kv">
        <div>
          <dt>Ступеней всего</dt>
          <dd>{u.StepCount}</dd>
        </div>
        <div>
          <dt>Нижний / верхний марш</dt>
          <dd>
            {u.LowerStepCount} / {u.UpperStepCount}
          </dd>
        </div>
        <div>
          <dt>Высота ступени</dt>
          <dd>{fmt.mm(u.StepHeight)}</dd>
        </div>
        <div>
          <dt>Глубина проступи</dt>
          <dd>{fmt.mm(u.TreadDepth)}</dd>
        </div>
        <div>
          <dt>Угол наклона</dt>
          <dd>{fmt.deg(u.Angle)}</dd>
        </div>
        <div>
          <dt>Площадка на высоте H1</dt>
          <dd>{fmt.mm(u.LowerHeight)}</dd>
        </div>
        <div>
          <dt>Ширина площадки Wp</dt>
          <dd>{fmt.mm(u.LandingWidth)}</dd>
        </div>
        <div>
          <dt>Нижний марш (L1 × R1)</dt>
          <dd>
            {fmt.mm(u.LowerRun)} × {fmt.mm(u.LowerStringer)}
          </dd>
        </div>
        <div>
          <dt>Верхний марш (L2 × R2)</dt>
          <dd>
            {fmt.mm(u.UpperRun)} × {fmt.mm(u.UpperStringer)}
          </dd>
        </div>
        <div>
          <dt>Общая высота H</dt>
          <dd>{fmt.mm(u.UpperHeight)}</dd>
        </div>
      </dl>
      <SolverDrawings snapshot={snapshot} />
    </section>
  )
}

// SpiralPanel — результат Solver спиральной лестницы с центральной колонной
// (EDR-0007): веерные проступи вокруг оси Z, полный поворот 360°.
function SpiralPanel({ snapshot }: { snapshot: Snapshot }) {
  const sp = snapshot.spiral!
  return (
    <section className="panel">
      <h2 className="panel__title">Спиральный марш (Solver)</h2>
      <dl className="kv">
        <div>
          <dt>Ступеней (полный поворот 360°)</dt>
          <dd>{sp.StepCount}</dd>
        </div>
        <div>
          <dt>Высота ступени</dt>
          <dd>{fmt.mm(sp.StepHeight)}</dd>
        </div>
        <div>
          <dt>Наружный радиус R</dt>
          <dd>{fmt.mm(sp.OuterRadius)}</dd>
        </div>
        <div>
          <dt>Радиус колонны r</dt>
          <dd>{fmt.mm(sp.ColumnRadius)}</dd>
        </div>
        <div>
          <dt>Радиус линии хода r_ход</dt>
          <dd>{fmt.mm(sp.WalkRadius)}</dd>
        </div>
        <div>
          <dt>Проступь у колонны / по линии хода / у кромки</dt>
          <dd>
            {fmt.mm(sp.InnerTread)} / {fmt.mm(sp.WalkTread)} / {fmt.mm(sp.OuterTread)}
          </dd>
        </div>
        <div>
          <dt>Угол подъёма</dt>
          <dd>{fmt.deg(sp.Angle)}</dd>
        </div>
        <div>
          <dt>Угловой шаг / полный угол</dt>
          <dd>
            {fmt.deg(sp.AngularStep)} / {fmt.deg(sp.AngularTotal)}
          </dd>
        </div>
        <div>
          <dt>Длина дуги по наружной кромке</dt>
          <dd>{fmt.mm(sp.ArcLength)}</dd>
        </div>
        <div>
          <dt>Шаг комфорта S = 2h + b_ход</dt>
          <dd>{fmt.mm(sp.ComfortStep)}</dd>
        </div>
      </dl>
      <SolverDrawings snapshot={snapshot} />
    </section>
  )
}

function GeometryPanel({ snapshot }: { snapshot: Snapshot }) {
  const m = snapshot.measurement!
  const sch = schematicOf(snapshot)
  // Фиолетовая линия верха марша на 3D (прямой марш): суммарный подъём и ширина.
  const stairTop =
    snapshot.flight
      ? {
          rise: snapshot.flight.StepHeight * snapshot.flight.StepCount,
        }
      : undefined

  return (
    <section className="panel">
      <h2 className="panel__title">Геометрия</h2>
      <dl className="kv">
        <div>
          <dt>Деталей</dt>
          <dd>{m.SolidCount}</dd>
        </div>
        <div>
          <dt>Объём</dt>
          <dd>{fmt.m3(m.Volume)}</dd>
        </div>
        <div>
          <dt>Площадь поверхности</dt>
          <dd>{fmt.m2(m.SurfaceArea)}</dd>
        </div>
        <div>
          <dt>Габаритный бокс</dt>
          <dd>
            {m.BoundingBox.Min.X.toFixed(0)},{m.BoundingBox.Min.Y.toFixed(0)},{m.BoundingBox.Min.Z.toFixed(0)}
            {' — '}
            {m.BoundingBox.Max.X.toFixed(0)},{m.BoundingBox.Max.Y.toFixed(0)},{m.BoundingBox.Max.Z.toFixed(0)}
          </dd>
        </div>
      </dl>
      {snapshot.issue_count > 0 && (
        <p className="alert alert--warn">Геометрических замечаний: {snapshot.issue_count}</p>
      )}
      {snapshot.mesh && (
        <Suspense fallback={<p className="muted">Загрузка 3D…</p>}>
          <GeometryViewer
            mesh={snapshot.mesh}
            roomMesh={snapshot.room_mesh}
            railingMesh={snapshot.railing_mesh}
            stairTop={stairTop}
            flight={sch?.kind}
            direction={sch?.solver?.direction}
            roomWidth={sch?.solver?.roomWidth}
            roomLength={sch?.solver?.roomLength}
            approachSpace={sch?.solver?.approachSpace}
          />
        </Suspense>
      )}
    </section>
  )
}

function ManufacturingPanel({ snapshot }: { snapshot: Snapshot }) {
  const mfg = snapshot.manufacturing!
  const nesting = mfg.Nesting

  return (
    <section className="panel">
      <h2 className="panel__title">Производство</h2>

      <h3 className="panel__sub">Детали ({mfg.Parts.length})</h3>
      <table className="table">
        <thead>
          <tr>
            <th>№</th>
            <th>Тип</th>
            <th>Материал</th>
            <th>Толщина</th>
            <th>Длина</th>
            <th>Ширина</th>
          </tr>
        </thead>
        <tbody>
          {mfg.Parts.map((p) => (
            <tr key={p.Number}>
              <td>{p.Number}</td>
              <td>{p.Kind}</td>
              <td>{p.Material}</td>
              <td>{fmt.mm(p.Thickness)}</td>
              <td>{fmt.mm(p.Length)}</td>
              <td>{fmt.mm(p.Width)}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <h3 className="panel__sub">Спецификация (BOM)</h3>
      <table className="table">
        <thead>
          <tr>
            <th>№</th>
            <th>Деталь</th>
            <th>Описание</th>
            <th>Кол-во</th>
            <th>Размер</th>
          </tr>
        </thead>
        <tbody>
          {mfg.BOM.Lines.map((l) => (
            <tr key={l.Number}>
              <td>{l.Number}</td>
              <td>{l.PartNumber}</td>
              <td>{l.Description}</td>
              <td>{l.Quantity}</td>
              <td>
                {fmt.mm(l.Length)} × {fmt.mm(l.Width)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <h3 className="panel__sub">Раскрой (листов: {nesting.Sheets.length})</h3>
      <dl className="kv">
        <div>
          <dt>Деталей</dt>
          <dd>{nesting.PartCount}</dd>
        </div>
        <div>
          <dt>Площадь деталей</dt>
          <dd>{fmt.m2(nesting.PartArea)}</dd>
        </div>
        <div>
          <dt>Площадь листов</dt>
          <dd>{fmt.m2(nesting.SheetArea)}</dd>
        </div>
        <div>
          <dt>Отходы</dt>
          <dd>{fmt.m2(nesting.WasteArea)}</dd>
        </div>
        <div>
          <dt>Использование</dt>
          <dd>{fmt.pct(nesting.Utilization)}</dd>
        </div>
      </dl>
      <NestingMap nesting={nesting} />

      <div className="row row--actions">
        <button className="btn" onClick={() => exportCsv.bom(snapshot, mfg)}>
          Экспорт BOM (CSV)
        </button>
        <button className="btn" onClick={() => exportCsv.parts(snapshot, mfg)}>
          Экспорт деталей (CSV)
        </button>
        <button className="btn" onClick={() => exportCsv.cutList(snapshot, mfg)}>
          Экспорт раскроя (CSV)
        </button>
      </div>
    </section>
  )
}

function PricingPanel({ snapshot, pricing }: { snapshot: Snapshot; pricing: Pricing }) {
  const p = pricing
  const rows: Array<[string, number | undefined]> = [
    ['Материалы', p.Material],
    ['Обработка (станки)', p.Machine],
    ['Труд', p.Labor],
    ['Накладные расходы', p.Overhead],
    ['Производственная себестоимость', p.ProductionCost],
    ['Маржа', p.Margin],
    ['Скидка', p.Discount],
    ['До налога', p.PreTax],
    ['НДС', p.Tax],
  ]
  return (
    <section className="panel">
      <h2 className="panel__title">Стоимость ({p.Currency.Code})</h2>
      <table className="table table--price">
        <tbody>
          {rows.map(([label, v]) => (
            <tr key={label}>
              <td>{label}</td>
              <td className="num">{fmt.rub(v, p.Currency.Decimals)}</td>
            </tr>
          ))}
          <tr className="row--total">
            <td>Итоговая цена</td>
            <td className="num">{fmt.rub(p.FinalPrice, p.Currency.Decimals)}</td>
          </tr>
        </tbody>
      </table>

      {p.Lines?.length > 0 && (
        <>
          <h3 className="panel__sub">Детализация</h3>
          <table className="table">
            <thead>
              <tr>
                <th>Наименование</th>
                <th>Категория</th>
                <th>Сумма</th>
                <th>Источник</th>
              </tr>
            </thead>
            <tbody>
              {p.Lines.map((l, i) => (
                <tr key={i}>
                  <td>{l.Name}</td>
                  <td>{l.Category}</td>
                  <td className="num">{fmt.rub(l.Amount, p.Currency.Decimals)}</td>
                  <td>{l.Source}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      <div className="row row--actions">
        <button className="btn" onClick={() => exportCsv.pricing(snapshot, pricing)}>
          Экспорт цен (CSV)
        </button>
      </div>
    </section>
  )
}