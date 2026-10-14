import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { Accordion } from './Accordion'

// Указатель «снизу есть ещё»: у панели нет своего скролла, скроллится тело
// открытой секции, и длинный раздел обрезан молча. Проверяем само правило
// (виден ⇔ низ не доскроллен) и то, что стрелка лежит СНАРУЖИ прокручиваемого
// тела: position: absolute внутри скроллера уехал бы с содержимым.

const sections = [
  { id: 'main', title: 'Основные настройки', content: <div>основные</div> },
  { id: 'color', title: 'Цвет и материал', content: <div>цвета</div> },
]

/** Подменяем геометрию тела: в jsdom нет раскладки, поэтому scrollHeight и
 *  clientHeight всегда 0 и указатель не появился бы никогда. */
function fakeOverflow({ scrollHeight, clientHeight }: { scrollHeight: number; clientHeight: number }) {
  const def = (prop: string, value: number) =>
    Object.defineProperty(HTMLElement.prototype, prop, { configurable: true, get: () => value })
  def('scrollHeight', scrollHeight)
  def('clientHeight', clientHeight)
}

const body = () => document.querySelector('.acc__body') as HTMLElement
const more = () => document.querySelector('.acc__more')

afterEach(() => {
  // Возвращаем геометрию по умолчанию, иначе следующий тест увидит чужие числа.
  Reflect.deleteProperty(HTMLElement.prototype, 'scrollHeight')
  Reflect.deleteProperty(HTMLElement.prototype, 'clientHeight')
})

describe('указатель «снизу есть ещё»', () => {
  it('появляется, пока низ тела не виден', () => {
    fakeOverflow({ scrollHeight: 1000, clientHeight: 500 })
    render(<Accordion sections={sections} defaultOpen="main" />)
    expect(more()).not.toBeNull()
  })

  it('пропадает, когда докрутили до низа', () => {
    fakeOverflow({ scrollHeight: 1000, clientHeight: 500 })
    render(<Accordion sections={sections} defaultOpen="main" />)
    expect(more()).not.toBeNull()

    // Прокрутка вниз до упора: 1000 - 500 - 500 = 0, снизу смотреть некуда.
    Object.defineProperty(body(), 'scrollTop', { configurable: true, value: 500 })
    fireEvent.scroll(body())
    expect(more()).toBeNull()
  })

  it('не появляется, если раздел помещается целиком', () => {
    fakeOverflow({ scrollHeight: 480, clientHeight: 500 })
    render(<Accordion sections={sections} defaultOpen="main" />)
    expect(more()).toBeNull()
  })

  it('лежит вне прокручиваемого тела, иначе уедет вместе с содержимым', () => {
    fakeOverflow({ scrollHeight: 1000, clientHeight: 500 })
    render(<Accordion sections={sections} defaultOpen="main" />)
    const hint = more()!
    // Родной контейнер — обёртка, а не .acc__body.
    expect(hint.closest('.acc__body')).toBeNull()
    expect(hint.closest('.acc__body-wrap')).not.toBeNull()
  })

  it('переезжает в другой раздел вместе с содержимым', () => {
    fakeOverflow({ scrollHeight: 1000, clientHeight: 500 })
    render(<Accordion sections={sections} defaultOpen="main" />)
    fireEvent.click(screen.getByText('Цвет и материал'))
    // Тело одно и то же, переехало содержимое — указатель обязан остаться
    // у нового раздела, а не отстать от первого.
    expect(more()!.closest('.acc__body-wrap')).not.toBeNull()
    expect(screen.getByRole('region', { name: 'Цвет и материал' })).toBeInTheDocument()
  })
})
