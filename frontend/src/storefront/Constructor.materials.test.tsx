import { describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Constructor } from '@shared/storefront/components/Constructor'

// Ассортимент калькулятора витрины: в металлокаркасе доступна только сталь,
// у деревянной лестницы породы выбираются кнопками. Сталь единственна, поэтому
// показывается строкой, а не кнопкой: один пункт в сегменте выглядит как
// выбор, которого нет.

const frameButtons = () =>
  screen.queryAllByRole('radio').map((b) => b.textContent?.trim()).filter(Boolean)

describe('ассортимент калькулятора', () => {
  it('в металлокаркасе доступна только сталь', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<Constructor />)
    fireEvent.click(screen.getByText('Цвет и материал'))

    expect(screen.getByText('Сталь S235')).toBeTruthy()
  })

  it('кнопки материала каркаса не появляются ни в одной роли', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<Constructor />)
    fireEvent.click(screen.getByText('Цвет и материал'))

    expect(frameButtons()).not.toContain('Сталь S235')
  })

  it('у деревянных лестниц породы остаются, это другая ось выбора', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<Constructor />)
    fireEvent.click(screen.getByRole('tab', { name: /Деревянные/ }))
    fireEvent.click(screen.getByText('Цвет и материал'))

    // «Дуб» встречается трижды: материал лестницы, порода ступеней и
    // заблокированный материал подступенков (он следует за ступенями).
    // Первые две — независимые оси выбора, третья — производное значение,
    // показать его надо, выбрать нельзя.
    expect(screen.getAllByText('Дуб').length).toBe(3)
    // Портоды ступеней — независимый выбор, он остаётся кнопками.
    expect(frameButtons()).toContain('Орех')
    expect(frameButtons()).toContain('Ясень')
    expect(frameButtons()).toContain('Сосна')
  })

  // Материал подступенков ПРОИЗВОДНЫЙ от материала ступеней (решение
  // владельца): переключатель показан, но заблокирован. Проверяем обе
  // стороны — металл и дерево — и то, что подступенок идёт по материалу
  // ступеней, а не каркаса.
  describe('материал подступенков следует за ступенями', () => {
    // Доступное имя группы — просто «Подступенки»: замок помечен aria-hidden
    // (он декоративный), а смысл блокировки несут aria-disabled на кнопках и
    // подсказка «Следует за материалом ступеней» — их скринридер читает.
    const riserGroup = () => screen.getByRole('group', { name: 'Подступенки' })

    it('металлические ступени: подступенок — металл, заблокирован', () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      render(<Constructor />)
      fireEvent.click(screen.getByText('Цвет и материал'))
      // Стартовая комплектация — стальной каркас с ДЕРЕВЯННЫМИ ступенями,
      // поэтому металлические ступени надо выбрать явно.
      fireEvent.click(screen.getByRole('radio', { name: 'Металл' }))

      const group = riserGroup()
      expect(group).toBeTruthy()
      // Обе кнопки — показатели, а не переключатели: role=radio у них нет
      // (иначе получилось бы две группы радиокнопок с именем «Металл»).
      expect(group.querySelectorAll('[role="radio"]')).toHaveLength(0)
      const items = Array.from(group.querySelectorAll('button'))
      expect(items.map((b) => b.getAttribute('aria-disabled'))).toEqual(['true', 'true'])
      // Активен «Металл» — он же материал ступеней по умолчанию.
      expect(items.find((b) => b.className.includes('is-active'))?.textContent?.trim()).toBe('Металл')
    })

    it('деревянные ступени: подступенок — дерево, отделка выбирается своя', () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      render(<Constructor />)
      fireEvent.click(screen.getByText('Цвет и материал'))
      fireEvent.click(screen.getByRole('radio', { name: 'Дерево' }))

      const items = Array.from(riserGroup().querySelectorAll('button'))
      // Активен именно дуб — порода ступеней, а не обобщённое «Дерево».
      expect(items.find((b) => b.className.includes('is-active'))?.textContent?.trim()).toBe('Дуб')

      // Отделка подступенков — отдельный ряд свотчей, и она своя.
      const riserSwatches = screen.getByRole('group', { name: 'Отделка подступенков' })
      const riserSwatch = riserSwatches.querySelector('button[aria-checked="true"]')
      const treadSwatch = screen
        .getByRole('group', { name: 'Отделка ступеней' })
        .querySelector('button[aria-checked="true"]')
      expect(riserSwatch).toBeTruthy()
      expect(riserSwatch?.textContent?.trim()).toBe(treadSwatch?.textContent?.trim())

      // Выбор другого цвета подступенков не трогает ступени.
      fireEvent.click(riserSwatches.querySelectorAll('button')[2])
      expect(
        screen.getByRole('group', { name: 'Отделка ступеней' }).querySelector('button[aria-checked="true"]'),
      ).toBe(treadSwatch)
    })
  })
})

// Толщина ступени из металла не выбирается: всегда 5 мм (решение владельца
// 2026-10-05). Поле у металла скрыто, потому что диапазон 3–8 мм предлагал
// выбор, которого в изделии нет.
//
// Проверяется через сегмент «Дерево / Металл» в самой форме: у дефолтной
// дубовой ступени поле видно, после переключения на металл — нет.
describe('толщина ступени из металла', () => {
  const labelVisible = () => screen.queryByText('Толщина ступени (мм)') !== null
  // Сегмент материала ступеней «Дерево / Металл». Слово «Металл» есть и в
  // легенде сегмента, поэтому берём именно ЯРЛЫК кнопки.
  const treadMetalOption = () => treadOption('Металл')
  const treadWoodOption = () => treadOption('Дерево')
  const treadOption = (label: string): HTMLElement =>
    screen.queryAllByText(label).find((el) => el.className.includes('segmented__label'))!

  it('у дубовой ступени поле есть, у стальной — скрыто', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<Constructor />)
    fireEvent.click(screen.getByText('Цвет и материал'))

    // Дефолт витрины — стальной каркас с ДЕРЕВЯНОЙ ступенью: у дуба толщина
    // выбирается, поэтому поле на месте.
    expect(labelVisible()).toBe(true)

    // Переключаем ступень на металл.
    fireEvent.click(treadMetalOption())

    expect(labelVisible()).toBe(false)
  })

  it('возврат к дереву возвращает поле', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<Constructor />)
    fireEvent.click(screen.getByText('Цвет и материал'))

    fireEvent.click(treadMetalOption())
    expect(labelVisible()).toBe(false)

    fireEvent.click(treadWoodOption())
    expect(labelVisible()).toBe(true)
  })
})

// Толщина ЛИСТА косоура у металла — не входной параметр: её выводит норматив
// по длине и ширине марша (решение владельца 2026-10-06). Поле у листового
// каркаса не показывается, потому что выбирать нечего: 8 / 10 / 12 мм решает
// длина марша.
describe('толщина листа косоура', () => {
  const labelVisible = () => screen.queryByText('Толщина косоура (мм)') !== null
  const woodTab = () => screen.getByRole('tab', { name: /Деревянные/ })
  const metalTab = () => screen.getByRole('tab', { name: /Металлокаркас/ })

  it('у стального каркаса поля нет ни в одном изделии', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<Constructor />)
    fireEvent.click(screen.getByText('Цвет и материал'))
    expect(labelVisible()).toBe(false)

    // Переключение изделия туда-обратно не должно поле вернуть.
    fireEvent.click(woodTab())
    fireEvent.click(metalTab())
    expect(labelVisible()).toBe(false)
  })

  it('у деревянного каркаса поле скрыто тоже: толщина балки выводится по маршу', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<Constructor />)
    fireEvent.click(screen.getByText('Цвет и материал'))
    fireEvent.click(woodTab())

    expect(labelVisible()).toBe(false)
  })
})
