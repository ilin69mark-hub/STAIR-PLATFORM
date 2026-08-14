package project

import "context"

// Repository — порт доступа к данным (BE-0005 Repository Layer).
// Application зависит от интерфейса; инфраструктура реализует его.
// Все методы скоупированы по tenant (SEC-0005): tenantID — граница
// изоляции; межтенантный доступ невозможен даже на уровне SQL.
//
// Phase C (EDR-0008): доступ к проекту определяется членством
// (project_members). GetProject/ListProjects учитывают членство вызывающего;
// CreateProject фиксирует владельца.
type Repository interface {
	// CreateProject создаёт проект в tenant с владельцем ownerID и
	// оформляет членство владельца (роль owner) атомарно.
	CreateProject(ctx context.Context, tenantID, ownerID string, p *Project) error
	// GetProject возвращает проект по ID внутри tenant;
	// ErrNotFound — отсутствует, принадлежит другому tenant'у или
	// вызывающий не является членом.
	GetProject(ctx context.Context, tenantID, userID, id string) (*Project, error)
	// ListProjects возвращает проекты tenant'а, где вызывающий является
	// членом (владелец или участник), в порядке создания.
	ListProjects(ctx context.Context, tenantID, userID string) ([]*Project, error)

	// GetMember возвращает членство пользователя в проекте;
	// ErrNotFound — не член (или проект вне tenant).
	GetMember(ctx context.Context, tenantID, projectID, userID string) (*ProjectMember, error)
	// ListMembers возвращает членов проекта в порядке добавления.
	ListMembers(ctx context.Context, tenantID, projectID string) ([]*ProjectMember, error)
	// AddMember добавляет члена с ролью (не владелец — владелец уже
	// существует). Ошибка — дубликат или роль owner.
	AddMember(ctx context.Context, tenantID, projectID string, m *ProjectMember) error
	// AddMemberByEmail добавляет члена по email (C2, EDR-0008): адрес
	// резолвится в пользователя того же tenant (SEC-0005). ErrNotFound —
	// пользователь с таким email не найден или вне tenant.
	AddMemberByEmail(ctx context.Context, tenantID, projectID, email string, role ProjectRole) error
	// UpdateMemberRole изменяет роль члена (owner не понижается в role и
	// не может быть назначен повторно).
	UpdateMemberRole(ctx context.Context, tenantID, projectID, userID string, role ProjectRole) error
	// RemoveMember удаляет члена; владельца удалить нельзя (собственность
	// проекта остаётся за владельцем; перенос владения — вне скоупа C1).
	RemoveMember(ctx context.Context, tenantID, projectID, userID string) error

	// AddComment добавляет комментарий к проекту (EDR-0009, member authz
	// проверяет service). Возвращает созданный комментарий с ID/CreatedAt.
	AddComment(ctx context.Context, tenantID, projectID string, c *Comment) (*Comment, error)
	// ListComments возвращает комментарии проекта (по возрастанию created_at).
	ListComments(ctx context.Context, tenantID, projectID string) ([]*Comment, error)
	// DeleteComment удаляет комментарий. actorID может удалить собственный
	// комментарий или — если роль owner — любой. ErrNotFound — комментария
	// нет (или вне tenant); ErrForbidden — нет прав на удаление.
	DeleteComment(ctx context.Context, tenantID, projectID, commentID, actorID string) error

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
