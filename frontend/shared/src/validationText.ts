// validationText — русские подписи для раздела валидации (S-P6). Бэкенд
// отдаёт коды и ключи element/severity латиницей; человекочитаемые подписи
// держим на клиенте в одном месте, чтобы админка и конструктор совпадали.

export type SeverityKey = 'error' | 'warning' | 'info' | 'critical'

const SEVERITY_RU: Record<string, string> = {
  error: 'Ошибка',
  warning: 'Предупреждение',
  info: 'Информация',
  critical: 'Критическая',
}

const ELEMENT_RU: Record<string, string> = {
  room: 'Помещение',
  clearance: 'Просвет',
  step_height: 'Ступень',
  tread_depth: 'Проступь',
  angle: 'Угол наклона',
  stringer_thickness: 'Толщина косоура',
  railing_height: 'Высота перил',
  stringer: 'Косоур',
  configuration: 'Параметры',
}

export function severityLabel(severity: string | null | undefined): string {
  if (!severity) return '—'
  return SEVERITY_RU[severity] ?? severity
}

export function elementLabel(element: string | null | undefined): string {
  if (!element) return '—'
  return ELEMENT_RU[element] ?? element
}

export function isValidColumnLabel(element: string | null | undefined): string {
  return elementLabel(element)
}

// isWarning — «горячая» подсветка строки только для критических/ошибок.
export function isCriticalSeverity(severity: string | null | undefined): boolean {
  return severity === 'error' || severity === 'critical'
}