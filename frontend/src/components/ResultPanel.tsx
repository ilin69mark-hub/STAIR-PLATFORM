import { lazy, Suspense } from 'react'
import type { Pricing, Snapshot } from '../api/types'
import { fmt } from '../lib/format'
import { exportCsv } from '../lib/export'
import { StairProfile } from './schemes/StairProfile'
import { NestingMap } from './schemes/NestingMap'

const GeometryViewer = lazy(() =>
  import('./viewer/GeometryViewer').then((m) => ({ default: m.GeometryViewer })),
)

interface Props {
  snapshot: Snapshot
}

export function ResultPanel({ snapshot }: Props) {
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

      <ValidationPanel issues={s.validation.Issues} />

      {!stopped && (
        <>
          <FlightPanel snapshot={s} />
          <GeometryPanel snapshot={s} />
          <ManufacturingPanel snapshot={s} />
          <PricingPanel snapshot={s} pricing={s.pricing!} />
        </>
      )}
    </div>
  )
}

function ValidationPanel({ issues }: { issues: Snapshot['validation']['Issues'] }) {
  if (!issues || issues.length === 0) {
    return (
      <section className="panel">
        <h2 className="panel__title">Валидация</h2>
        <p className="ok-text">Нарушений не обнаружено.</p>
      </section>
    )
  }
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
            <th>Рекомендация</th>
          </tr>
        </thead>
        <tbody>
          {issues.map((i, idx) => (
            <tr key={idx} className={i.Severity === 'error' ? 'row--error' : ''}>
              <td>{i.Code}</td>
              <td>{i.Severity}</td>
              <td>{i.Element}</td>
              <td>{i.Message}</td>
              <td>{i.Fix ?? '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
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
      <StairProfile flight={f} />
    </section>
  )
}

function GeometryPanel({ snapshot }: { snapshot: Snapshot }) {
  const m = snapshot.measurement!
  return (
    <section className="panel">
      <h2 className="panel__title">Геометрия</h2>
      <dl className="kv">
        <div>
          <dt>Твёрдых тел</dt>
          <dd>{m.SolidCount}</dd>
        </div>
        <div>
          <dt>Объём</dt>
          <dd>{fmt.mm3(m.Volume)}</dd>
        </div>
        <div>
          <dt>Площадь поверхности</dt>
          <dd>{fmt.mm2(m.SurfaceArea)}</dd>
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
          <GeometryViewer mesh={snapshot.mesh} />
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
          <dd>{fmt.mm2(nesting.PartArea)}</dd>
        </div>
        <div>
          <dt>Площадь листов</dt>
          <dd>{fmt.mm2(nesting.SheetArea)}</dd>
        </div>
        <div>
          <dt>Отходы</dt>
          <dd>{fmt.mm2(nesting.WasteArea)}</dd>
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