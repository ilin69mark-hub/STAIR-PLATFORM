// Русские подписи материалов для витрины.
//
// Каталог на бэкенде (MFG-0005) хранит технические английские имена — они
// приезжают в /public/materials вместе с кодом, плотностью и ценой. Для
// русскоязычной витрины нужны человеческие названия, поэтому берём их из
// общего словаря материалов фронтендов (`@shared/config`), который уже
// синхронизирован с кодами каталога. Код и параметры — из API, подпись — из
// UI-словаря. TODO: перенести локализацию в каталог (i18n на бэкенде).
import { materialOptions } from '@shared/config'

const RU = new Map<string, string>(materialOptions.map((o) => [o.value, o.label]))

export function materialLabelRu(code: string, fallback: string): string {
  return RU.get(code) ?? fallback
}
