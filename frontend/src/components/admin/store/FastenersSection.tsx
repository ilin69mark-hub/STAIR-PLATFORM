// Раздел «Магазин» → Крепёж: прайс дюбелей и глухарей в рублях за штуку
// (MNT-0001, решение владельца 2026-10-08).
//
// Отдельный экран, а не ещё одна колонка в «Ценах материалов», по двум
// причинам. Первая — единица: материал в рублях за килограмм, крепёж в
// рублях за штуку, и в одной таблице «цена» означала бы то одно, то
// другое. Вторая — количество: позиций крепежа около сорока, материалов
// пять, и общий список на 45 строк перестал бы читаться.

import { useMemo, useState } from 'react'
import { storeApi, type FastenerPosition, type FastenerPrice } from '../../../api/store'
import { useLoad } from '../../../lib/useLoad'
import { ErrorBox, Loading } from './SectionFeedback'

type Status = 'idle' | 'loading' | 'saving' | 'saved' | 'error'

type KindFilter = 'all' | 'screw' | 'plug'

const KIND_LABELS: Record<string, string> = {
  all: 'Все позиции',
  screw: 'Глухари',
  plug: 'Дюбели',
}

// Материал, в который крепёж вворачивают. Пустое значение — «не задан», и
// это НЕ бетон: так же, как на сервере (FastenerBase).
const BASE_LABELS: Record<string, string> = {
  '': 'Не задан',
  concrete: 'Бетон (через дюбель)',
  wood: 'Дерево',
  metal: 'Металл (стойка, шпилька)',
}

// Пустая форма добавления позиции. Держится объектом, а не пятью
// useState: полей много, и по одному на каждое форма рассыпается.
const EMPTY_POSITION: FastenerPosition = {
  code: '',
  name_ru: '',
  kind: 'screw',
  diameter_mm: 0,
  length_mm: 0,
  base: '',
  price_per_pc_rub: 0,
}

export function FastenersSection() {
  const { data, error, reload } = useLoad<FastenerPrice[]>(() => storeApi.fasteners(), [])
  const {
    data: ownPositions,
    error: ownError,
    reload: reloadOwn,
  } = useLoad<FastenerPosition[]>(() => storeApi.fastenerPositions(), [])
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [status, setStatus] = useState<Status>('idle')
  const [message, setMessage] = useState('')
  const [kind, setKind] = useState<KindFilter>('all')
  const [query, setQuery] = useState('')
  const [form, setForm] = useState<FastenerPosition>(EMPTY_POSITION)
  const [adding, setAdding] = useState(false)

  // useMemo идёт ДО ранних return: хуки нельзя вызывать условно, иначе
  // React ловит «Rendered more hooks than during the previous render» при
  // первой же загрузке данных.
  //
  // Фильтр по виду и поиск по названию/коду. Сортировка — по размеру, а не
  // по алфавиту: монтажник ищет «10х60», и в списке, отсортированном по коду,
  // это находится с трудом (PLUG-10x100 стоит раньше PLUG-10x60).
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    return (data ?? [])
      .filter((f) => kind === 'all' || f.kind === kind)
      .filter((f) => !q || f.name_ru.toLowerCase().includes(q) || f.code.toLowerCase().includes(q))
      .slice()
      .sort((a, b) => a.diameter_mm - b.diameter_mm || a.length_mm - b.length_mm)
  }, [data, kind, query])

  if (error) return <ErrorBox text={error} />
  if (!data) return <Loading />

  const priceOf = (f: FastenerPrice) => draft[f.code] ?? String(f.price_per_pc_rub)

  const save = async (code: string) => {
    setStatus('saving')
    setMessage('')
    try {
      const raw = priceOf(data.find((item) => item.code === code)!)
      if (raw.trim() === '') throw new Error('Укажите цену крепежа')
      const price = Number(raw)
      if (!Number.isFinite(price) || price <= 0) {
        throw new Error('Цена должна быть положительным числом')
      }
      await storeApi.setFastenerPrice(code, Math.round(price))
      setDraft((d) => {
        const next = { ...d }
        delete next[code]
        return next
      })
      setStatus('saved')
      setMessage(`Цена ${code} сохранена`)
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }

  const reset = async (code: string) => {
    setStatus('saving')
    setMessage('')
    try {
      await storeApi.resetFastenerPrice(code)
      setDraft((d) => {
        const next = { ...d }
        delete next[code]
        return next
      })
      setStatus('saved')
      setMessage(`Цена ${code} возвращена к встроенной ставке`)
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }

  // Проверка формы ДО похода на сервер. Дублирует серверную намеренно: сервер
  // остаётся единственным, кто решает, а форма не тратит поход на заведомо
  // негодные значения и говорит то же самыми словами.
  const positionError = (p: FastenerPosition): string | null => {
    if (p.code.trim() === '') return 'Укажите код позиции'
    if (p.name_ru.trim() === '') return 'Укажите название'
    if (!(p.diameter_mm > 0)) return 'Диаметр должен быть больше нуля'
    if (!(p.length_mm > 0)) return 'Длина должна быть больше нуля'
    if (!(p.price_per_pc_rub > 0)) return 'Цена должна быть больше нуля'
    return null
  }

  const addPosition = async () => {
    const problem = positionError(form)
    if (problem) {
      setStatus('error')
      setMessage(problem)
      return
    }
    setAdding(true)
    setStatus('saving')
    setMessage('')
    try {
      await storeApi.createFastenerPosition({
        ...form,
        code: form.code.trim(),
        name_ru: form.name_ru.trim(),
        price_per_pc_rub: Math.round(form.price_per_pc_rub),
      })
      setForm(EMPTY_POSITION)
      setAdding(false)
      setStatus('saved')
      setMessage(`Позиция ${form.code} добавлена`)
      // Перечитываются ОБА списка: позиция появилась и в своих позициях, и в
      // общем прайсе. Перечитывание только одного оставило бы её в списке
      // без цены — выглядело бы, как будто сохранение не сработало.
      reloadOwn()
      reload()
    } catch (e) {
      setAdding(false)
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }

  const removePosition = async (code: string) => {
    setStatus('saving')
    setMessage('')
    try {
      await storeApi.deleteFastenerPosition(code)
      setStatus('saved')
      setMessage(`Позиция ${code} удалена`)
      reloadOwn()
      reload()
    } catch (e) {
      setStatus('error')
      setMessage(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <section>
      <h2>Крепёж</h2>
      <p className="hint">
        Цена в ₽ за штуку. Крепёж попадает в смету автоматически: глухарь 6х35 на пятки
        подступенка и ступеней, глухарь 10х60 на пятку пола и пятку выхода на перекрытие
        (плюс дюбель 10х60, если основание бетонное), глухарь 6х35 на анкерные отверстия
        фланцев ограждения и саморез 5х32 на крепление поручня к стойкам. Нужного крепежа,
        которого нет в списке, можно добавить формой ниже — он попадёт в сметы этого
        магазина.
      </p>

      <h3>Добавить позицию</h3>
      <p className="hint">
        Код должен быть свой: совпавший со встроенной позицией код отклоняется, иначе
        «глухарь 6х35» с другим размером изменил бы расчёт у всех, кто считает по
        встроенному коду. Встроенную позицию удалить нельзя — ей меняют цену.
      </p>
      <div className="row--actions">
        <label className="field">
          <span className="field__label">Код</span>
          <input
            className="field__input"
            type="text"
            placeholder="SCREW-6x16"
            value={form.code}
            onChange={(e) => setForm((f) => ({ ...f, code: e.target.value }))}
          />
        </label>
        <label className="field">
          <span className="field__label">Название</span>
          <input
            className="field__input"
            type="text"
            placeholder="Глухарь 6х16, оцинкованный"
            value={form.name_ru}
            onChange={(e) => setForm((f) => ({ ...f, name_ru: e.target.value }))}
          />
        </label>
        <label className="field">
          <span className="field__label">Вид</span>
          <select
            className="field__input"
            value={form.kind}
            onChange={(e) => setForm((f) => ({ ...f, kind: e.target.value }))}
          >
            <option value="screw">Глухарь / саморез</option>
            <option value="plug">Дюбель</option>
          </select>
        </label>
        <label className="field">
          <span className="field__label">Диаметр, мм</span>
          <input
            className="field__input"
            type="number"
            min={0}
            step={0.1}
            value={form.diameter_mm || ''}
            onChange={(e) => setForm((f) => ({ ...f, diameter_mm: Number(e.target.value) }))}
          />
        </label>
        <label className="field">
          <span className="field__label">Длина, мм</span>
          <input
            className="field__input"
            type="number"
            min={0}
            step={0.1}
            value={form.length_mm || ''}
            onChange={(e) => setForm((f) => ({ ...f, length_mm: Number(e.target.value) }))}
          />
        </label>
        <label className="field">
          <span className="field__label">Материал</span>
          <select
            className="field__input"
            value={form.base}
            onChange={(e) => setForm((f) => ({ ...f, base: e.target.value }))}
          >
            {Object.entries(BASE_LABELS).map(([v, label]) => (
              <option key={v} value={v}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span className="field__label">Цена, ₽/шт</span>
          <input
            className="field__input"
            type="number"
            min={1}
            step={1}
            value={form.price_per_pc_rub || ''}
            onChange={(e) => setForm((f) => ({ ...f, price_per_pc_rub: Number(e.target.value) }))}
          />
        </label>
        <button type="button" onClick={() => void addPosition()} disabled={adding}>
          Добавить
        </button>
      </div>

      <h3>Свои позиции</h3>
      {ownError && <ErrorBox text={ownError} />}
      {!ownPositions && <Loading />}
      {ownPositions && ownPositions.length === 0 && (
        <p className="hint">
          Своих позиций нет: считается встроенный каталог движка.
        </p>
      )}
      {ownPositions && ownPositions.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Позиция</th>
              <th>Код</th>
              <th>Вид</th>
              <th>Размер, мм</th>
              <th>Материал</th>
              <th>Цена, ₽/шт</th>
              <th>Действия</th>
            </tr>
          </thead>
          <tbody>
            {ownPositions.map((p) => (
              <tr key={p.code}>
                <td>{p.name_ru}</td>
                <td>
                  <code>{p.code}</code>
                </td>
                <td>{p.kind === 'plug' ? 'Дюбель' : 'Глухарь / саморез'}</td>
                <td>
                  {p.diameter_mm} × {p.length_mm}
                </td>
                <td>{BASE_LABELS[p.base] ?? p.base}</td>
                <td>{p.price_per_pc_rub}</td>
                <td className="actions">
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => void removePosition(p.code)}
                    disabled={status === 'saving'}
                  >
                    Удалить
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h3>Цены</h3>
      <div className="row--actions">
        <label className="field">
          <span className="field__label">Вид</span>
          <select
            className="field__input"
            value={kind}
            onChange={(e) => setKind(e.target.value as KindFilter)}
          >
            {(Object.keys(KIND_LABELS) as KindFilter[]).map((k) => (
              <option key={k} value={k}>
                {KIND_LABELS[k]}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span className="field__label">Поиск</span>
          <input
            className="field__input"
            type="search"
            placeholder="10х60, дюбель, глухарь…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
      </div>

      <table>
        <thead>
          <tr>
            <th>Позиция</th>
            <th>Код</th>
            <th>Цена, ₽/шт</th>
            <th>Источник</th>
            <th>Действия</th>
          </tr>
        </thead>
        <tbody>
          {visible.map((f) => (
            <tr key={f.code}>
              <td>{f.name_ru}</td>
              <td>
                <code>{f.code}</code>
              </td>
              <td>
                <input
                  aria-label={`Цена ${f.name_ru}`}
                  type="number"
                  min={1}
                  step={1}
                  value={priceOf(f)}
                  onChange={(e) => setDraft((d) => ({ ...d, [f.code]: e.target.value }))}
                />
              </td>
              <td>{f.overridden ? 'задана магазином' : 'встроенная ставка'}</td>
              <td className="actions">
                <button type="button" onClick={() => void save(f.code)} disabled={status === 'saving'}>
                  Сохранить
                </button>
                {f.overridden && (
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => void reset(f.code)}
                    disabled={status === 'saving'}
                  >
                    Сбросить
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {visible.length === 0 && (
        <p className="hint">
          {query ? `Ничего не найдено по запросу «${query}».` : 'Под этот фильтр ничего не попало.'}
        </p>
      )}
      {message && <p className={status === 'error' ? 'error' : 'ok'}>{message}</p>}
    </section>
  )
}