import { useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react'

// Аккордеон панели калькулятора.
//
// Четыре свойства, которые важны и на которые стоит смотреть при правках:
//
//  1. СЕКЦИЯ ВСЕГДА ВИДНА. Закрытая показывает только заголовок, но НЕ исчезает:
//     пользователь должен видеть, что настройки вообще есть, и куда вернуться.
//     Скрывать секцию целиком — нельзя.
//
//  2. ОДНА ОТКРЫТА. Нажатие на закрытую секцию закрывает раскрытую. Отдельного
//     «развернуть всё» нет намеренно: 6 открытых секций в узкой колонке —
//     это вечный скролл, а задача панели — уместить управление в один экран.
//
//  3. ВСЕ МОГУТ БЫТЬ СВЕРНУТЫ. Повторное нажатие на открытую секцию закрывает
//     и её — состояние «ничего не раскрыто» достижимо. Изначально здесь была
//     оговорка, что единственную открытую секцию закрывать нельзя («иначе
//     панель выглядит поломанной»); по требованию владельца продукта убрана:
//     свёрнутый список из пяти заголовков — законное и компактное состояние,
//     в котором пользователь сам открывает нужный раздел.
//
//  4. ЗАГОЛОВКИ — ЧЕТЫРЕ ОДИНАКОВЫЕ ВКЛАДКИ 2×2, ТЕЛО — ОДНО ПОД НИМИ.
//     Раньше каждая секция была отдельной карточкой (заголовок + своё тело),
//     пять карточек занимали пять рядов по 38px, и на тело открытой секции
//     оставалось около 280px при ширине 356px — данные приходилось вводить в
//     узкое окошко со скроллом внутри скролла. Теперь заголовки — компактная
//     сетка, а под ними одно тело на всю оставшуюся высоту панели.
//
//     Порядок вкладок ФИКСИРОВАН (основные | ограждение / цвет | помещение) и
//     все четыре прямоугольника ОДИНАКОВЫЕ: открытая не растягивается на всю
//     ширину и не прыгает наверх. Так сделано по требованию владельца: с
//     перестановкой заголовков человек терял из виду раздел, который только
//     что открывал, — искать приходилось заново каждый раз. Активная вкладка
//     выглядит как выбранный чип Segmented (зелёная заливка и рамка), чтобы
//     «где я сейчас» читалось без догадок по знаку +/−.
//
//     Когда вкладок пять (L/П-марш добавляет «Настройки поворота» в начало),
//     нижняя строка остаётся полупустой, и последняя вкладка растягивается
//     на всю ширину — см. .acc__head:last-child:nth-child(odd) в controls.css.
//
//  6. УКАЗАТЕЛЬ «СНИЗУ ЕСТЬ ЕЩЁ». Панель не скроллится, скроллится тело
//     открытой секции, поэтому длинный раздел («Цвет и материал») обрезан
//     молча: низ выглядит как конец, и поля под ним не видно. У нижнего края
//     тела стоит маленький кружок со стрелкой вниз, и он УХОДИТ, как только
//     пользователь докрутил до низа (требование владельца) — то есть он
//     честно отвечает «есть ещё», а не висит украшением.
//
//     Сам указатель лежит ВНЕ .acc__body, в .acc__body-wrap: position:
//     absolute внутри прокручиваемого контейнера уезжает вместе с
//     содержимым, и стрелка уехала бы с экрана на первом же прокрутке.
//
// Разметка повторяет референс: заголовок + знак «+»/«−». Знак — не декорация,
// он показывает состояние при мгновенном закрытии анимации.

export interface AccordionSection {
  id: string
  title: string
  /** Скрывать секцию целиком, если условие не выполнено (например, поворот
   *  есть только у L/П-маршей). При false секция показывается, но тело пустое. */
  hidden?: boolean
  content: ReactNode
}

export interface AccordionProps {
  sections: AccordionSection[]
  /** id открытой секции; uncontrolled-состояние живёт внутри. */
  defaultOpen?: string
  onChange?: (id: string | null) => void
}

export function Accordion({ sections, defaultOpen, onChange }: AccordionProps) {
  const [open, setOpen] = useState<string | null>(defaultOpen ?? sections.find((s) => !s.hidden)?.id ?? null)
  const bodyRef = useRef<HTMLDivElement>(null)
  const [more, setMore] = useState(false)
  const baseId = useId()

  // Показываем указатель «снизу есть ещё» ровно тогда, когда низ не виден.
  // Эффект без зависимостей — намеренно: длина содержимого живёт (выбрал
  // материал — добавились поля, сменил марш — секция стала короче), и решать
  // это по одному id секции нельзя. setMore с тем же значением ререндер не
  // вызывает, поэтому меряем дёшево и на каждый рендер.
  useLayoutEffect(() => {
    const el = bodyRef.current
    if (!el) {
      setMore(false)
      return
    }
    // 12px допуска: остаток в 1–2px — округление, а не недоступный контент.
    const update = () => setMore(el.scrollHeight - el.scrollTop - el.clientHeight > 12)
    update()
    el.addEventListener('scroll', update, { passive: true })
    window.addEventListener('resize', update)
    return () => {
      el.removeEventListener('scroll', update)
      window.removeEventListener('resize', update)
    }
  })

  // Повторное нажатие на открытую секцию закрывает её: свернуть всё —
  // разрешённое состояние. Иначе «свернуть» означало бы «свернуть нельзя».
  const toggle = (id: string) => {
    setOpen((cur) => {
      const next = cur === id ? null : id
      onChange?.(next)
      return next
    })
  }

  // Скрытые секции фильтруем, а не пропускаем в разметке, иначе
  // «Настройки поворота» (только L/П) оставляла бы дыру в сетке.
  const visible = sections.filter((s) => !s.hidden)
  const current = visible.find((s) => s.id === open)

  return (
    <div className="accordion">
      <div className="accordion__heads">
        {visible.map((s) => {
          const isOpen = s.id === open
          return (
            <button
              type="button"
              // Открытый заголовок занимает всю ширину сетки: он связан с
              // телом под ней, и заодно закрывает «дыру» неполного ряда.
              className={`acc__head${isOpen ? ' is-open' : ''}`}
              key={s.id}
              aria-expanded={isOpen}
              aria-controls={`${baseId}-${s.id}`}
              onClick={() => toggle(s.id)}
            >
              <span className="acc__title">{s.title}</span>
              <span className="acc__sign" aria-hidden="true">
                {isOpen ? '−' : '+'}
              </span>
            </button>
          )
        })}
      </div>
      {current && (
        <div className="acc__body-wrap">
          <div
            className="acc__body"
            id={`${baseId}-${current.id}`}
            role="region"
            aria-label={current.title}
            ref={bodyRef}
          >
            {current.content}
          </div>
          {more && (
            <div className="acc__more" aria-hidden="true">
              <span className="acc__more-dot">
                <svg viewBox="0 0 16 16" width="11" height="11" focusable="false">
                  <path
                    d="M3.5 6.5 8 11l4.5-4.5"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
