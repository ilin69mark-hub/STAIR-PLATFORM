import * as THREE from 'three'
import { EffectComposer } from 'three/examples/jsm/postprocessing/EffectComposer.js'
import { RenderPass } from 'three/examples/jsm/postprocessing/RenderPass.js'
import { GTAOPass } from 'three/examples/jsm/postprocessing/GTAOPass.js'
import { OutputPass } from 'three/examples/jsm/postprocessing/OutputPass.js'

/**
 * Затенение контактов (ambient occlusion) пост-обработкой.
 *
 * Зачем: сцена из плоских светлых деталей на белом фоне выглядела «висящей
 * в пустоте» — не было видно, где ступень касается косоура, где поручень
 * стоит на площадке, где изделие стоит на полу. Мягкое затенение в этих
 * стыках даёт ощущение вещественности без единой дополнительной лампы.
 *
 * Почему GTAO, а не SSAO/SAO: это тот же класс эффекта (затенение по
 * пост-обработке), но GTAO считает по восстановленной геометрии, а не по
 * экранному поиску, поэтому даёт меньше «каши» на рёбрах и заметно дешевле
 * по кадру. Старый SSAOPass на таких тонких деталях (4 мм лист, 8 мм
 * косоур) давал бы грязь.
 *
 * Почему OutputPass последним: при рендере в render-target three
 * отключает тонмаппинг (toneMapping = NoToneMapping для целей рендера), и
 * ACES + sRGB применяются уже в финальном проходе. Без него сцена была бы
 * бледной/пересвеченной — ровно та проблема, которую чинили в этапе 1.
 */
export interface PostFX {
  composer: EffectComposer
  gtao: GTAOPass
  setSize: (width: number, height: number) => void
  dispose: () => void
}

// Матрицы и формула — копия ACESFilmicToneMapping из three
// (tonemapping_pars_fragment.glsl.js). Нужны, чтобы найти белую точку фона.
const ACES_IN: readonly number[] = [
  0.59719, 0.076, 0.0284,
  0.35458, 0.90834, 0.13383,
  0.04823, 0.01566, 0.83777,
]
const ACES_OUT: readonly number[] = [
  1.60475, -0.10208, -0.00327,
  -0.53108, 1.10813, -0.07276,
  -0.07367, -0.00605, 1.07602,
]

/** ACES для серого входа, как в шейдере three (GLSL mat3 — по столбцам). */
function acesRgb(value: number, exposure: number): number[] {
  const v = [value * (exposure / 0.6), value * (exposure / 0.6), value * (exposure / 0.6)]
  const mul = (m: readonly number[], c: number[]) => [
    m[0] * c[0] + m[3] * c[1] + m[6] * c[2],
    m[1] * c[0] + m[4] * c[1] + m[7] * c[2],
    m[2] * c[0] + m[5] * c[1] + m[8] * c[2],
  ]
  let c = mul(ACES_IN, v)
  c = c.map((x) => {
    const a = x * (x + 0.0245786) - 0.000090537
    const b = x * (0.983729 * x + 0.432951) + 0.238081
    return a / b
  })
  return mul(ACES_OUT, c).map((x) => Math.min(1, Math.max(0, x)))
}

/** Тонмаппинг серого: возвращает КРАСНЫЙ канал (так же, как считал шейдер). */
export function acesToneMappedGrey(value: number, exposure: number): number {
  return acesRgb(value, exposure)[0]
}

/**
 * Белая точка фона для ACES.
 *
 * Фон сцены — это просто clear-цвет кадра, и он проходит по той же цепочке,
 * что и геометрия: RenderPass кладёт линейное 1.0 в буфер, а OutputPass
 * тонмаппит его в 0.8 → на экране #e8e8e8 вместо белого. Продуктовый «белый
 * циклорамный» фон (решение владельца) от этого серел.
 *
 * Поэтому цвет фона ставится не 1.0, а таким значением, которое ПОСЛЕ
 * ACES даёт ровно белый. Ищем его бисекцией по формуле выше — константой
 * здесь нельзя: экспозиция уже менялась (1.05) и поедет снова.
 *
 * Ищем по МИНИМУМУ каналов, а не по красному: ACES слегка красит серый, и
 * если остановиться на первом ушедшем в 1.0 канале, зелёный и синий останутся
 * чуть ниже — фон будет не белым, а еле заметно тёплым.
 */
export function acesWhitePoint(exposure: number): number {
  let lo = 1
  // Верхняя граница поиска с запасом: чем выше экспозиция, тем больше нужно
  // линейного значения (при 1.2 требуется ~17, при 1.05 — ~14.7). С узкой
  // границей бисекция упиралась в потолок и возвращала недобелый фон.
  let hi = 64
  for (let i = 0; i < 40; i++) {
    const mid = (lo + hi) / 2
    if (Math.min(...acesRgb(mid, exposure)) < 1) lo = mid
    else hi = mid
  }
  return hi
}

export function createPostFX(
  renderer: THREE.WebGLRenderer,
  scene: THREE.Scene,
  camera: THREE.Camera,
  width: number,
  height: number,
  /** Габарит сцены в мм — по нему подбирается радиус затенения. */
  sceneRadiusMM: number,
): PostFX {
  const composer = new EffectComposer(renderer)
  composer.setPixelRatio(renderer.getPixelRatio())
  composer.setSize(width, height)
  composer.addPass(new RenderPass(scene, camera))

  const gtao = new GTAOPass(scene, camera, width, height)
  gtao.output = GTAOPass.OUTPUT.Default
  // Затенение — не главный свет, а «подкраска» контактов. На полной силе
  // белые ступени на белом фоне сереют и выглядят грязными, поэтому
  // интенсивность ниже единицы.
  gtao.blendIntensity = 0.85
  gtao.updateGtaoMaterial({
    // Радиус в МИРОВЫХ единицах сцены, то есть в миллиметрах. Сцена у нас
    // около 3000 мм, и «родные» 0.25 из примеров three дали бы невидимую
    // крупинку. Считаем от габарита: ~2% от размера сцены — это как раз
    // зазор между ступенью и косоуром.
    radius: THREE.MathUtils.clamp(sceneRadiusMM * 0.02, 20, 120),
    // Поверхность тонкая (лист 4 мм, косоур 8 мм): thickness должен быть
    // сопоставим с радиусом, иначе GTAO считает детали «бесконечно толстыми
    // пластинами» и затеняет их изнутри.
    thickness: THREE.MathUtils.clamp(sceneRadiusMM * 0.015, 15, 90),
    distanceExponent: 1.0,
    scale: 1.0,
    samples: 16,
    screenSpaceRadius: false,
  })
  // Сглаживание AO кольцами Пуассона: без него по краям ступеней видно
  // «проседание» тона. Сэмплов денойза 8 вместо 16: AO считается в
  // половинном разрешении, и разницы глазом не видно, а работы вдвое меньше.
  // Именно updatePdMaterial, а не присваивание pdSamples: сэмпл-векторы
  // генерируются в нём, и «ручная» правка поля дала бы шейдер со старыми
  // векторами и рассыпанным шумом.
  gtao.updatePdMaterial({ radiusExponent: 2, rings: 2, samples: 8 })
  composer.addPass(gtao)
  composer.addPass(new OutputPass())

  // AO считаем в половинном разрешении: затенение низкочастотное, разницы
  // глазом не видно, а работы вдвое меньше. composer.setSize дёргает setSize
  // у ВСЕХ проходов, поэтому половинный размер AO ставим после него.
  const half = (w: number, h: number) => gtao.setSize(Math.max(2, Math.round(w / 2)), Math.max(2, Math.round(h / 2)))
  half(width, height)

  return {
    composer,
    gtao,
    setSize: (w, h) => {
      composer.setSize(w, h)
      half(w, h)
    },
    dispose: () => {
      gtao.dispose()
      composer.dispose()
    },
  }
}
