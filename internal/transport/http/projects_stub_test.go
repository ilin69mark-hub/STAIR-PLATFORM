package http

import (
	"context"
	"strconv"

	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
	kerngeo "stairplatform/internal/geometry"
)

// sec1Projects — минимальная заглушка ProjectService для тестов пагинации
// (API-001) и общих HTTP-проверок. Списки возвращаются заполненными, чтобы
// нарезка в памяти реально выполнялась.
//
// Заглушка нужна потому, что обработчики /projects зарегистрированы всегда
// (NewRouter регистрирует их вне `if projects != nil`), и при nil-сервисе
// handleListProjects паникует — это отдельная находка (см. AUDIT INF:
// маршруты, зарегистрированные вне своего nil-guard'а).
type sec1Projects struct{ n int }

func (p *sec1Projects) mk(i int) *project.Project {
	return &project.Project{ID: "p" + strconv.Itoa(i), Name: "project " + strconv.Itoa(i), Status: project.StatusDraft}
}

func (p *sec1Projects) ListProjects(context.Context, string, string) ([]*project.Project, error) {
	out := make([]*project.Project, 0, p.n)
	for i := 0; i < p.n; i++ {
		out = append(out, p.mk(i))
	}
	return out, nil
}

func (p *sec1Projects) ListMembers(context.Context, string, string, string) ([]*project.ProjectMember, error) {
	return nil, nil
}
func (p *sec1Projects) ListComments(context.Context, string, string, string) ([]*project.Comment, error) {
	return nil, nil
}
func (p *sec1Projects) ListReviews(context.Context, string, string, string) ([]*project.ProjectReview, error) {
	return nil, nil
}
func (p *sec1Projects) ListApprovals(context.Context, string, string, string) ([]*project.ConfigurationApproval, error) {
	return nil, nil
}
func (p *sec1Projects) ListTenantProjects(context.Context, string) ([]*project.Project, error) {
	return p.ListProjects(context.Background(), "", "")
}

func (p *sec1Projects) CreateProject(context.Context, string, string, string, string) (*project.Project, error) {
	return p.mk(0), nil
}
func (p *sec1Projects) GetProject(_ context.Context, _, _, id string) (*project.Project, error) {
	if id == "" {
		return nil, project.ErrNotFound
	}
	return &project.Project{ID: id, Status: project.StatusDraft}, nil
}
func (p *sec1Projects) AddMember(context.Context, string, string, string, string, project.ProjectRole) error {
	return nil
}
func (p *sec1Projects) AddMemberByEmail(context.Context, string, string, string, string, project.ProjectRole) error {
	return nil
}
func (p *sec1Projects) UpdateMemberRole(context.Context, string, string, string, string, project.ProjectRole) error {
	return nil
}
func (p *sec1Projects) RemoveMember(context.Context, string, string, string, string) error {
	return nil
}
func (p *sec1Projects) AddComment(context.Context, string, string, string, string) (*project.Comment, error) {
	return &project.Comment{ID: "c1"}, nil
}
func (p *sec1Projects) DeleteComment(context.Context, string, string, string, string) error {
	return nil
}
func (p *sec1Projects) RequestReview(context.Context, string, string, string, string) (*project.ProjectReview, error) {
	return &project.ProjectReview{ID: "r1"}, nil
}
func (p *sec1Projects) SignOffReview(context.Context, string, string, string, string, string) (*project.ProjectReview, error) {
	return &project.ProjectReview{ID: "r1"}, nil
}
func (p *sec1Projects) RequestChanges(context.Context, string, string, string, string, string) (*project.ProjectReview, error) {
	return &project.ProjectReview{ID: "r1"}, nil
}
func (p *sec1Projects) ApproveConfiguration(context.Context, string, string, string, string, string) (*project.ConfigurationApproval, error) {
	return &project.ConfigurationApproval{ID: "a1"}, nil
}
func (p *sec1Projects) GetConfigurationApproval(context.Context, string, string, string, string) (*project.ConfigurationApproval, error) {
	return &project.ConfigurationApproval{ID: "a1"}, nil
}
func (p *sec1Projects) Calculate(context.Context, string, string, string, stair.Config, stair.Options) (*project.Calculation, error) {
	return &project.Calculation{ID: "calc1"}, nil
}
func (p *sec1Projects) Preview(context.Context, string, string, string, stair.Config, stair.Options) (*project.Snapshot, error) {
	return &project.Snapshot{ProjectID: "p1"}, nil
}
func (p *sec1Projects) Optimize(context.Context, string, string, string, stair.Config, stair.Options, stair.OptimizeRequest) (*project.OptimizeOutcome, error) {
	return &project.OptimizeOutcome{}, nil
}
func (p *sec1Projects) GetResult(context.Context, string, string, string) (*project.Calculation, error) {
	return &project.Calculation{ID: "calc1", Result: []byte(`{}`)}, nil
}

// ExportCADWithRailings — экспорт с перилами (DOM-003).
func (p *sec1Projects) ExportCADWithRailings(ctx context.Context, tenantID, userID, projectID string) (*kerngeo.Mesh, *kerngeo.Mesh, error) {
	m, err := p.ExportCAD(ctx, tenantID, userID, projectID)
	return m, &kerngeo.Mesh{Vertices: []kerngeo.Point3{}, Triangles: [][3]int{}}, err
}

func (p *sec1Projects) ExportCAD(context.Context, string, string, string) (*kerngeo.Mesh, error) {
	return &kerngeo.Mesh{}, nil
}
func (p *sec1Projects) ListConfigurations(context.Context, string, string, string) ([]*project.StairConfiguration, error) {
	return nil, nil
}
func (p *sec1Projects) GetConfiguration(context.Context, string, string, string, string) (*project.StairConfiguration, error) {
	return &project.StairConfiguration{ID: "cfg1"}, nil
}
func (p *sec1Projects) RestoreConfiguration(context.Context, string, string, string, string) (*project.StairConfiguration, error) {
	return &project.StairConfiguration{ID: "cfg1"}, nil
}
