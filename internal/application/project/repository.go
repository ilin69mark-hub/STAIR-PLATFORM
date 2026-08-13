package project

import "context"

// Repository — порт доступа к данным (BE-0005 Repository Layer).
// Application зависит от интерфейса; инфраструктура реализует его.
type Repository interface {
	// CreateProject создаёт проект и возвращает его с присвоенным ID.
	CreateProject(ctx context.Context, p *Project) error
	// GetProject возвращает проект по ID; ErrNotFound — отсутствует.
	GetProject(ctx context.Context, id string) (*Project, error)
	// ListProjects возвращает проекты в порядке создания (новые первыми).
	ListProjects(ctx context.Context) ([]*Project, error)

	// SaveConfiguration создаёт новую ревизию конфигурации проекта.
	SaveConfiguration(ctx context.Context, c *StairConfiguration) error
	// GetLatestConfiguration возвращает последнюю ревизию конфигурации.
	GetLatestConfiguration(ctx context.Context, projectID string) (*StairConfiguration, error)

	// SaveCalculationWithConfig атомарно сохраняет конфигурацию и расчёт
	// (BE-0006 Transaction Management): одна транзакция — один расчёт.
	// Сериализацию Snapshot в JSON выполняет реализация (encoding/json
	// не разрешён в application, ADR-0006).
	SaveCalculationWithConfig(ctx context.Context, cfg *StairConfiguration, snap Snapshot) (*Calculation, error)
	// GetLatestCalculation возвращает последний расчёт проекта.
	GetLatestCalculation(ctx context.Context, projectID string) (*Calculation, error)
}
