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

const projectCols = `id, name, description, status, created_at, updated_at`

func scanProject(row pgx.Row) (*project.Project, error) {
	var p project.Project
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepository) CreateProject(ctx context.Context, tenantID string, p *project.Project) error {
	// id генерируется БД (gen_random_uuid); timestamp — now().
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO projects (tenant_id, name, description, status) VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		tenantID, p.Name, p.Description, p.Status,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("project: create: %w", err)
	}
	return nil
}

func (r *ProjectRepository) GetProject(ctx context.Context, tenantID, id string) (*project.Project, error) {
	p, err := scanProject(r.pool.QueryRow(ctx,
		`SELECT `+projectCols+` FROM projects WHERE id = $1 AND tenant_id = $2`, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, project.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("project: get: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) ListProjects(ctx context.Context, tenantID string) ([]*project.Project, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+projectCols+` FROM projects WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
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

const configCols = `id, project_id, width_mm, height_mm, flight, step_height_mm,
	stringer_thickness_mm, step_thickness_mm, clearance_mm, railing_height_mm,
	comfort_step_mm, landing_width_mm, lower_step_count, created_at, updated_at`

func scanConfig(row pgx.Row) (*project.StairConfiguration, error) {
	var c project.StairConfiguration
	if err := row.Scan(&c.ID, &c.ProjectID, &c.WidthMM, &c.HeightMM, &c.Flight,
		&c.StepHeightMM, &c.StringerThicknessMM, &c.StepThicknessMM, &c.ClearanceMM,
		&c.RailingHeightMM, &c.ComfortStepMM, &c.LandingWidthMM, &c.LowerStepCount,
		&c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ProjectRepository) SaveConfiguration(ctx context.Context, c *project.StairConfiguration) error {
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO stair_configurations (project_id, width_mm, height_mm, flight,
			step_height_mm, stringer_thickness_mm, step_thickness_mm, clearance_mm,
			railing_height_mm, comfort_step_mm, landing_width_mm, lower_step_count)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 RETURNING id, created_at, updated_at`,
		c.ProjectID, c.WidthMM, c.HeightMM, c.Flight, c.StepHeightMM,
		c.StringerThicknessMM, c.StepThicknessMM, c.ClearanceMM, c.RailingHeightMM,
		c.ComfortStepMM, c.LandingWidthMM, c.LowerStepCount,
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
	if _, err := r.GetProject(ctx, tenantID, cfg.ProjectID); err != nil {
		return nil, err
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
			railing_height_mm, comfort_step_mm, landing_width_mm, lower_step_count)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 RETURNING id, created_at, updated_at`,
		cfg.ProjectID, cfg.WidthMM, cfg.HeightMM, cfg.Flight, cfg.StepHeightMM,
		cfg.StringerThicknessMM, cfg.StepThicknessMM, cfg.ClearanceMM, cfg.RailingHeightMM,
		cfg.ComfortStepMM, cfg.LandingWidthMM, cfg.LowerStepCount,
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
