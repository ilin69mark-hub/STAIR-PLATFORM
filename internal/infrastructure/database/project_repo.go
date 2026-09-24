package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	return WithTx(ctx, r.pool, func(tx pgx.Tx) error {
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

		return nil
	})
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

// ListTenantProjects возвращает все проекты tenant (EDR-0016: экспорт
// данных и admin-overview); членство не требуется.
func (r *ProjectRepository) ListTenantProjects(ctx context.Context, tenantID string) ([]*project.Project, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.name, p.description, p.status, p.owner_id, p.created_at, p.updated_at
		 FROM projects p
		 WHERE p.tenant_id = $1
		 ORDER BY p.created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("project: list tenant projects: %w", err)
	}
	defer rows.Close()
	var out []*project.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("project: list tenant projects: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---- members (EDR-0008) ----

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

// AddMemberByEmail добавляет члена по email (C2, EDR-0008): резолвит
// email в пользователя того же tenant (SEC-0005).
func (r *ProjectRepository) AddMemberByEmail(ctx context.Context, tenantID, projectID, email string, role project.ProjectRole) error {
	var uid string
	if err := r.pool.QueryRow(ctx,
		`SELECT id FROM users
		 WHERE email = $1 AND tenant_id = (SELECT tenant_id FROM projects WHERE id = $2)`,
		email, projectID,
	).Scan(&uid); errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("project: member by email not found: %w", project.ErrNotFound)
	} else if err != nil {
		return fmt.Errorf("project: member by email lookup: %w", err)
	}
	return r.AddMember(ctx, tenantID, projectID, &project.ProjectMember{ProjectID: projectID, UserID: uid, Role: role})
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

const configCols = `id, project_id, revision, width_mm, height_mm, flight, step_height_mm,
	stringer_thickness_mm, step_thickness_mm, riser, clearance_mm, railing_height_mm,
	comfort_step_mm, landing_width_mm, landing_depth_mm, room_width_mm, room_length_mm,
	approach_space_mm, lower_step_count, outer_radius_mm, created_at, updated_at`

func scanConfig(row pgx.Row) (*project.StairConfiguration, error) {
	var c project.StairConfiguration
	if err := row.Scan(&c.ID, &c.ProjectID, &c.Revision, &c.WidthMM, &c.HeightMM, &c.Flight,
		&c.StepHeightMM, &c.StringerThicknessMM, &c.StepThicknessMM, &c.Riser, &c.ClearanceMM,
		&c.RailingHeightMM, &c.ComfortStepMM, &c.LandingWidthMM, &c.LandingDepthMM,
		&c.RoomWidthMM, &c.RoomLengthMM, &c.ApproachSpaceMM, &c.LowerStepCount,
		&c.OuterRadiusMM, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// SaveConfiguration сохраняет конфигурацию проекта (новая ревизия).
// DB-001 (forensic 2026-09-24): вставка идёт в транзакции с
// `SELECT ... FOR UPDATE` по строке проекта — иначе конкурентные вызовы
// вычисляют одинаковую ревизию (MAX+1) и второй падает на
// UNIQUE(project_id, revision). Блокировка живёт ровно до COMMIT.
func (r *ProjectRepository) SaveConfiguration(ctx context.Context, c *project.StairConfiguration) error {
	return WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		var exists string
		if err := tx.QueryRow(ctx, `SELECT id FROM projects WHERE id = $1 FOR UPDATE`, c.ProjectID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
			return project.ErrNotFound
		} else if err != nil {
			return fmt.Errorf("project: lock project: %w", err)
		}
		return insertConfiguration(ctx, tx, c)
	})
}

// insertConfiguration вставляет конфигурацию с вычислением ревизии внутри
// уже открытой транзакции (блокировка проекта должна быть взята вызывающим).
func insertConfiguration(ctx context.Context, tx pgx.Tx, c *project.StairConfiguration) error {
	if err := tx.QueryRow(ctx,
		`INSERT INTO stair_configurations (project_id, width_mm, height_mm, flight,
			step_height_mm, stringer_thickness_mm, step_thickness_mm, riser, clearance_mm,
			railing_height_mm, comfort_step_mm, landing_width_mm, landing_depth_mm,
			room_width_mm, room_length_mm, approach_space_mm, lower_step_count, outer_radius_mm, revision)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,
		   (SELECT COALESCE(MAX(s.revision),0)+1 FROM stair_configurations s WHERE s.project_id = $1))
		 RETURNING id, created_at, updated_at, revision`,
		c.ProjectID, c.WidthMM, c.HeightMM, c.Flight, c.StepHeightMM,
		c.StringerThicknessMM, c.StepThicknessMM, c.Riser, c.ClearanceMM, c.RailingHeightMM,
		c.ComfortStepMM, c.LandingWidthMM, c.LandingDepthMM, c.RoomWidthMM, c.RoomLengthMM,
		c.ApproachSpaceMM, c.LowerStepCount, c.OuterRadiusMM,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt, &c.Revision); err != nil {
		return fmt.Errorf("project: save config: %w", err)
	}
	return nil
}

func (r *ProjectRepository) GetLatestConfiguration(ctx context.Context, tenantID, projectID string) (*project.StairConfiguration, error) {
	c, err := scanConfig(r.pool.QueryRow(ctx,
		`SELECT `+configCols+` FROM stair_configurations
		 WHERE id = (SELECT current_configuration_id FROM projects
		              WHERE id = $1 AND tenant_id = $2)
		   AND project_id = $1`, projectID, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		// Текущая ревизия не задана — возвращаем последнюю сохранённую.
		c, err = scanConfig(r.pool.QueryRow(ctx,
			`SELECT `+configCols+` FROM stair_configurations
			 WHERE project_id = $1
			   AND project_id IN (SELECT id FROM projects WHERE tenant_id = $2 AND id = $1)
			 ORDER BY created_at DESC LIMIT 1`, projectID, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, project.ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("project: get latest config: %w", err)
		}
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("project: get current config: %w", err)
	}
	return c, nil
}

// ListConfigurations возвращает историю ревизий конфигурации проекта
// (EDR-0012) по возрастанию номера ревизии. Проект вне tenant — ErrNotFound.
func (r *ProjectRepository) ListConfigurations(ctx context.Context, tenantID, projectID string) ([]*project.StairConfiguration, error) {
	var exists string
	if err := r.pool.QueryRow(ctx,
		`SELECT id FROM projects WHERE id = $1 AND tenant_id = $2`, projectID, tenantID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return nil, project.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("project: check project: %w", err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+configCols+` FROM stair_configurations
		 WHERE project_id = $1 ORDER BY revision ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("project: list configs: %w", err)
	}
	defer rows.Close()
	var out []*project.StairConfiguration
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("project: list configs: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetConfigurationByID возвращает ревизию конфигурации по ID (EDR-0012).
func (r *ProjectRepository) GetConfigurationByID(ctx context.Context, tenantID, projectID, configurationID string) (*project.StairConfiguration, error) {
	c, err := scanConfig(r.pool.QueryRow(ctx,
		`SELECT `+configCols+` FROM stair_configurations
		 WHERE id = $1 AND project_id = $2
		   AND project_id IN (SELECT id FROM projects WHERE tenant_id = $3 AND id = $2)`,
		configurationID, projectID, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, project.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("project: get config: %w", err)
	}
	return c, nil
}

// HasConfigAccess проверяет членство пользователя в проекте, которому
// принадлежит конфигурация (S-132c). Член owner/editor/viewer имеет доступ;
// несуществующая конфигурация или отсутствие членства → false, без ошибки.
func (r *ProjectRepository) HasConfigAccess(ctx context.Context, userID, configurationID string) (bool, error) {
	var ok bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM stair_configurations sc
			JOIN project_members pm ON pm.project_id = sc.project_id
			WHERE sc.id = $1 AND pm.user_id = $2
		 )`,
		configurationID, userID).Scan(&ok); err != nil {
		return false, fmt.Errorf("project: has config access: %w", err)
	}
	return ok, nil
}

// RestoreConfiguration делает ревизию конфигурации текущей (EDR-0012).
func (r *ProjectRepository) RestoreConfiguration(ctx context.Context, tenantID, projectID, configurationID string) error {
	// Проверяем, что ревизия принадлежит проекту внутри tenant.
	var ok bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM stair_configurations
		   WHERE id = $1 AND project_id = $2
		     AND project_id IN (SELECT id FROM projects WHERE tenant_id = $3 AND id = $2))`,
		configurationID, projectID, tenantID).Scan(&ok); err != nil {
		return fmt.Errorf("project: check config: %w", err)
	}
	if !ok {
		return project.ErrNotFound
	}
	if _, err := r.pool.Exec(ctx,
		`UPDATE projects SET current_configuration_id = $1 WHERE id = $2 AND tenant_id = $3`,
		configurationID, projectID, tenantID); err != nil {
		return fmt.Errorf("project: restore config: %w", err)
	}
	return nil
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

	// Проверка принадлежности tenant'у + блокировка строки проекта в одной
	// транзакции (SEC-0005 + DB-001): FOR UPDATE сериализует конкурентные
	// сохранения, поэтому MAX(revision)+1 не может совпасть.
	var exists string
	if err := tx.QueryRow(ctx,
		`SELECT id FROM projects WHERE id = $1 AND tenant_id = $2 FOR UPDATE`,
		cfg.ProjectID, tenantID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return nil, project.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("project: check: %w", err)
	}

	if err := insertConfiguration(ctx, tx, cfg); err != nil {
		return nil, err
	}

	// Новая конфигурация становится текущей ревизией проекта (EDR-0012).
	if _, err := tx.Exec(ctx,
		`UPDATE projects SET current_configuration_id = $1 WHERE id = $2 AND tenant_id = $3`,
		cfg.ID, cfg.ProjectID, tenantID); err != nil {
		return nil, fmt.Errorf("project: set current config: %w", err)
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

func (r *ProjectRepository) AddComment(ctx context.Context, tenantID, projectID string, c *project.Comment) (*project.Comment, error) {
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO project_comments (project_id, author_id, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`, projectID, c.AuthorID, c.Body,
	).Scan(&c.ID, &c.CreatedAt); err != nil {
		return nil, fmt.Errorf("project: add comment: %w", err)
	}
	return c, nil
}

func (r *ProjectRepository) ListComments(ctx context.Context, tenantID, projectID string) ([]*project.Comment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT pc.id, pc.project_id, pc.author_id, pc.body, pc.created_at
		 FROM project_comments pc
		 WHERE pc.project_id = $1
		 ORDER BY pc.created_at`, projectID)
	if err != nil {
		return nil, fmt.Errorf("project: list comments: %w", err)
	}
	defer rows.Close()

	out := []*project.Comment{}
	for rows.Next() {
		var c project.Comment
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.AuthorID, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("project: scan comment: %w", err)
		}
		out = append(out, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("project: list comments: %w", err)
	}
	return out, nil
}

func (r *ProjectRepository) DeleteComment(ctx context.Context, tenantID, projectID, commentID, actorID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM project_comments pc
		 WHERE pc.id = $1 AND pc.project_id = $2
		   AND pc.project_id IN (SELECT id FROM projects WHERE tenant_id = $3)
		   AND (pc.author_id = $4
		        OR EXISTS (SELECT 1 FROM project_members pm
		                   WHERE pm.project_id = $2 AND pm.user_id = $4 AND pm.role = 'owner'))`,
		commentID, projectID, tenantID, actorID)
	if err != nil {
		return fmt.Errorf("project: delete comment: %w", err)
	}
	switch tag.RowsAffected() {
	case 0:
		return fmt.Errorf("project: comment not found or not deletable: %w", project.ErrNotFound)
	default:
		return nil
	}
}

// ---- review (EDR-0010) ----

const reviewCols = `id, project_id, requester_id, reviewer_id, decision, comment, created_at, decided_at`

// scanReview считывает строку ревью. reviewer_id может быть NULL (запрос
// ещё не решён) — в этом случае в сущность попадает пустая строка.
func scanReview(row pgx.Row) (*project.ProjectReview, error) {
	var rv project.ProjectReview
	var reviewer *string
	if err := row.Scan(&rv.ID, &rv.ProjectID, &rv.RequesterID, &reviewer,
		&rv.Decision, &rv.Comment, &rv.CreatedAt, &rv.DecidedAt); err != nil {
		return nil, err
	}
	if reviewer != nil {
		rv.ReviewerID = *reviewer
	}
	return &rv, nil
}

// RequestReview создаёт запрос ревью (EDR-0010): в одной транзакции
// переводит проект draft|changes_requested → in_review и добавляет строку
// ревью (decision=requested). Недопустимый статус → ErrConflict; проект вне
// tenant → ErrNotFound.
func (r *ProjectRepository) RequestReview(ctx context.Context, tenantID, projectID, requesterID, comment string) (*project.ProjectReview, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("project: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var cur string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM projects WHERE id = $1 AND tenant_id = $2 FOR UPDATE`,
		projectID, tenantID).Scan(&cur); errors.Is(err, pgx.ErrNoRows) {
		return nil, project.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("project: review status lock: %w", err)
	}
	if cur != project.StatusDraft && cur != project.StatusChangesRequested {
		return nil, fmt.Errorf("project: review from status %s: %w", cur, project.ErrConflict)
	}

	rv, err := scanReview(tx.QueryRow(ctx,
		`INSERT INTO project_reviews (project_id, requester_id, decision, comment)
		 VALUES ($1, $2, 'requested', $3)
		 RETURNING `+reviewCols,
		projectID, requesterID, comment))
	if err != nil {
		return nil, fmt.Errorf("project: insert review: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE projects SET status = $2, updated_at = now() WHERE id = $1`,
		projectID, project.StatusInReview); err != nil {
		return nil, fmt.Errorf("project: set in_review: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("project: commit: %w", err)
	}
	return rv, nil
}

// DecideReview завершает ревью (EDR-0010): в одной транзакции переводит
// проект in_review → approved|changes_requested и фиксирует решение в
// строке ревью (reviewer_id, decided_at). Только owner (проверка в service);
// автор запроса ревью не может решать (self-approve запрещён) →
// ErrForbidden; ревью не в статусе pending или проект вне tenant →
// ErrNotFound; недопустимый статус проекта → ErrConflict.
func (r *ProjectRepository) DecideReview(ctx context.Context, tenantID, projectID, reviewID, reviewerID, decision, comment string) (*project.ProjectReview, error) {
	if decision != project.ReviewApproved && decision != project.ReviewChangesRequest {
		return nil, fmt.Errorf("project: unknown review decision %q", decision)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("project: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rv, err := scanReview(tx.QueryRow(ctx,
		`SELECT `+reviewCols+` FROM project_reviews rv
		 WHERE rv.id = $1 AND rv.project_id = $2
		   AND rv.decision = 'requested' AND rv.decided_at IS NULL
		 FOR UPDATE`,
		reviewID, projectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("project: review not pending: %w", project.ErrNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("project: decide review lock: %w", err)
	}

	if rv.RequesterID == reviewerID {
		return nil, fmt.Errorf("project: self-approve forbidden: %w", project.ErrForbidden)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE project_reviews
		 SET reviewer_id = $3, decided_at = now(), decision = $5, comment = $4
		 WHERE id = $1 AND project_id = $2`,
		reviewID, projectID, reviewerID, comment, decision); err != nil {
		return nil, fmt.Errorf("project: decide review: %w", err)
	}
	rv.ReviewerID = reviewerID
	rv.Comment = comment
	now := time.Now()
	rv.DecidedAt = &now

	var next string
	if decision == project.ReviewApproved {
		next = project.StatusApproved
	} else {
		next = project.StatusChangesRequested
	}
	tag, err := tx.Exec(ctx,
		`UPDATE projects SET status = $2, updated_at = now()
		 WHERE id = $1 AND status = $3 AND tenant_id = $4`,
		projectID, next, project.StatusInReview, tenantID)
	if err != nil {
		return nil, fmt.Errorf("project: decide status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("project: decide from invalid status: %w", project.ErrConflict)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("project: commit: %w", err)
	}
	rv.Decision = decision
	return rv, nil
}

func (r *ProjectRepository) ListReviews(ctx context.Context, tenantID, projectID string) ([]*project.ProjectReview, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+reviewCols+` FROM project_reviews rv
		 WHERE rv.project_id = $1
		   AND rv.project_id IN (SELECT id FROM projects WHERE tenant_id = $2 AND id = $1)
		 ORDER BY rv.created_at`, projectID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("project: list reviews: %w", err)
	}
	defer rows.Close()

	out := []*project.ProjectReview{}
	for rows.Next() {
		rv, err := scanReview(rows)
		if err != nil {
			return nil, fmt.Errorf("project: scan review: %w", err)
		}
		out = append(out, rv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("project: list reviews: %w", err)
	}
	return out, nil
}

// ---- configuration approval (EDR-0011) ----

const approvalCols = `id, project_id, configuration_id, approved_by, comment, created_at`

// isUniqueViolation — признак нарушения UNIQUE-констрейнта PostgreSQL.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" // unique_violation
}

func (r *ProjectRepository) ApproveConfiguration(ctx context.Context, tenantID, projectID, configurationID, approvedByID, comment string) (*project.ConfigurationApproval, error) {
	// Конфигурация должна принадлежать проекту внутри tenant (SEC-0005).
	var cfgProject string
	if err := r.pool.QueryRow(ctx,
		`SELECT project_id FROM stair_configurations sc
		 WHERE sc.id = $1 AND sc.project_id = $2
		   AND sc.project_id IN (SELECT id FROM projects WHERE tenant_id = $3)`,
		configurationID, projectID, tenantID).Scan(&cfgProject); errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("project: approval config not found: %w", project.ErrNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("project: approval config check: %w", err)
	}

	var a project.ConfigurationApproval
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO configuration_approvals (project_id, configuration_id, approved_by, comment)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+approvalCols,
		projectID, configurationID, approvedByID, comment).Scan(
		&a.ID, &a.ProjectID, &a.ConfigurationID, &a.ApprovedByID, &a.Comment, &a.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("project: configuration already approved: %w", project.ErrConflict)
		}
		return nil, fmt.Errorf("project: approve configuration: %w", err)
	}
	return &a, nil
}

func (r *ProjectRepository) GetConfigurationApproval(ctx context.Context, tenantID, projectID, configurationID string) (*project.ConfigurationApproval, error) {
	var a project.ConfigurationApproval
	if err := r.pool.QueryRow(ctx,
		`SELECT `+approvalCols+` FROM configuration_approvals ca
		 WHERE ca.configuration_id = $1 AND ca.project_id = $2
		   AND ca.project_id IN (SELECT id FROM projects WHERE tenant_id = $3)`,
		configurationID, projectID, tenantID).Scan(
		&a.ID, &a.ProjectID, &a.ConfigurationID, &a.ApprovedByID, &a.Comment, &a.CreatedAt); errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("project: approval not found: %w", project.ErrNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("project: get approval: %w", err)
	}
	return &a, nil
}

func (r *ProjectRepository) ListApprovals(ctx context.Context, tenantID, projectID string) ([]*project.ConfigurationApproval, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+approvalCols+` FROM configuration_approvals ca
		 WHERE ca.project_id = $1
		   AND ca.project_id IN (SELECT id FROM projects WHERE tenant_id = $2 AND id = $1)
		 ORDER BY ca.created_at`, projectID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("project: list approvals: %w", err)
	}
	defer rows.Close()

	out := []*project.ConfigurationApproval{}
	for rows.Next() {
		var a project.ConfigurationApproval
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.ConfigurationID, &a.ApprovedByID, &a.Comment, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("project: scan approval: %w", err)
		}
		out = append(out, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("project: list approvals: %w", err)
	}
	return out, nil
}
