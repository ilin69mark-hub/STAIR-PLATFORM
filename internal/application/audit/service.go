package audit

import "context"

// Service — прикладной сервис аудита (BE-0002 Use Cases, EDR-0013).
// Записывает события (Record) и отдаёт историю (List*). Не зависит от
// транспорта и БД (инверсия зависимостей). Запись — best-effort: при
// ошибке журнала возвращает ошибку вызывающему, который логирует и
// продолжает бизнес-операцию (EDR-0013 §4.1).
type Service struct {
	repo Repository
}

// NewService создаёт сервис аудита.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Record фиксирует событие аудита (SEC-0013). ActorID/TenantID должны быть
// заполнены из контекста запроса вызывающим (EDR-0013 §4.2).
func (s *Service) Record(ctx context.Context, e *Event) error {
	// SEC-002: доменная валидация до порта. Транспорт уже проверяет
	// allowlist клиентских действий, но Record — публичная точка для всех
	// вызывающих (в т.ч. фоновых), поэтому проверка дублируется здесь.
	if err := e.Validate(); err != nil {
		return err
	}
	// AUDIT-002 (2026-09-27): единственная точка нормализации исхода. Record —
	// единственный нетестовый вызывающий repo.Insert, поэтому приведение
	// «не задан → ок» здесь означает, что в БД физически не может попасть
	// пустая строка (её отверг бы CHECK audit_events_result_check).
	//
	// Смысл: забытое Result больше не «съедает» событие молча. Раньше пустое
	// поле давало ошибку валидации, а девять вызовов из двенадцати глушили её
	// через `_ =` — действие происходило, а в журнале не появлялось ничего.
	e.Result = e.Result.OrOK()
	return s.repo.Insert(ctx, e)
}

// ListProjectAudit возвращает события проекта внутри tenant (SEC-0005),
// новые сверху. Требуется членство в проекте (проверка в вызывающем
// сервисе/транспорте); чужой/несуществующий проект — ErrNotFound.
func (s *Service) ListProjectAudit(ctx context.Context, tenantID, projectID string) ([]*Event, error) {
	return s.repo.ListByProject(ctx, tenantID, projectID, DefaultListLimit)
}

// ListTenantAudit возвращает события tenant (глобальный аудит). Требуется
// роль admin (проверка в транспорте).
func (s *Service) ListTenantAudit(ctx context.Context, tenantID string) ([]*Event, error) {
	return s.repo.ListByTenant(ctx, tenantID, DefaultListLimit)
}
