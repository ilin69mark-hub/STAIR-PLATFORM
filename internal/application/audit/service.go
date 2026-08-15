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
	return s.repo.Insert(ctx, e)
}

// ListProjectAudit возвращает события проекта внутри tenant (SEC-0005),
// новые сверху. Требуется членство в проекте (проверка в вызывающем
// сервисе/транспорте); чужой/несуществующий проект — ErrNotFound.
func (s *Service) ListProjectAudit(ctx context.Context, tenantID, projectID string) ([]*Event, error) {
	return s.repo.ListByProject(ctx, tenantID, projectID)
}

// ListTenantAudit возвращает события tenant (глобальный аудит). Требуется
// роль admin (проверка в транспорте).
func (s *Service) ListTenantAudit(ctx context.Context, tenantID string) ([]*Event, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}
