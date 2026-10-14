// Разделы «Магазин» → Материалы и Услуги: каталоги витрины, только чтение.
// Перенесено из отдельного приложения frontend-store-admin при слиянии
// кабинетов. Экран «Заказы» из того приложения НЕ перенесён: в админке
// уже есть один экран заказов — табличный (components/admin/OrdersPanel.tsx),
// он же показывается по праву orders.list.
//
// Оба каталога бьют в публичные эндпоинты /api/v1/public/*, поэтому в карте
// прав их needs = null — видны любому авторизованному.
import { storeApi, type MaterialCatalogItem, type PaymentTier } from '../../../api/store'
import { useLoad } from '../../../lib/useLoad'
import { ErrorBox, Loading } from './SectionFeedback'

// MaterialsSection — каталог материалов витрины (только чтение: цена меняется
// в разделе «Цены», параметры материала — в движке MFG-0005).
export function MaterialsSection() {
  const { data, error } = useLoad<MaterialCatalogItem[]>(() => storeApi.materials(), [])
  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />
  return (
    <section>
      <h2>Материалы</h2>
      <table>
        <thead>
          <tr>
            <th>Код</th>
            <th>Название</th>
            <th>Категория</th>
            <th>Плотность, кг/м³</th>
            <th>Толщина, мм</th>
            <th>Цена, ₽/кг</th>
          </tr>
        </thead>
        <tbody>
          {data.map((m) => (
            <tr key={m.code}>
              <td>
                <code>{m.code}</code>
              </td>
              <td>{m.name_ru || m.name}</td>
              <td>{m.category}</td>
              <td>{m.density_kg_m3}</td>
              <td>
                {m.min_thickness_mm}–{m.max_thickness_mm}
              </td>
              <td>{m.price_per_kg_rub}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}

// ServicesSection — платные услуги витрины (каталог tiers, этап 4).
export function ServicesSection() {
  const { data, error } = useLoad<PaymentTier[]>(() => storeApi.tiers(), [])
  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />
  return (
    <section>
      <h2>Услуги</h2>
      <table>
        <thead>
          <tr>
            <th>Код</th>
            <th>Название</th>
            <th>Описание</th>
            <th>Цена</th>
          </tr>
        </thead>
        <tbody>
          {data.map((t) => (
            <tr key={t.id}>
              <td>
                <code>{t.id}</code>
              </td>
              <td>{t.title}</td>
              <td>{t.description ?? '—'}</td>
              <td>
                {t.amount_rub} {t.currency}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
