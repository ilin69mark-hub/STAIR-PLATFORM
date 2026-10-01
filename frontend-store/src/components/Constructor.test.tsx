import type { ReactNode } from 'react'
import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Constructor } from './Constructor'
import { quoteApi } from '../api/store'
import { renderWithAuth } from '../test/render'
import type { QuoteResult, Variation } from '@shared/types'

// Захват пропсов GeometryViewer: QuoteResult рендерится настоящим, а вьювер
// мокаем (WebGL в jsdom не строится). Проверяем связку «Высота (мм) в
// калькуляторе → QuoteResult → вьювер».
const viewerProps = vi.hoisted(() => ({ current: {} as Record<string, unknown> }))
vi.mock('@shared/viewer/GeometryViewer', () => ({
  GeometryViewer: (p: Record<string, unknown>) => {
    viewerProps.current = p
    return <div data-testid="mock-3d-viewer">{p.overlay as ReactNode}</div>
  },
}))

const mesh3d = {
  Vertices: [
    { X: 0, Y: 0, Z: 0 },
    { X: 900, Y: 0, Z: 0 },
    { X: 0, Y: 0, Z: 2700 },
  ],
  Triangles: [[0, 1, 2]] as Array<[number, number, number]>,
}

// blockedVariations — blocking-ответ с GEO-ANGLE, несущим одну вариацию
// (имитация того, что отдаёт бэкенд через variation.ForAngle).
function blockedVariations(cfg: Record<string, string>): QuoteResult {
  const v: Variation = {
    id: 'Угол 30°',
    title: 'Угол 30°',
    description: 'Сделать угол наклона в норме 30–45°.',
    config: cfg,
    fits: true,
    summary: 'Угол 30,2°, 18 ступ., h 167 мм, b 620 мм',
  }
  return {
    validation: {
      valid: false,
      blocking: true,
      issues: [
        {
          code: 'GEO-ANGLE',
          severity: 'error',
          element: 'angle',
          message: 'угол вне нормы',
          guide: 'Угол наклона 26,7° вне нормы (30–45°). Измените число ступеней.',
          param: 'Число ступеней',
          variations: [v],
        },
      ],
    },
  }
}

// --- Работа с аккордеоном -------------------------------------------------
//
// Панель — аккордеон с ОДНОЙ открытой секцией, и закрытая секция не рендерит
// своё тело вообще. Поэтому getByLabelText по полю из закрытой секции падает.
// Хелперы ниже открывают секции по очереди и находят нужное поле.
//
// Раньше форма была плоской, и кода было меньше, но тесты ловили ошибки
// редизайна только потому, что случайно совпадали с новой разметкой.

// SECTIONS — заголовки в порядке объявления в calcSections.
const SECTIONS = [
  'Основные настройки',
  'Настройки поворота',
  'Ограждение',
  'Цвет и материал',
  'Помещение',
]

/**
 * Открыть секцию, если она закрыта. Открытая остаётся открытой.
 *
 * Разметка: заголовки лежат в сетке `.accordion__heads`, а тело ОДНО —
 * `.acc__body` с aria-label открытой секции. Раньше секция была отдельной
 * карточкой `.acc` со своим телом, поэтому проверять приходилось вложенность.
 */
function openSection(title: string) {
  const head = screen.queryByText(title)
  if (!head) return
  if (head.closest('.acc__head')?.getAttribute('aria-expanded') === 'true') return
  fireEvent.click(head)
}

/**
 * Выполнить проверку ВНУТРИ конкретной секции.
 *
 * Нужна вместо inSections, когда известно, где искать: одновременно открыта
 * только одна секция, поэтому перебор «открыть следующую» закрывает
 * предыдущую, и следующая же проверка не находит элемент.
 */
function inSection<T>(title: string, fn: () => T): T {
  openSection(title)
  return fn()
}

/** Найти поле по подписи, при необходимости проходя по секциям. */
function field(label: string): HTMLElement {
  for (const title of SECTIONS) {
    openSection(title)
    const el = screen.queryByLabelText(label)
    if (el) return el as HTMLElement
  }
  throw new Error(`поле «${label}» не найдено ни в одной секции аккордеона`)
}

/**
 * Отрицательный вариант inSections: возвращает null, а не бросает.
 * Нужен для проверок «такого текста НЕТ ни в одной секции» — там отсутствие
 * и есть ожидаемый результат, а inSections на отсутствии падает.
 */
function maybeInSections<T>(find: () => T | null): T | null {
  for (const title of SECTIONS) {
    openSection(title)
    const found = find()
    if (found) return found
  }
  return null
}

/** То же, но для текста внутри секции (подсказки, кнопки). */
function inSections<T>(find: () => T | null): T {
  for (const title of SECTIONS) {
    openSection(title)
    const found = find()
    if (found) return found
  }
  throw new Error('элемент не найден ни в одной секции аккордеона')
}

// pickTreadWood — ступени из дерева. В металлокаркасе доступна только
// сталь, поэтому вид ступеней выбирается тумблером, а не произвольным
// кодом материала; общие пределы по материалам покрыты
// в shared/src/config.test.ts.
/** Переключить тип марша кнопкой сегмента (flight — код из config). */
function pickFlight(label: string) {
  fireEvent.click(
    inSections(() => screen.queryByRole('radio', { name: label })),
  )
}

function pickTreadWood() {
  inSections(() => screen.queryByRole('radio', { name: 'Дерево' }))
  fireEvent.click(inSections(() => screen.queryByRole('radio', { name: 'Дерево' })))
}

function pickTreadMetal() {
  fireEvent.click(inSections(() => screen.queryByRole('radio', { name: 'Металл' })))
}

const okQuote: QuoteResult = {
  validation: { valid: true, blocking: false, issues: [] },
  flight: {
    step_count: 15,
    step_height_mm: 180,
    tread_depth_mm: 270,
    run_mm: 4050,
    stringer_mm: 4867.49,
    angle_deg: 33.69,
    width_mm: 900,
    step_thickness_mm: 40,
    railing_height_mm: 900,
    riser: true,
    stringer_thickness_mm: 50,
  },
  geometry: { solid_count: 1, volume_mm3: 1e7, surface_area_mm2: 2e5, bbox: { min: { x: 0, y: 0, z: 0 }, max: { x: 900, y: 2700, z: 500 } } },
  pricing: { currency: 'RUB', material_rub: 1, machine_rub: 2, labor_rub: 3, overhead_rub: 4, production_cost_rub: 10, margin_rub: 5, discount_rub: 0, pre_tax_rub: 15, tax_rub: 3, final_price_rub: 18, lines: [] },
}

const blockedWithAdvice: QuoteResult = {
  validation: {
    valid: false,
    blocking: true,
    issues: [
      {
        code: 'GEO-ANGLE',
        severity: 'error',
        element: 'angle',
        message: 'угол вне нормы',
        guide: 'Угол наклона 29,7° вне нормы (30–45°). Измените число ступеней.',
        param: 'Число ступеней',
        suggestions: [{ step_count: 18, step_height_mm: 166.7, tread_depth_mm: 266.7, angle_deg: 32 }],
      },
    ],
  },
}

const validValues: Record<string, string> = {
  'Ширина марша (мм)': '900',
  'Высота (мм)': '2700',
  'Толщина ступени (мм)': '40',
  'Просвет (мм)': '2000',
  'Высота перил (мм)': '900',
  'Ширина помещения (мм)': '3000',
  'Длина помещения (мм)': '4200',
}

function fillValid() {
  for (const [label, value] of Object.entries(validValues)) {
    fireEvent.change(field(label), { target: { value } })
  }
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('Constructor', () => {
  it('показывает форму конструктора', async () => {
    await renderWithAuth(<Constructor />, null)
    expect(screen.getByText('Конструктор лестницы')).toBeInTheDocument()
    expect(field('Тип лестницы')).toBeInTheDocument()
    expect(screen.getByText('Прямой марш')).toBeInTheDocument()
    // Материал каркаса в калькуляторе только сталь, поэтому показан строкой,
    // а не кнопкой выбора (одна кнопка — ложный выбор).
    expect(inSections(() => screen.queryByText('Сталь S235'))).toBeInTheDocument()
  })

  // Раскладка аккордеона. Пять секций в пять рядов оставляли телу ~280px при
  // ширине 356px: данные приходилось вводить в маленькое окошко. Теперь
  // заголовки — компактная сетка, а тело ОДНО под ними на всю высоту панели.
  // Тест ловит возврат к отдельным карточкам и к «прыгающему» телу.
  it('все заголовки секций видны сразу, тело открытой — одно', async () => {
    await renderWithAuth(<Constructor />, null)
    // «Настройки поворота» у прямого марша скрыта, поэтому сверяемся с тем,
    // что реально отрисовано, а не со списком SECTIONS.
    const heads = Array.from(
      document.querySelectorAll('.accordion__heads .acc__head'),
    ) as HTMLElement[]
    expect(heads.length).toBeGreaterThanOrEqual(4)
    for (const head of heads) {
      expect(head).toBeVisible()
      expect(['true', 'false']).toContain(head.getAttribute('aria-expanded'))
    }
    expect(heads.map((h) => h.textContent?.trim()).join(' ')).toContain('Основные настройки')
    expect(heads.map((h) => h.textContent?.trim()).join(' ')).toContain('Помещение')

    // Тело ровно одно — под открытой секцией.
    expect(document.querySelectorAll('.acc__body')).toHaveLength(1)
  })

  it('скрытая для прямого марша секция появляется в сетке при повороте', async () => {
    await renderWithAuth(<Constructor />, null)
    expect(
      screen.queryByRole('button', { name: /Настройки поворота/ }),
    ).not.toBeInTheDocument()
    pickFlight('L-образная (с площадкой)')
    // Заголовок встаёт в общую сетку, а не отдельной карточкой.
    const added = screen.getByRole('button', { name: /Настройки поворота/ })
    expect(added.closest('.accordion__heads')).not.toBeNull()
    expect(document.querySelectorAll('.acc__body')).toHaveLength(1)
  })

  it('переключение секции не оставляет второго тела и не теряет заголовки', async () => {
    await renderWithAuth(<Constructor />, null)
    fireEvent.click(screen.getByRole('button', { name: /Помещение/ }))
    expect(document.querySelectorAll('.acc__body')).toHaveLength(1)
    // Раскрытая секция подписана своей меткой — иначе при прокрутке не
    // понять, что за раздел открыт.
    expect(screen.getByRole('region', { name: 'Помещение' })).toBeInTheDocument()
    for (const head of Array.from(
      document.querySelectorAll('.accordion__heads .acc__head'),
    ) as HTMLElement[]) {
      expect(head).toBeVisible()
    }
  })

  it('загружается с пустыми полями', async () => {
    await renderWithAuth(<Constructor />, null)
    // Толщина ступени — единственное числовое поле с дефолтом: дефолт
    // изделия «сталь + дуб» обязан быть валидным (20–60 мм), иначе
    // калькулятор открывается с красным полем.
    for (const label of Object.keys(validValues)) {
      const expected = label === 'Толщина ступени (мм)' ? '40' : ''
      expect(field(label)).toHaveValue(expected)
    }
  })

  // Поля панели принимают только целые миллиметры. Буквы и знаки отбрасываются
  // на входе: иначе «1o00» или «-900» уходили в запрос и отваливались уже на
  // сервере с 422, вместо того чтобы поле просто не дало их напечатать.
  it('в числовые поля нельзя ввести буквы и знаки', async () => {
    await renderWithAuth(<Constructor />, null)
    const width = field('Ширина марша (мм)')

    for (const [typed, expected] of [
      ['1o00', '100'],
      ['абв', ''],
      ['-900', '900'],
      ['1 200', '1200'],
      ['12.5', '125'],
      ['+800', '800'],
    ] as const) {
      fireEvent.change(width, { target: { value: typed } })
      expect((width as HTMLInputElement).value).toBe(expected)
    }

    // Пустая строка — валидное промежуточное состояние: из неё печатается
    // новое число, поэтому фильтр не должен её запрещать.
    fireEvent.change(width, { target: { value: '' } })
    expect((width as HTMLInputElement).value).toBe('')

    // Мобильная клавиатура получает numeric, а не decimal: значения целые.
    expect(width).toHaveAttribute('inputmode', 'numeric')
  })

  it('чекбокс подступенка: включён по умолчанию и передаётся в запрос', async () => {
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    const checkbox = field('Подступень') as HTMLInputElement
    expect(checkbox.checked).toBe(true)

    fireEvent.click(checkbox)
    expect((field('Подступень') as HTMLInputElement).checked).toBe(false)

    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => {
      expect(spy).toHaveBeenCalledWith(expect.objectContaining({ riser: false }))
    })
  })

  // После УСПЕШНОГО расчёта вкладки параметров схлопываются, и под ними
  // раскрывается результат. Владелец: «раскрытая вкладка должна схлопнутся
  // с параметрами, и под ними должна отображатся вся эта информация».
  // Отдельная колонка для результата была попыткой выиграть место по ширине —
  // вернули результат под параметры.
  it('после расчёта вкладки схлопываются, результат — под ними в рельсе', async () => {
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    // До расчёта раскрыта «Основные настройки».
    expect(document.querySelector('.acc__body')).not.toBeNull()

    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(spy).toHaveBeenCalled())

    await waitFor(() => {
      expect(document.querySelector('.acc__body')).toBeNull()
    })
    // Заголовки вкладок остаются на месте — схлопнулось тело, а не панель.
    expect(document.querySelectorAll('.acc__head').length).toBeGreaterThanOrEqual(4)
    // Результат лежит В рельсе конструктора, ниже его панели, и цена в нём
    // первая карточкой: человек, нажавший «Рассчитать», видит сумму сразу.
    const rail = document.querySelector('.calc__rail') as HTMLElement
    const price = rail.querySelector('.price-value')
    expect(price).not.toBeNull()
    const text = rail.textContent ?? ''
    expect(text.indexOf('Предварительная цена')).toBeLessThan(text.indexOf('Геометрия марша'))
    // Порядок в DOM: панель конструктора, потом результат.
    const children = Array.from(rail.children)
    const panelAt = children.findIndex((n) => n.classList.contains('panel'))
    const resultAt = children.findIndex((n) => n.classList.contains('panel--result'))
    expect(panelAt).toBeGreaterThanOrEqual(0)
    expect(resultAt).toBeGreaterThan(panelAt)
  })

  // При БЛОКИРУЮЩЕМ ответе вкладки остаются раскрытыми: там наоборот надо
  // вернуться к полям и поправить их.
  it('при блокирующем ответе вкладки не схлопываются', async () => {
    const blocked = { ...okQuote, validation: { valid: false, blocking: true, issues: [] } }
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(blocked)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect(document.querySelector('.acc__body')).not.toBeNull()
  })

  // Пределы под ползунком НЕ дублируются текстом: минимум и максимум показывает
  // шкала .slider__scale. Дубли («Мин 20 / макс 60 мм», «Макс 6000 мм») съедали
  // по строке на каждое поле, а у полей с материал-зависимыми пределами ещё и
  // вводили в заблуждение. Под ползунком остались только рекомендации.
  it('под ползунком нет дублей пределов, шкала показывает min/max', async () => {
    await renderWithAuth(<Constructor />, null)
    const noRangeDuplicates = [
      /^Мин \d+/,
      /^Макс \d+/,
      /\/ макс /,
    ]
    for (const re of noRangeDuplicates) {
      expect(maybeInSections(() => screen.queryByText(re))).toBeNull()
    }
    // Шкала ползунка толщины ступени: по умолчанию дуб, 20–60 мм. Поле
    // перенесено в «Цвет и материал» под переключатель материала ступеней,
    // потому что пределы задаёт именно он.
    const scale = inSection('Цвет и материал', () =>
      screen.getByLabelText('Толщина ступени (мм), ползунок').parentElement
        ?.querySelector('.slider__scale'),
    )
    expect(scale?.textContent).toBe('2060')
  })

  // Компактность панели по требованию владельца: убраны подписи, которые
  // дублировали то, что видно на ползунке, и повторяли единицу измерения.
  it('убраны лишние подписи и повторы единиц', async () => {
    await renderWithAuth(<Constructor />, null)

    // Степпер «Количество ступеней» — число выводится геометрией и меняется
    // перетаскиванием ступени в 3D.
    expect(screen.queryByText('Количество ступеней')).not.toBeInTheDocument()

    // «Влияет только на вид модели. На цену не влияет.» под палитрами.
    expect(maybeInSections(() => screen.queryByText(/На цену не влияет/))).toBeNull()

    // Подпись под подступенком убрана целиком: переключатель «Да/Нет»
    // говорит о поле всё, что нужно, а строка под ним была пустой высотой.
    expect(maybeInSections(() => screen.queryByText(/Высота равна высоте ступени/))).toBeNull()

    // Пояснения комнатных полей убраны (они были по два предложения каждый).
    expect(maybeInSections(() => screen.queryByText(/без проверки вписываемости/))).toBeNull()
    expect(maybeInSections(() => screen.queryByText(/направление марша/))).toBeNull()
  })

  it('под ползунком остались рекомендации, а не пределы', async () => {
    await renderWithAuth(<Constructor />, null)
    // Рекомендация — это не предел, её смысл в другом, и она осталась.
    expect(inSection('Ограждение', () => screen.queryByText('Рекомендуем 900–1100 мм'))).toBeInTheDocument()
    expect(inSection('Помещение', () => screen.queryByText('Рекомендуем ≥ 2000 мм'))).toBeInTheDocument()
  })

  // Требование владельца: толщина ступени стоит в «Цвете и материале», сразу
  // под выбором «Дерево/Металл». Пределы толщины задаёт именно материал
  // ступеней (дуб 20–60, сталь 3–8), и в «Основных настройках» человек
  // ставил 40 мм, переключал на металл и получал отказ сервера.
  it('толщина ступени живёт в разделе материалов, а не в основных', async () => {
    await renderWithAuth(<Constructor />, null)
    expect(inSection('Цвет и материал', () =>
      screen.queryByLabelText('Толщина ступени (мм)'),
    )).toBeInTheDocument()
    expect(inSection('Основные настройки', () =>
      screen.queryByLabelText('Толщина ступени (мм)'),
    )).not.toBeInTheDocument()
  })

  it('пределы ползунка толщины ступени следуют за материалом ступеней', async () => {
    await renderWithAuth(<Constructor />, null)
    const scale = () =>
      inSection('Цвет и материал', () =>
        screen.getByLabelText('Толщина ступени (мм), ползунок').parentElement
          ?.querySelector('.slider__scale')?.textContent,
      )
    // По умолчанию дуб: 20–60.
    expect(scale()).toBe('2060')
    // Металл: 3–8.
    pickTreadMetal()
    expect(scale()).toBe('38')
    // Обратно в дерево.
    pickTreadWood()
    expect(scale()).toBe('2060')
  })

  it('3D: перетаскивание ступени меняет высоту и сразу пересчитывает', async () => {
    const okQuote3D: QuoteResult = { ...okQuote, mesh: mesh3d }
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote3D)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(screen.getByTestId('mock-3d-viewer')).toBeInTheDocument())

    act(() => {
      ;(viewerProps.current.onDragHeight as (h: number) => void)(3100)
    })
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ height_mm: 3100 })),
    )
    expect(field('Высота (мм)')).toHaveValue('3100')
  })

  it('3D: горизонтальный drag меняет шаг комфорта и пересчитывает', async () => {
    const okQuote3D: QuoteResult = { ...okQuote, mesh: mesh3d }
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote3D)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(screen.getByTestId('mock-3d-viewer')).toBeInTheDocument())

    act(() => {
      ;(viewerProps.current.onDragComfortStep as (v: number) => void)(640)
    })
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ comfort_step_mm: 640 })),
    )
  })

  it('3D: у прямого марша кнопки «Развернуть» нет (у API нет направления)', async () => {
    const okQuote3D: QuoteResult = { ...okQuote, mesh: mesh3d }
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote3D)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(screen.getByTestId('mock-3d-viewer')).toBeInTheDocument())
    act(() => {
      ;(viewerProps.current.onSelectPart as (p: unknown) => void)({ solid: 2, role: 'tread' })
    })
    expect(screen.getByText('Ступень 3')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Развернуть' })).not.toBeInTheDocument()
  })

  it('смена материала на дерево подтягивает толщину ступени в допуск (иначе 422)', async () => {
    await renderWithAuth(<Constructor />, null)
    const thickness = field('Толщина ступени (мм)')
    fireEvent.change(thickness, { target: { value: '6' } })
    expect(field('Толщина ступени (мм)')).toHaveValue('6')
    // Переход на дерево подтягивает толщину к минимуму дуба (20 мм).
    pickTreadWood()
    expect(field('Толщина ступени (мм)')).toHaveValue('20')
    pickTreadMetal()
    expect(Number(field('Толщина ступени (мм)').getAttribute('value') ?? 0)).toBeLessThanOrEqual(8)
  })

  it('валидация учитывает пределы материала ступеней', async () => {
    await renderWithAuth(<Constructor />, null)
    // Металл: ступень 3–8 мм, 2 мм — вне предела.
    pickTreadMetal()
    fireEvent.change(field('Толщина ступени (мм)'), { target: { value: '2' } })
    expect(inSections(() => screen.queryByText('Не менее 3'))).toBeInTheDocument()
    // Дерево: минимум 20 мм, 10 мм — вне предела.
    pickTreadWood()
    fireEvent.change(field('Толщина ступени (мм)'), { target: { value: '10' } })
    expect(inSections(() => screen.queryByText('Не менее 20'))).toBeInTheDocument()
  })

  it('показывает знак справки с тултипом у просвета', async () => {
    await renderWithAuth(<Constructor />, null)
    // Подсказка «?» едет вместе с полем в секции «Помещение», поэтому её
    // надо найти, открыв секцию.
    const help = inSections(() =>
      screen.queryAllByRole('tooltip').find((el) =>
        el.textContent?.includes('Просвет — вертикальное расстояние'),
      ),
    )
    expect(help).toBeDefined()
  })

  it('прямой марш: один выбор перил с тултипом-подсказкой', async () => {
    await renderWithAuth(<Constructor />, null)
    const railing = inSection('Ограждение', () => screen.getByRole('radio', { name: 'С двух сторон' }))
    expect(railing).toBeVisible()
    expect(railing).toHaveAttribute('aria-checked', 'true')
    // Направление поворота — только для L/П, у прямого марша его нет.
    expect(screen.queryByRole('radio', { name: 'Влево' })).toBeNull()
    expect(screen.queryByRole('radio', { name: /Против часовой/ })).toBeNull()
    // Варианты перил доступны все четыре.
    inSection('Ограждение', () => {
      for (const label of ['Без перил', 'Слева', 'Справа', 'С двух сторон']) {
        expect(screen.getByRole('radio', { name: label })).toBeInTheDocument()
      }
    })
  })

  it('L-образная: перила по сегментам и направление поворота', async () => {
    await renderWithAuth(<Constructor />, null)
    pickFlight('L-образная (с площадкой)')
    // Прямой «Перила» скрывается, появляются сегменты (кнопки-радио).
    expect(screen.queryByRole('radio', { name: 'Без перил' })).toBeNull()
    // Три группы сегментных перил, у каждой своя подпись.
    for (const legend of ['Перила: первый марш', 'Перила: площадка', 'Перила: второй марш']) {
      inSection('Ограждение', () => {
        expect(screen.getByRole('group', { name: legend })).toBeInTheDocument()
      })
    }
    // По умолчанию во всех трёх группах выбрано «С двух сторон».
    inSection('Ограждение', () => {
      const checked = screen
        .queryAllByRole('radio', { name: 'С двух сторон' })
        .filter((b) => b.getAttribute('aria-checked') === 'true')
      expect(checked).toHaveLength(3)
    })
    // Направление поворота появляется и по умолчанию «влево».
    const dirLeft = inSection('Настройки поворота', () => screen.getByRole('radio', { name: 'Влево' }))
    expect(dirLeft).toHaveAttribute('aria-checked', 'true')
    // Спиральных полей у L-марша нет.
    expect(screen.queryByRole('radio', { name: /Против часовой/ })).toBeNull()

    // Переключаем направление поворота кнопкой сегмента.
    const dirRight = inSection('Настройки поворота', () =>
      screen.getByRole('radio', { name: 'Вправо' }),
    )
    fireEvent.click(dirRight)
    expect(
      inSection('Настройки поворота', () => screen.getByRole('radio', { name: 'Вправо' })),
    ).toHaveAttribute('aria-checked', 'true')
  })

  it('спиральный марш скрыт из формы (S-152)', async () => {
    await renderWithAuth(<Constructor />, null)
    // Тип марша — сегментированные кнопки: спираль в них отсутствует.
    const flights = inSections(() =>
      Array.from(
        document.querySelectorAll('.segmented[data-cols="3"] .segmented__item'),
      ).map((b) => b.textContent?.trim()),
    )
    expect(flights?.length).toBe(3)
    expect(flights?.join(' ')).not.toMatch(/Спираль/)
    // Поля спирали (радиус, направление) в форме отсутствуют.
    expect(screen.queryByLabelText('Радиус (мм)')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Направление спирали')).not.toBeInTheDocument()
  })

  it('не вызывает расчёт при ошибках валидации', async () => {
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    const width = field('Ширина марша (мм)')
    fireEvent.change(width, { target: { value: '100' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText('Исправьте поля формы перед расчётом')).toBeInTheDocument()
    expect(spy).not.toHaveBeenCalled()
  })

  it('выполняет расчёт и показывает результат с ценой', async () => {
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
    expect(screen.getByText('Предварительная цена')).toBeInTheDocument()
    await waitFor(() =>
      expect(quoteApi.calculate).toHaveBeenCalledWith(
        expect.objectContaining({ width_mm: 900, height_mm: 2700, flight: 'straight', material: 'STEEL-S235' }),
      ),
    )
  })

  it('высота поля «Высота (мм)» доходит до вьювера и пересчёт меняет её', async () => {
    const okQuote3D: QuoteResult = {
      ...okQuote,
      flight: { ...okQuote.flight!, room_width_mm: 3000, room_length_mm: 4200 },
      room_mesh: mesh3d,
      mesh: mesh3d,
    }
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote3D)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/^Результат расчёта/)
    await waitFor(() => {
      expect(viewerProps.current).toMatchObject({
        flight: 'straight',
        heightMM: 2700,
        roomWidth: 3000,
        roomLength: 4200,
      })
    })

    // Правка высоты и повторный расчёт — вьювер получает новое значение.
    fireEvent.change(field('Высота (мм)'), { target: { value: '2750' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(viewerProps.current.heightMM).toBe(2750))
    expect(viewerProps.current.flight).toBe('straight')
  })

  it('выбранный материал попадает в запрос расчёта', async () => {
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    pickTreadWood()
    // Толщина ступени 6 мм допустима для стали, для дуба — нет (min 20).
    fireEvent.change(field('Толщина ступени (мм)'), { target: { value: '30' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
    // Каркас — сталь, ступени — дуб: запрос несёт оба кода раздельно.
    await waitFor(() =>
      expect(quoteApi.calculate).toHaveBeenCalledWith(
        expect.objectContaining({ material: 'STEEL-S235', tread_material: 'WOOD-OAK' }),
      ),
    )
    expect(await screen.findByText('Материал каркаса: Сталь S235')).toBeInTheDocument()
  })

  it('L-образный тип показывает поля площадки', async () => {
    await renderWithAuth(<Constructor />, null)
    pickFlight('L-образная (с площадкой)')
    expect(field('Ширина площадки (мм)')).toBeInTheDocument()
    expect(field('Нижних ступеней (шт)')).toBeInTheDocument()
  })

  it('прямой марш скрывает поля площадки, радиуса спирали в форме нет', async () => {
    await renderWithAuth(<Constructor />, null)
    // У прямого марша секции поворота нет вовсе, поэтому полей площадки
    // нет даже после её открытия.
    expect(screen.queryByText('Настройки поворота')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Ширина площадки (мм)')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Нижних ступеней (шт)')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Радиус (мм)')).not.toBeInTheDocument()
  })

  it('высота ступени и шаг комфорта скрыты: рассчитываются автоматически', async () => {
    await renderWithAuth(<Constructor />, null)
    expect(screen.queryByLabelText('Высота ступени (мм)')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Шаг комфорта (мм)')).not.toBeInTheDocument()

    pickFlight('L-образная (с площадкой)')
    expect(screen.queryByLabelText('Шаг комфорта (мм)')).not.toBeInTheDocument()

  })

  it('пустое обязательное поле краснеет, но текст «Укажите значение» не показывается', async () => {
    await renderWithAuth(<Constructor />, null)
    const width = field('Ширина марша (мм)')
    fireEvent.change(width, { target: { value: '900' } })
    fireEvent.change(width, { target: { value: '' } })
    expect(width).toHaveClass('field-invalid')
    expect(screen.queryByText('Укажите значение')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Ширина площадки (мм)')).not.toBeInTheDocument()
  })

  // Регрессия: touched был одним флагом на всю форму, поэтому сдвиг ЛЮБОГО
  // ползунка показывал ошибки под всеми пустыми полями — «Укажите значение»
  // вылезал под «Шириной марша», которую пользователь не трогал. Теперь
  // помечается конкретное поле.
  it('сдвиг чужого ползунка не трогает соседние пустые поля', async () => {
    await renderWithAuth(<Constructor />, null)
    const height = field('Высота (мм)')
    expect(height).not.toHaveClass('field-invalid')
    expect(field('Ширина марша (мм)')).not.toHaveClass('field-invalid')

    fireEvent.change(height, { target: { value: '2500' } })

    // «Ширина марша» пуста и не тронута — красной рамки и текста быть не должно.
    expect(field('Ширина марша (мм)')).not.toHaveClass('field-invalid')
    expect(screen.queryByText('Укажите значение')).not.toBeInTheDocument()

    // А вот очистка «Ширины марша» — уже её собственная ошибка: поле краснеет.
    const width = field('Ширина марша (мм)')
    fireEvent.change(width, { target: { value: '900' } })
    fireEvent.change(width, { target: { value: '' } })
    expect(width).toHaveClass('field-invalid')
    expect(screen.queryByText('Укажите значение')).not.toBeInTheDocument()
  })

  it('ошибки пропадают после заполнения обязательных полей', async () => {
    await renderWithAuth(<Constructor />, null)
    fillValid()
    expect(screen.queryByText('Укажите значение')).not.toBeInTheDocument()
  })

  it('без габаритов помещения спрашивает; «Продолжить без площади» продолжает', async () => {
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    // Очищаем габариты помещения, чтобы сработал запрос подтверждения.
    fireEvent.change(field('Ширина помещения (мм)'), { target: { value: '' } })
    fireEvent.change(field('Длина помещения (мм)'), { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText(/Корректный расчёт/)).toBeInTheDocument()
    expect(spy).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Продолжить без площади' }))
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()

    // Выбор запоминается на сессию: повторный расчёт не спрашивает снова.
    fireEvent.change(field('Высота (мм)'), { target: { value: '2800' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(2))
  })

  it('«Внести данные площади» подсвечивает и фокусирует комнатные поля', async () => {
    const spy = vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.change(field('Ширина помещения (мм)'), { target: { value: '' } })
    fireEvent.change(field('Длина помещения (мм)'), { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Корректный расчёт/)

    fireEvent.click(screen.getByRole('button', { name: 'Внести данные площади' }))
    expect(screen.queryByText(/Корректный расчёт/)).not.toBeInTheDocument()
    // Подсказки-инструкции убраны намеренно: пустые комнатные поля помечаются
    // только красной рамкой (field-invalid), без текста «Укажите ширину…».
    expect(field('Ширина помещения (мм)')).toHaveClass('field-invalid')
    expect(field('Длина помещения (мм)')).toHaveClass('field-invalid')
    expect(screen.queryByText(/Укажите (ширину|длину) помещения/)).not.toBeInTheDocument()
    expect(spy).not.toHaveBeenCalled()

    fireEvent.change(field('Ширина помещения (мм)'), { target: { value: '3000' } })
    fireEvent.change(field('Длина помещения (мм)'), { target: { value: '4200' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
  })

  it('кнопка «Применить» подставляет значения и снимает блокировку', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(blockedWithAdvice)
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)

    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText('Расчёт остановлен: обнаружены блокирующие нарушения.')

    fireEvent.click(screen.getByRole('button', { name: 'Применить эти значения' }))

    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ step_height_mm: 166.7 })),
    )
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
    // Высота ступени больше не показывается полем ввода — применяется скрытый таргет.
    expect(screen.queryByLabelText('Высота ступени (мм)')).not.toBeInTheDocument()
  })

  it('«Применить» подставляет предложение советника и снимает блокировку', async () => {
    const blocked: QuoteResult = {
      validation: {
        valid: false,
        blocking: true,
        issues: [
          {
            code: 'GEO-TREAD',
            severity: 'error',
            element: 'tread_depth',
            message: 'проступь вне нормы',
            guide: 'Проступь 240 мм вне диапазона 260–320 мм. Увеличьте шаг комфорта.',
            param: 'Шаг комфорта',
            suggestions: [
              { step_count: 16, step_height_mm: 168.75, tread_depth_mm: 300, angle_deg: 29.3 },
            ],
          },
        ],
      },
    }
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(blocked)
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText('Расчёт остановлен: обнаружены блокирующие нарушения.')

    fireEvent.click(screen.getByRole('button', { name: 'Применить эти значения' }))

    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ flight: 'straight' })),
    )
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
  })

  it('вариация (straight) подставляет высоту ступени и пересчитывает', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({ flight: 'straight', heightMM: '3000', widthMM: '1000', stepHeightMM: '166.67', comfortStepMM: '620' }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText('Расчёт остановлен: обнаружены блокирующие нарушения.')
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({ step_height_mm: 166.67, flight: 'straight' }),
      ),
    )
    // Шаг комфорта из варианта не применяется — дефолт 630 на бэкенде.
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastCall['comfort_step_mm']).toBeUndefined()
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
  })

  it('вариация (l_shape) меняет тип марша и подставляет площадку', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({
          flight: 'l_shape', heightMM: '3000', widthMM: '1000', landingWidthMM: '1000',
          landingDepthMM: '1500', lowerStepCountMM: '9', stepHeightMM: '166.67', comfortStepMM: '600',
        }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText('Расчёт остановлен: обнаружены блокирующие нарушения.')
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({
          flight: 'l_shape', step_height_mm: 166.67,
          landing_width_mm: 1000, lower_step_count: 9,
        }),
      ),
    )
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastCall['comfort_step_mm']).toBeUndefined()
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
  })

  it('вариация (u_shape) меняет тип марша и подставляет площадку', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({
          flight: 'u_shape', heightMM: '3000', widthMM: '1000', landingWidthMM: '1000',
          landingDepthMM: '1500', lowerStepCountMM: '9', stepHeightMM: '166.67', comfortStepMM: '620',
        }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText('Расчёт остановлен: обнаружены блокирующие нарушения.')
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({
          flight: 'u_shape', step_height_mm: 166.67,
          landing_width_mm: 1000, lower_step_count: 9,
        }),
      ),
    )
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastCall['comfort_step_mm']).toBeUndefined()
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
  })

  it('вариация не затирает выбор перил пустыми полями (clobbering)', async () => {
    // Бэкенд отдаёт вариацию со всеми полями ConfigForm, в т.ч. пустыми
    // (railing/direction не заданы для прямого марша). До фикса эти пустые
    // строки перезаписывали выбор пользователя → форма невалидна → пересчёт
    // падал. После фикса пустые значения пропускаются.
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({
          flight: 'straight', heightMM: '3000', widthMM: '1000', stepHeightMM: '166.67',
          comfortStepMM: '620', railing: '', direction: '',
        }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText('Расчёт остановлен: обнаружены блокирующие нарушения.')
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({
          step_height_mm: 166.67, flight: 'straight', railing: 'both',
        }),
      ),
    )
    // пустые поля вариации не должны попасть в запрос; шаг комфорта — дефолт
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1][0] as Record<string, unknown>
    expect(lastCall['railing']).toBe('both')
    expect(lastCall['direction']).toBeUndefined()
    expect(lastCall['comfort_step_mm']).toBeUndefined()
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
  })

  it('исходный марш остаётся в галерее и возвращается по клику', async () => {
    const spy = vi
      .spyOn(quoteApi, 'calculate')
      .mockResolvedValueOnce(
        blockedVariations({
          flight: 'l_shape', heightMM: '3000', widthMM: '1000',
          landingWidthMM: '1000', landingDepthMM: '1500', lowerStepCountMM: '9',
          stepHeightMM: '166.67',
        }),
      )
      .mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)
    fillValid()
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText('Расчёт остановлен: обнаружены блокирующие нарушения.')

    // Список вариантов: исходный «Прямой марш» + альтернатива от бэкенда.
    // Метки «Выбран» нет: снапшот ИСХОДНОГО (заблокированного) конфига не
    // считается применённым вариантом — он как раз тот, от чего предлагают
    // уйти, и раньше именно он помечался «Выбран».
    expect(screen.queryByText('Выбран')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Прямой марш/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Угол 30°/ })).toBeInTheDocument()

    // Выбираем L-образный вариант — он применяется, и всё остальное скрывается:
    // это подтверждение выбора (владелец: «как мне подтвердить этот выбор,
    // чтобы все остальное скрылось»).
    fireEvent.click(screen.getByRole('button', { name: /Угол 30°/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ flight: 'l_shape' })),
    )
    // Применённый вариант показан строкой (она некликабельна) + «Изменить».
    expect(screen.getByText('L-образный марш')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Прямой марш/ })).not.toBeInTheDocument()

    // «Изменить» возвращает полный список — история конфигураций на месте.
    fireEvent.click(screen.getByRole('button', { name: 'Изменить' }))
    expect(screen.getByRole('button', { name: /Прямой марш/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /L-образный марш/ })).toBeInTheDocument()

    // Возврат к исходному прямому маршу — полное восстановление конфига.
    fireEvent.click(screen.getByRole('button', { name: /Прямой марш/ }))
    await waitFor(() =>
      expect(spy).toHaveBeenLastCalledWith(
        expect.objectContaining({ width_mm: 900, height_mm: 2700, flight: 'straight' }),
      ),
    )
    expect(await screen.findByText(/^Результат расчёта/)).toBeInTheDocument()
  })
})
// ---- Живая валидация при вводе (S-P5) ----
// Мок ответа публичного :validate: спираль, радиус меньше ширины марша.
// Блокирующая живая валидация на прямом марше: проступь вне диапазона и
// готовое предложение советника (спираль отключена, S-152).
const liveBlockedStraight = {
  valid: false,
  blocking: true,
  issues: [
    {
      code: 'GEO-TREAD',
      severity: 'error',
      element: 'tread_depth',
      message: 'проступь вне диапазона 260–320 мм',
      param: 'Шаг комфорта',
      guide: 'Проступь 240 мм вне диапазона 260–320 мм.',
      suggestions: [
        { step_count: 16, step_height_mm: 168.75, tread_depth_mm: 300, angle_deg: 29.3 },
      ],
    },
  ],
}

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))
const LIVE_DEBOUNCE = 700

describe('Constructor · живая валидация (S-P5)', () => {
  it('вызывает :validate при изменении полей и показывает баннер блокировки', async () => {
    const validateSpy = vi.spyOn(quoteApi, 'validate').mockResolvedValue(liveBlockedStraight)
    vi.spyOn(quoteApi, 'calculate').mockResolvedValue(okQuote)
    await renderWithAuth(<Constructor />, null)

    fillValid()
    // Спираль отключена (S-152) — блокировку показываем на прямом марше
    // по проступи, применяя предложение советника.
    fireEvent.change(field('Ширина марша (мм)'), { target: { value: '1000' } })

    await waitFor(() => expect(validateSpy).toHaveBeenCalledTimes(1), { timeout: 2500 })
    const body = validateSpy.mock.calls[0][0] as Record<string, unknown>
    expect(body.flight).toBe('straight')
    expect(body.width_mm).toBe(1000)

    // Баннер блокировки с guide и подсветка поля (S-P5).
    await waitFor(() => {
      expect(screen.getAllByText(/Проступь 240 мм вне диапазона/).length).toBeGreaterThanOrEqual(1)
    }, { timeout: 2500 })
    expect(validateSpy).toHaveBeenCalledTimes(1)

    // Кнопка «Применить» из баннера: вариант советника подставляется в форму
    // и запускается полный расчёт с новым радиусом/шириной.
    fireEvent.click(screen.getByText(/Применить: 16 ступ/))
    await waitFor(() => expect(quoteApi.calculate).toHaveBeenCalledTimes(1), { timeout: 2500 })
    const calcBody = (quoteApi.calculate as ReturnType<typeof vi.fn>).mock.calls[0][0] as Record<string, unknown>
    expect(calcBody.flight).toBe('straight')
    expect(calcBody.step_height_mm).toBeCloseTo(168.75, 1)
  })

  it('не дёргает :validate, пока форма имеет локальные ошибки', async () => {
    const validateSpy = vi.spyOn(quoteApi, 'validate').mockResolvedValue(liveBlockedStraight)
    await renderWithAuth(<Constructor />, null)
    // Высота пустая — локальная ошибка «Укажите значение».
    fireEvent.change(field('Высота (мм)'), { target: { value: '9999' } })
    await sleep(LIVE_DEBOUNCE + 200)
    expect(validateSpy).not.toHaveBeenCalled()
  })
})
