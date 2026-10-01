// Подпись материала для витрины.
//
// Русское витринное имя приезжает с API (`name_ru`, MFG-0005 NameRu) — это
// источник истины. Словарь фронтендов остаётся фолбэком на случай, если каталог
// пополнится раньше, чем обновится статический словарь: тогда покажем техническое
// имя из API, а не сломаем страницу.
import { materialOptions } from '@shared/config'

const RU = new Map<string, string>(materialOptions.map((o) => [o.value, o.label]))

export function materialLabelRu(code: string, nameRu: string | undefined, name = ''): string {
  return nameRu?.trim() || RU.get(code) || name || code
}
