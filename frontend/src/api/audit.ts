// API аудита (Phase G, EDR-0013): журнал событий безопасности.

import { get } from './client'
import type { AuditEvent } from '@shared/types'

export const actionLabels: Record<string, string> = {
  'auth.register': 'Регистрация',
  'auth.login': 'Вход',
  'auth.logout': 'Выход',
  'auth.login_denied': 'Вход отклонён',
  'authz.denied': 'Отказ в доступе',
  'user.role_changed': 'Смена роли пользователя',
  'user.status_changed': 'Смена статуса пользователя',
  'settings.updated': 'Обновление политики безопасности',
  'data.exported': 'Экспорт данных',
  'api_key.created': 'Создание API-ключа',
  'api_key.revoked': 'Отзыв API-ключа',
  'project.created': 'Создание проекта',
  'project.modified': 'Изменение проекта',
  'member.added': 'Добавление участника',
  'member.role_changed': 'Смена роли',
  'member.removed': 'Удаление участника',
  'config.approved': 'Утверждение конфигурации',
  'config.restored': 'Восстановление версии',
  'review.requested': 'Запрос ревью',
  'review.signed': 'Подпись ревью',
  'review.changes': 'Запрос изменений',
  'stair.calculated': 'Расчёт лестницы',
  'stair.config_changed': 'Изменение поля конфигурации',
  'stair.suggestion_applied': 'Применён совет',
  'stair.variation_applied': 'Применён вариант',
}

export const auditApi = {
  listProjectAudit: (projectId: string) => get<AuditEvent[]>(`/api/v1/projects/${projectId}/audit`),

  listTenantAudit: () => get<AuditEvent[]>('/api/v1/audit'),
}
