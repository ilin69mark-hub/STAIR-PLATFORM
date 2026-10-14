import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Constructor } from '@shared/storefront/components/Constructor'

// Регрессия: 2026-09-28. Конструктор витрины падал в ErrorBoundary с
// «Что-то пошло не так» на КАЖДОМ открытии страницы.
//
// Причина — temporal dead zone. Блок с renderField/calcSections был
// объявлен выше const-ов visible/selectOptions/update, а JSX внутри
// content: (<>…</>) вычисляется в момент создания массива секций, то есть
// ДО их инициализации. Падало всё, без ввода данных и без расчёта.
//
// Тест ловит именно это: любой ReferenceError на рендере означает возврат
// бага. Проверяем исходное состояние, поведение аккордеона и секцию
// материалов — там строились палитры, которые тоже падали бы при TDZ.
//
// Третий тест заодно поймал отдельный баг Slider: <label htmlFor> указывал
// на id, которого не было ни у одного инпута, поэтому подпись не была
// связана с полем (getByLabelText её не находил, и клик по подписи не
// фокусировал ввод). Теперь id у числового поля, у ползунка — производный
// с aria-label.

describe('Constructor: рендер без падения в ErrorBoundary', () => {
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('исходное состояние: секции аккордеона на месте', () => {
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    let thrown: unknown = null
    try {
      render(<Constructor />)
    } catch (e) {
      thrown = e
    }
    if (thrown) console.log('THROWN:', (thrown as Error).stack)
    errSpy.mockRestore()
    expect(thrown).toBeNull()
    // Первая секция открыта, две кнопки изделий видны.
    expect(screen.getByText('Основные настройки')).toBeTruthy()
    expect(screen.getByRole('tab', { name: /Металлокаркас/ })).toBeTruthy()
    expect(screen.getByRole('tab', { name: /Деревянные/ })).toBeTruthy()
  })

  it('аккордеон: секции раскрываются, сворачиваются и могут быть свёрнуты все', () => {
    render(<Constructor />)
    const main = screen.getByText('Основные настройки')
    // «Форма лестницы» — содержимое открытой секции.
    expect(screen.getByText('Тип лестницы')).toBeTruthy()
    fireEvent.click(main)
    // Клик по открытой секции закрывает и её: состояние «всё свёрнуто»
    // должно быть достижимо (требование владельца продукта).
    expect(screen.queryByText('Тип лестницы')).toBeNull()
    // Открываем другую секцию — открытой остаётся ровно одна.
    fireEvent.click(screen.getByText('Цвет и материал'))
    expect(screen.getByText('Каркас (косоуры и площадка)')).toBeTruthy()
    expect(screen.queryByText('Тип лестницы')).toBeNull()

    // И её тоже можно закрыть — все заголовки остаются на месте.
    fireEvent.click(screen.getByText('Цвет и материал'))
    expect(screen.queryByText('Каркас (косоуры и площадка)')).toBeNull()
    // Прямой марш: «Настройки поворота» скрыта (только L/П), поэтому в
    // свёрнутом состоянии остаются четыре заголовка, а не пять.
    for (const t of ['Основные настройки', 'Ограждение', 'Цвет и материал', 'Помещение']) {
      expect(screen.getByText(t)).toBeTruthy()
    }
    expect(screen.queryByText('Настройки поворота')).toBeNull()
  })

  // Порядок секций в DOM НЕ меняется при раскрытии. Раньше открытая
  // секция поднималась наверх, а остальные уходили под неё — это спасало от
  // выталкивания заголовков за край панели, но само по себе было прыжком:
  // тело открытой секции ездило по высоте. Теперь заголовки — компактная
  // сетка (три ряда), а тело ОДНО под ними, поэтому прыгать нечему и
  // порядок разделов остаётся постоянным.
  // Вкладки: четыре ОДИНАКОВЫЕ плитки в одной сетке и фиксированный порядок.
  // Тест ловит возврат к «открытая на всю ширину и первой» (order: -1):
  // размер плиток должен быть одинаковым, а позиции — неизменными.
  it('заголовки — одинаковые вкладки в фиксированном порядке', () => {
    render(<Constructor />)
    const heads = Array.from(
      document.querySelectorAll('.accordion__heads .acc__head'),
    ) as HTMLElement[]
    expect(heads.map((h) => h.textContent?.replace(/[+−]$/, '').trim())).toEqual([
      'Основные настройки',
      'Ограждение',
      'Цвет и материал',
      'Помещение',
    ])

    // Ровно одна открыта, и помечена — иначе «где я сейчас» не читается.
    const open = heads.filter((h) => h.classList.contains('is-open'))
    expect(open).toHaveLength(1)
    expect(open[0].getAttribute('aria-expanded')).toBe('true')

    // Переключение не двигает и не переставляет вкладки.
    fireEvent.click(screen.getByText('Помещение'))
    const after = Array.from(
      document.querySelectorAll('.accordion__heads .acc__head'),
    ) as HTMLElement[]
    expect(after).toEqual(heads)
    expect(after.filter((h) => h.classList.contains('is-open'))).toHaveLength(1)
    expect(after[3]).toHaveClass('is-open')
  })

  it('порядок секций не меняется при раскрытии, тело всегда одно', () => {
    render(<Constructor />)
    const titles = () =>
      Array.from(document.querySelectorAll('.acc__title')).map((n) => n.textContent)
    const initial = [
      'Основные настройки',
      'Ограждение',
      'Цвет и материал',
      'Помещение',
    ]
    expect(titles()).toEqual(initial)

    fireEvent.click(screen.getByText('Цвет и материал'))
    expect(titles()).toEqual(initial)
    expect(document.querySelectorAll('.acc__body')).toHaveLength(1)

    // Открытая секция помечена и подписана — иначе при прокрутке тела
    // непонятно, какой раздел раскрыт.
    const open = screen
      .getByText('Цвет и материал')
      .closest('.acc__head') as HTMLElement
    expect(open).toHaveAttribute('aria-expanded', 'true')
    expect(document.querySelectorAll('.acc__head.is-open')).toHaveLength(1)
    expect(screen.getByRole('region', { name: 'Цвет и материал' })).toBeInTheDocument()

    // Клик по нижней секции: порядок тот же, тело по-прежнему одно.
    fireEvent.click(screen.getByText('Помещение'))
    expect(titles()).toEqual(initial)
    expect(document.querySelectorAll('.acc__body')).toHaveLength(1)

    // Свернуть всё: тела нет, порядок прежний.
    fireEvent.click(screen.getByText('Помещение'))
    expect(titles()).toEqual(initial)
    expect(document.querySelectorAll('.acc__body')).toHaveLength(0)
  })

  it('секция материалов: палитры каркаса и ступеней, переключение Дерево/Металл', () => {
    render(<Constructor />)
    fireEvent.click(screen.getByText('Цвет и материал'))

    // Каркас в металлокаркасе всегда сталь, поэтому палитра каркаса
    // показывается строкой, а у ступеней — переключатель вида материала.
    expect(screen.getByText('Каркас (косоуры и площадка)')).toBeTruthy()
    expect(screen.getByText('Ступени (проступи и площадка)')).toBeTruthy()
    // Дефолт витрины — дуб, поэтому видны и отделка ступеней, и палитра каркаса.
    expect(screen.getByText('Цвет каркаса')).toBeTruthy()
    expect(screen.getByText('Отделка ступеней')).toBeTruthy()

    // Переключаем на металл: подпись палитры ступеней меняется, рендер не падает.
    fireEvent.click(screen.getByRole('radio', { name: 'Металл' }))
    expect(screen.getByText('Цвет ступеней')).toBeTruthy()

    // И обратно на дерево.
    fireEvent.click(screen.getByRole('radio', { name: 'Дерево' }))
    expect(screen.getByText('Отделка ступеней')).toBeTruthy()
  })
})

// Сворачиваемая панель параметров. Панель уезжает вбок, отдавая всю ширину
// 3D-сцене, а ручка остаётся на кромке — вернуть панель можно всегда.
describe('Constructor: панель параметров убирается и выдвигается', () => {
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  // Имя кнопки — в aria-label: подпись «Панель» убрали с кромки, но кнопка
  // осталась доступной для скринридера и для поиска в тестах.
  const railToggle = () => screen.getByRole('button', { name: /панель параметров/i })

  it('ручка видна, панель раскрыта', () => {
    render(<Constructor />)
    const btn = railToggle()
    expect(btn).toBeTruthy()
    expect(btn.getAttribute('aria-expanded')).toBe('true')
    expect(btn.getAttribute('aria-controls')).toBe('calc-rail')
    expect(document.getElementById('calc-rail')).toBeTruthy()
    expect(document.querySelector('.calc--rail-hidden')).toBeNull()
    // На кромке только стрелка (решение владельца): подпись «Панель»
    // перекрывала изделие, имя кнопки живёт в aria-label.
    expect(btn.textContent).toBe('›')
    expect(screen.queryByText('Панель')).toBeNull()
  })

  it('свёрнутая ручка показывает стрелку обратно', () => {
    render(<Constructor />)
    fireEvent.click(railToggle())
    expect(railToggle().textContent).toBe('‹')
    fireEvent.click(railToggle())
    expect(railToggle().textContent).toBe('›')
  })

  it('клик убирает панель и раскрывает обратно, состояние полей не теряется', () => {
    render(<Constructor />)
    const width = document.querySelector('input[type=text]') as HTMLInputElement
    fireEvent.change(width, { target: { value: '1234' } })

    fireEvent.click(railToggle())
    expect(railToggle().getAttribute('aria-expanded')).toBe('false')
    expect(document.querySelector('.calc')).toHaveClass('calc--rail-hidden')
    // Панель не размонтирована — форма держит введённое, иначе сворачивание
    // стирало бы расчёт настроек.
    expect(document.getElementById('calc-rail')).toBeTruthy()
    expect((document.querySelector('input[type=text]') as HTMLInputElement).value).toBe('1234')

    fireEvent.click(railToggle())
    expect(railToggle().getAttribute('aria-expanded')).toBe('true')
    expect(document.querySelector('.calc--rail-hidden')).toBeNull()
    expect((document.querySelector('input[type=text]') as HTMLInputElement).value).toBe('1234')
  })

  it('свёрнутая панель уходит из фокуса (inert), иначе Tab лезет в невидимые поля', () => {
    render(<Constructor />)
    const rail = document.getElementById('calc-rail') as HTMLElement
    expect(rail.hasAttribute('inert')).toBe(false)
    fireEvent.click(railToggle())
    expect(rail.hasAttribute('inert')).toBe(true)
    fireEvent.click(railToggle())
    expect(rail.hasAttribute('inert')).toBe(false)
  })
})
