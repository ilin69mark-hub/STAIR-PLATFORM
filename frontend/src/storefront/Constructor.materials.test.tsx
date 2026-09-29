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
    const riserGroup = () => screen.getByRole('group', { name: 'Подступенки (материал)' })

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
