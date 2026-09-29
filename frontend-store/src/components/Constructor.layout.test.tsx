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

  // Карточки результата стоят в том же рельсе, что и конструктор. Раньше они
  // наследовали от .panel `flex: 1 1 auto; min-height: 0`, рельс сжимал все
  // шесть карточек, и содержимое панели конструктора (вкладки, кнопки
  // «Рассчитать/Сбросить») вылезало наружу и ложилось поверх результата.
  it('карточки результата не сжимаются, а рельс прокручивается', () => {
    const calc = css('calc.css')
    const rail = calc.match(/\.calc__rail\s*\{[^}]*\}/)?.[0] ?? ''
    expect(rail).toContain('overflow-y: auto')
    const result = calc.match(/\.calc__rail \.panel--result\s*\{[^}]*\}/)?.[0] ?? ''
    expect(result).toMatch(/flex:\s*0 0 auto/)
    expect(result).toContain('min-height: 0')
    // У панели конструктора есть пол: ниже него вкладкам и кнопкам негде
    // развернуться, поэтому дальше сжимать нельзя — прокручивается рельс.
    const panel = calc.match(/\.calc__rail \.panel\s*\{[^}]*\}/)?.[0] ?? ''
    expect(panel).toMatch(/min-height:\s*\d+px/)
  })

  // Указатель «снизу есть ещё» обязан лежать ВНЕ прокручиваемого тела:
  // position: absolute внутри скроллера уезжает вместе с содержимым, и
  // стрелка исчезла бы с экрана на первом же прокрутке.
  it('указатель прокрутки живёт в обёртке, а не в прокручиваемом теле', () => {
    const controls = css('controls.css')
    const wrap = controls.match(/\.acc__body-wrap\s*\{[^}]*\}/)?.[0] ?? ''
    expect(wrap).toContain('position: relative')
    const more = controls.match(/\.acc__more\s*\{[^}]*\}/)?.[0] ?? ''
    expect(more).toContain('position: absolute')
    // Под стрелкой всё ещё лежат поля раздела — клик должен проходить.
    expect(more).toContain('pointer-events: none')
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

  // Регрессия: чип был width 40px, то есть content-box 32px, а четыре цифры с
  // курсором занимают 33.3px (замерено в Chromium: цифра 7.56px при 12px,
  // system-ui + tabular-nums). Как только курсор вставал в конец, старшая цифра
  // уезжала за левый край — обрезались все четырёхзначные пределы каталога
  // (высота 6000, ширина марша 3000, глубина площадки 5000, помещение 8000).
  // Закрепляем саму арифметику, а не конкретные пиксели: сколько бы CSS ни
  // менялся, content-box обязан перекрывать 4 цифры + курсор, а сам чип —
  // оставаться ниже 17px, чтобы не давить на подпись ползунка.
  it('числовой чип ползунка вмещает четыре цифры и остаётся низким', () => {
    const block = css('controls.css').match(/\.slider__number\s*\{[^}]*\}/)?.[0] ?? ''
    const width = Number(block.match(/(?<!-)width:\s*(\d+)px/)?.[1] ?? 0)
    const padX = Number(block.match(/padding:\s*0\s+(\d+)px/)?.[1] ?? 0)
    const contentBox = width - 2 * padX - 2 // border 1px с каждой стороны
    // 4 × 7.56px + курсор 3.1px = 33.3px → округляем вверх до 34.
    expect(contentBox).toBeGreaterThanOrEqual(34)
    const fontSize = Number(block.match(/font-size:\s*(\d+)px/)?.[1] ?? 99)
    const lineHeight = Number(block.match(/line-height:\s*([\d.]+)/)?.[1] ?? 99)
    expect(lineHeight * fontSize + 2).toBeLessThanOrEqual(17)
    // Подпись ползунка по-прежнему сжимается многоточием, а не выталкивает чип.
    expect(css('constructor.css')).toMatch(/\.field-label\s*\{[^}]*min-width: 0/)
  })

  // Регрессия: .field input:not([type=checkbox]):not([type=range])
  // (specificity 0,2,3) перебивал .slider__number (0,1,0) — причём всегда,
  // независимо от порядка правил. Чип — type="text", так что два этих
  // :not() его не исключали: он получал padding 10px 12px и font: inherit
  // (16px), то есть 54×46.8px вместо 54×16.4px и content-box 28px вместо
  // 44px. Четырёхзначные значения обрезались. Проверяем, что базовое
  // правило теперь исключает чип явным :not(.slider__number).
  it('базовые отступы поля не применяются к ползунку и галочке', () => {
    const base = css('constructor.css').match(
      /\.field input:not\(\[type='checkbox'\]\):not\(\[type='range'\]\)[^{]*\{[^}]*\}/,
    )
    expect(base).toBeTruthy()
    expect(base![0]).toMatch(/:not\(\.slider__number\)/)
    // Ни у .slider__range, ни у input[type=checkbox] собственных отступов
    // нет — им нечего перебивать. Собственный отступ числового поля нужен
    // ещё и для контента: при padding 0 ободок рамки съедал бы его.
    const controls = css('controls.css')
    expect(controls).toMatch(/\.slider__number\s*\{[^}]*padding: 0 4px/)
    expect(controls).toMatch(/\.slider__range\s*\{[^}]*height: 14px/)
    // Плюс в index.css витрины есть `.field input` (0,1,1) с border-radius
    // 8px — он перебил бы (0,1,0), поэтому чип объявлен как
    // `.field .slider__number` (0,2,0).
    expect(controls).toMatch(/\.field \.slider__number\s*\{/)
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
