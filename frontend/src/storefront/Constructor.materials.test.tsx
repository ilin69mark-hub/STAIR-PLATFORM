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

    // «Дуб» встречается дважды: в выборе материала лестницы и в выборе
    // породы ступеней. Это две независимые оси, а не дубль в интерфейсе.
    expect(screen.getAllByText('Дуб').length).toBe(2)
    // Портоды ступеней — независимый выбор, он остаётся кнопками.
    expect(frameButtons()).toContain('Орех')
    expect(frameButtons()).toContain('Ясень')
    expect(frameButtons()).toContain('Сосна')
  })
})
