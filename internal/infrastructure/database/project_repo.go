package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stairplatform/internal/application/project"
)

// ProjectRepository — реализация порта Repository (BE-0005) на PostgreSQL
// через пул pgx. Сущности маппятся на таблицы projects,
// stair_configurations, calculations.
type ProjectRepository struct {
	pool *pgxpool.Pool
}

// NewProjectRepository создаёт репозиторий поверх пула.
func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

var _ project.Repository = (*ProjectRepository)(nil)

const projectCols = `id, name, description, status, owner_id, created_at, updated_at`

func scanProject(row pgx.Row) (*project.Project, error) {
	var p project.Project
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Status, &p.OwnerID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateProject создаёт проект с владельцем ownerID и оформляет членство
// владельца атомарно (EDR-0008). Проект и членство владельца — одна
// транзакция: проект не существует без владельца.
func (r *ProjectRepository) CreateProject(ctx context.Context, tenantID, ownerID string, p *project.Project) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("project: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx,
		`INSERT INTO projects (tenant_id, owner_id, name, description, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, updated_at`,
		tenantID, ownerID, p.Name, p.Description, p.Status,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("project: create: %w", err)
	}
	p.OwnerID = ownerID

	if _, err := tx.Exec(ctx,
		`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'owner')`,
		p.ID, ownerID); err != nil {
		return fmt.Errorf("project: create owner membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("project: commit: %w", err)
	}
	return nil
}

// projectScope — условие видимости проекта: вызывающий должен быть членом
// (владелец или участник). Проект вне tenant не попадает в выборку
// (SEC-0005); членство проверяется через project_members.
func projectScope() string {
	return `id = $1 AND tenant_id = $2
		AND EXISTS (SELECT 1 FROM project_members pm
			WHERE pm.project_id = projects.id AND pm.user_id = $3)`
}

func (r *ProjectRepository) GetProject(ctx context.Context, tenantID, userID, id string) (*project.Project, error) {
	p, err := scanProject(r.pool.QueryRow(ctx,
		`SELECT `+projectCols+` FROM projects WHERE `+projectScope(), id, tenantID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, project.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("project: get: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) ListProjects(ctx context.Context, tenantID, userID string) ([]*project.Project, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.name, p.description, p.status, p.owner_id, p.created_at, p.updated_at
		 FROM projects p
		 WHERE p.tenant_id = $1
		   AND EXISTS (SELECT 1 FROM project_members pm
		        WHERE pm.project_id = p.id AND pm.user_id = $2)
		 ORDER BY p.created_at DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("project: list: %w", err)
	}
	defer rows.Close()
	var out []*project.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("project: list: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---- members (EDR-0008) ----

const memberCols = `project_id, user_id, role, created_at`

func scanMember(row pgx.Row) (*project.ProjectMember, error) {
	var m project.ProjectMember
	if err := row.Scan(&m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ProjectRepository) GetMember(ctx context.Context, tenantID, projectID, userID string) (*project.ProjectMember, error) {
	m, err := scanMember(r.pool.QueryRow(ctx,
		`SELECT pm.project_id, pm.user_id, pm.role, pm.created_at
		 FROM project_members pm
		 JOIN projects p ON p.id = pm.project_id
		 WHERE pm.project_id = $1 AND pm.user_id = $2 AND p.tenant_id = $3`,
		projectID, userID, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, project.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("project: get member: %w", err)
	}
	return m, nil
}

func (r *ProjectRepository) ListMembers(ctx context.Context, tenantID, projectID string) ([]*project.ProjectMember, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT pm.project_id, pm.user_id, pm.role, pm.created_at
		 FROM project_members pm
		 JOIN projects p ON p.id = pm.project_id
		 WHERE pm.project_id = $1 AND p.tenant_id = $2
		 ORDER BY pm.created_at ASC`, projectID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("project: list members: %w", err)
	}
	defer rows.Close()
	var out []*project.ProjectMember
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, fmt.Errorf("project: list members: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *ProjectRepository) AddMember(ctx context.Context, tenantID, projectID string, m *project.ProjectMember) error {
	// Добавляемый пользователь должен существовать в том же tenant (SEC-0005).
	var uid string
	if err := r.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE id = $1 AND tenant_id = (SELECT tenant_id FROM projects WHERE id = $2)`,
		m.UserID, projectID).Scan(&uid); errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("project: member user not found: %w", project.ErrNotFound)
	} else if err != nil {
		return fmt.Errorf("project: member tenant check: %w", err)
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)`,
		projectID, m.UserID, string(m.Role)); err != nil {
		return fmt.Errorf("project: add member: %w", err)
	}
	return nil
}

func (r *ProjectRepository) UpdateMemberRole(ctx context.Context, tenantID, projectID, userID string, role project.ProjectRole) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE project_members pm SET role = $3
		 FROM projects p
		 WHERE pm.project_id = $1 AND pm.user_id = $2 AND p.id = $1 AND p.tenant_id = $4
		   AND pm.role <> 'owner'`,
		projectID, userID, string(role), tenantID)
	if err != nil {
		return fmt.Errorf("project: update member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project: member not found or is owner: %w", project.ErrNotFound)
	}
	return nil
}

func (r *ProjectRepository) RemoveMember(ctx context.Context, tenantID, projectID, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM project_members pm
		 USING projects p
		 WHERE pm.project_id = $1 AND pm.user_id = $2 AND p.id = $1 AND p.tenant_id = $3
		   AND pm.role <> 'owner'`,
		projectID, userID, tenantID)
	if err != nil {
		return fmt.Errorf("project: remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project: member not found or is owner: %w", project.ErrNotFound)
	}
	return nil
}

const configCols = `id, project_id, width_mm, height_mm, flight, step_height_mm,
	stringer_thickness_mm, step_thickness_mm, clearance_mm, railing_height_mm,
	comfort_step_mm, landing_width_mm, lower_step_count, outer_radius_mm, created_at, updated_at`

func scanConfig(row pgx.Row) (*project.StairConfiguration, error) {
	var c project.StairConfiguration
	if err := row.Scan(&c.ID, &c.ProjectID, &c.WidthMM, &c.HeightMM, &c.Flight,
		&c.StepHeightMM, &c.StringerThicknessMM, &c.StepThicknessMM, &c.ClearanceMM,
		&c.RailingHeightMM, &c.ComfortStepMM, &c.LandingWidthMM, &c.LowerStepCount,
		&c.OuterRadiusMM, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ProjectRepository) SaveConfiguration(ctx context.Context, c *project.StairConfiguration) error {
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO stair_configurations (project_id, width_mm, height_mm, flight,
			step_height_mm, stringer_thickness_mm, step_thickness_mm, clearance_mm,
			railing_height_mm, comfort_step_mm, landing_width_mm, lower_step_count, outer_radius_mm)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 RETURNING id, created_at, updated_at`,
		c.ProjectID, c.WidthMM, c.HeightMM, c.Flight, c.StepHeightMM,
		c.StringerThicknessMM, c.StepThicknessMM, c.ClearanceMM, c.RailingHeightMM,
		c.ComfortStepMM, c.LandingWidthMM, c.LowerStepCount, c.OuterRadiusMM,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return fmt.Errorf("project: save config: %w", err)
	}
	return nil
}

func (r *ProjectRepository) GetLatestConfiguration(ctx context.Context, tenantID, projectID string) (*project.StairConfiguration, error) {
	c, err := scanConfig(r.pool.QueryRow(ctx,
		`SELECT `+configCols+` FROM stair_configurations
		 WHERE project_id = $1
		   AND project_id IN (SELECT id FROM projects WHERE tenant_id = $2 AND id = $1)
		 ORDER BY created_at DESC LIMIT 1`, projectID, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, project.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("project: get config: %w", err)
	}
	return c, nil
}

func (r *ProjectRepository) SaveCalculation(ctx context.Context, c *project.Calculation) error {
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO calculations (project_id, configuration_id, valid, blocking, result)
		 VALUES ($1, $2, $3, $4, $5::jsonb)
		 RETURNING id, created_at`,
		c.ProjectID, c.ConfigurationID, c.Valid, c.Blocking, string(c.Result),
	).Scan(&c.ID, &c.CreatedAt); err != nil {
		return fmt.Errorf("project: save calculation: %w", err)
	}
	return nil
}

// SaveCalculationWithConfig атомарно сохраняет конфигурацию и расчёт
// (BE-0006): одна транзакция. Сериализация Snapshot выполняется здесь —
// encoding/json разрешён в infrastructure (ADR-0006). Проверяется, что
// проект принадлежит tenant'у (SEC-0005).
func (r *ProjectRepository) SaveCalculationWithConfig(ctx context.Context, tenantID string, cfg *project.StairConfiguration, snap project.Snapshot) (*project.Calculation, error) {
	var exists string
	if err := r.pool.QueryRow(ctx,
		`SELECT id FROM projects WHERE id = $1 AND tenant_id = $2`, cfg.ProjectID, tenantID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return nil, project.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("project: check: %w", err)
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("project: marshal snapshot: %w", err)
	}
	calc := &project.Calculation{
		ProjectID: cfg.ProjectID,
		Valid:     snap.Validation.Valid,
		Blocking:  snap.Validation.Blocking,
		Result:    payload,
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("project: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx,
		`INSERT INTO stair_configurations (project_id, width_mm, height_mm, flight,
			step_height_mm, stringer_thickness_mm, step_thickness_mm, clearance_mm,
			railing_height_mm, comfort_step_mm, landing_width_mm, lower_step_count, outer_radius_mm)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 RETURNING id, created_at, updated_at`,
		cfg.ProjectID, cfg.WidthMM, cfg.HeightMM, cfg.Flight, cfg.StepHeightMM,
		cfg.StringerThicknessMM, cfg.StepThicknessMM, cfg.ClearanceMM, cfg.RailingHeightMM,
		cfg.ComfortStepMM, cfg.LandingWidthMM, cfg.LowerStepCount, cfg.OuterRadiusMM,
	).Scan(&cfg.ID, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
		return nil, fmt.Errorf("project: save config: %w", err)
	}

	if err := tx.QueryRow(ctx,
		`INSERT INTO calculations (project_id, configuration_id, valid, blocking, result)
		 VALUES ($1, $2, $3, $4, $5::jsonb)
		 RETURNING id, created_at`,
		calc.ProjectID, cfg.ID, calc.Valid, calc.Blocking, string(calc.Result),
	).Scan(&calc.ID, &calc.CreatedAt); err != nil {
		return nil, fmt.Errorf("project: save calculation: %w", err)
	}
	calc.ConfigurationID = cfg.ID

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("project: commit: %w", err)
	}
	return calc, nil
}

func (r *ProjectRepository) GetLatestCalculation(ctx context.Context, tenantID, projectID string) (*project.Calculation, error) {
	var c project.Calculation
	var raw []byte
	if err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.project_id, c.configuration_id, c.valid, c.blocking, c.result, c.created_at
		 FROM calculations c
		 WHERE c.project_id = $1
		   AND c.project_id IN (SELECT id FROM projects WHERE tenant_id = $2 AND id = $1)
		 ORDER BY c.created_at DESC LIMIT 1`, projectID, tenantID,
	).Scan(&c.ID, &c.ProjectID, &c.ConfigurationID, &c.Valid, &c.Blocking, &raw, &c.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, project.ErrNotFound
		}
		return nil, fmt.Errorf("project: get calculation: %w", err)
	}
	c.Result = raw
	return &c, nil
}
