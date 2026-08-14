package project

import "context"

// Repository — порт доступа к данным (BE-0005 Repository Layer).
// Application зависит от интерфейса; инфраструктура реализует его.
// Все методы скоупированы по tenant (SEC-0005): tenantID — граница
// изоляции; межтенантный доступ невозможен даже на уровне SQL.
type Repository interface {
	// CreateProject создаёт проект в tenant и возвращает его с ID.
	CreateProject(ctx context.Context, tenantID string, p *Project) error
	// GetProject возвращает проект по ID внутри tenant;
	// ErrNotFound — отсутствует или принадлежит другому tenant'у.
	GetProject(ctx context.Context, tenantID, id string) (*Project, error)
	// ListProjects возвращает проекты tenant'а в порядке создания.
	ListProjects(ctx context.Context, tenantID string) ([]*Project, error)

	// SaveConfiguration создаёт новую ревизию конфигурации проекта.
	SaveConfiguration(ctx context.Context, c *StairConfiguration) error
	// GetLatestConfiguration возвращает последнюю ревизию конфигурации
	// проекта внутри tenant.
	GetLatestConfiguration(ctx context.Context, tenantID, projectID string) (*StairConfiguration, error)

	// SaveCalculationWithConfig атомарно сохраняет конфигурацию и расчёт
	// (BE-0006 Transaction Management): одна транзакция — один расчёт.
	// Сериализацию Snapshot в JSON выполняет реализация (encoding/json
	// не разрешён в application, ADR-0006). Проект проверяется по tenant.
	SaveCalculationWithConfig(ctx context.Context, tenantID string, cfg *StairConfiguration, snap Snapshot) (*Calculation, error)
	// GetLatestCalculation возвращает последний расчёт проекта внутри tenant.
	GetLatestCalculation(ctx context.Context, tenantID, projectID string) (*Calculation, error)
}
