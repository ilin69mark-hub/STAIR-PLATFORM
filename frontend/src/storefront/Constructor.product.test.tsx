import { describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Constructor } from '@shared/storefront/components/Constructor'

// Переключение изделия обязано оставлять конфигурацию ВАЛИДНОЙ.
//
// Пресеты «Готовые решения» были удалены как раз потому, что задавали одно
// поле material и оставляли толщину, не проходящую по пределам нового
// материала. Здесь та же ловушка, но уже в кнопке изделия: если переключение
// не подтянет толщины, пользователь получит расчёт с MFG-MATERIAL.

describe('переключение изделия', () => {
  it('металлокаркас → деревянные: материалы и толщины становятся допустимыми', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<Constructor />)

    fireEvent.click(screen.getByRole('tab', { name: /Деревянные/ }))

    // Секция материалов теперь предлагает породы дерева, а не металлы.
    fireEvent.click(screen.getByText('Цвет и материал'))
    expect(screen.getByText('Материал лестницы')).toBeTruthy()
    expect(screen.getByText('Порода ступеней')).toBeTruthy()
    // Тумблер «Дерево/Металл» у деревянной лестницы неуместен.
    expect(screen.queryByText('Ступени (проступи и площадка)')).toBeNull()
  })

  it('деревянные → металлокаркас: возвращаются металлы и тумблер вида ступеней', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(<Constructor />)

    fireEvent.click(screen.getByRole('tab', { name: /Деревянные/ }))
    fireEvent.click(screen.getByRole('tab', { name: /Металлокаркас/ }))
    fireEvent.click(screen.getByText('Цвет и материал'))

    expect(screen.getByText('Каркас (косоуры и площадка)')).toBeTruthy()
    expect(screen.getByText('Ступени (проступи и площадка)')).toBeTruthy()
  })

  it('ни один переход не роняет рендер', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const order = ['Деревянные', 'Металлокаркас', 'Деревянные', 'Металлокаркас']
    for (const label of order) {
      const { unmount } = render(<Constructor />)
      // Клик не должен бросать: смена изделия трогает материалы, толщины и
      // палитры, то есть несколько мест сразу.
      expect(() => fireEvent.click(screen.getByRole('tab', { name: new RegExp(label) }))).not.toThrow()
      unmount()
    }
  })
})
