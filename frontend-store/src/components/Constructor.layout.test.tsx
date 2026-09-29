import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

// Контракт компоновки панели. Проверяем правила CSS, а не поведение:
// загрузка стилей в общий набор тестов меняет результаты toBeVisible(),
// поэтому здесь читаются сами файлы.
//
// Мотивация — реальный дефект: длинная подпись ползунка «Свободное
// пространство перед маршем» давала горизонтальную полосу прокрутки в теле
// секции. Причина в том, что .field-label — флекс-потомок с min-width: auto
// по умолчанию, то есть он не сжимался уже своего nowrap-контента, из-за
// чего .slider__head распирался, а .acc__body (overflow-y: auto) по
// спецификации получал overflow-x: auto.

// Путь от cwd (корень vitest — frontend-store), а НЕ от import.meta.url:
// в пути проекта есть пробел («STAIR PLATFORM»), и fileURLToPath на
// неэкодированном import.meta.url возвращает не тот файл — тест читал
// пустоту и падал на expect('').toContain(...)
const css = (name: string) =>
  readFileSync(resolve(process.cwd(), '../frontend/shared/src/storefront/styles', name), 'utf8')

describe('компоновка панели конструктора', () => {
  it('обёртка подписи ползунка умеет сжиматься', () => {
    const block = css('constructor.css').match(/\.field-label\s*\{[^}]*\}/)?.[0] ?? ''
    expect(block).toContain('min-width: 0')
  })

  it('тело открытой секции не получает горизонтальный скролл', () => {
    const block = css('controls.css').match(/\.calc__rail \.acc__body\s*\{[^}]*\}/)?.[0] ?? ''
    expect(block).toContain('overflow-y: auto')
    expect(block).toContain('overflow-x: hidden')
  })

  it('чипы перил ниже, чем остальные сегменты', () => {
    const controls = css('controls.css')
    const compact = controls.match(/\.segmented\[data-compact\] \.segmented__item\s*\{[^}]*\}/)?.[0] ?? ''
    expect(compact).toMatch(/min-height: (2[0-9]|3[0-1])px/)
    // Обычный сегмент выше — иначе правило compact ничего не меняет.
    const normal = controls.match(/\n\.segmented__item\s*\{[^}]*\}/)?.[0] ?? ''
    const normalH = Number(normal.match(/min-height:\s*(\d+)px/)?.[1] ?? 0)
    const compactH = Number(compact.match(/min-height:\s*(\d+)px/)?.[1] ?? 99)
    expect(compactH).toBeLessThan(normalH)
  })

  it('числовой чип ползунка ужат, чтобы подписи хватало места', () => {
    const block = css('controls.css').match(/\.slider__number\s*\{[^}]*\}/)?.[0] ?? ''
    const w = Number(block.match(/width:\s*(\d+)px/)?.[1] ?? 999)
    expect(w).toBeLessThanOrEqual(40)
    expect(block).toMatch(/font-size: 1[12]px/)
  })

  // Регрессия: .field input (specificity 0,1,1) перебивал .slider__number
  // (0,1,0), поэтому поле получало padding 10px 12px и шрифт 14px. В боксе
  // 42px оставалось 16px контента, и числа обрезались; у ползунка padding
  // съедал высоту дорожки, у галочки 14px — больше суммы отступов.
  it('базовые отступы поля не применяются к ползунку и галочке', () => {
    const base = css('constructor.css').match(
      /\.field input:not\(\[type='checkbox'\]\):not\(\[type='range'\]\)[^{]*\{[^}]*\}/,
    )
    expect(base).toBeTruthy()
    // Ни у .slider__number, ни у .slider__range, ни у input[type=checkbox]
    // собственных отступов нет — им нечего перебивать.
    const controls = css('controls.css')
    expect(controls).toMatch(/\.slider__number\s*\{[^}]*padding: 0 3px/)
    expect(controls).toMatch(/\.slider__range\s*\{[^}]*height: 14px/)
  })

  it('дорожка ползунка и блок «Подступень» ниже, чем были', () => {
    const controls = css('controls.css')
    const range = controls.match(/\.slider__range\s*\{[^}]*\}/)?.[0] ?? ''
    expect(Number(range.match(/height:\s*(\d+)px/)?.[1] ?? 99)).toBeLessThanOrEqual(14)

    const constructor_ = css('constructor.css')
    const box = constructor_.match(/\.field \.checkbox\s*\{[^}]*\}/)?.[0] ?? ''
    // 3px 8px вместо 10px 12px — блок не должен занимать две строки.
    expect(box).toMatch(/padding: 3px 8px/)
    const tick = constructor_.match(/\.field \.checkbox input\[type='checkbox'\]\s*\{[^}]*\}/)?.[0] ?? ''
    expect(Number(tick.match(/width:\s*(\d+)px/)?.[1] ?? 99)).toBeLessThanOrEqual(14)
  })
})
