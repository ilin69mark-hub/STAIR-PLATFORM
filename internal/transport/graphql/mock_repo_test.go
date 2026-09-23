package graphql

import (
	"context"

	"stairplatform/internal/application/project"
)

// MockProjectRepository — мок для тестирования.
type MockProjectRepository struct{}

func (m *MockProjectRepository) CreateProject(ctx context.Context, tenantID, ownerID string, p *project.Project) error {
	return nil
}

func (m *MockProjectRepository) GetProject(ctx context.Context, tenantID, userID, id string) (*project.Project, error) {
	return &project.Project{ID: id, Name: "Test Project"}, nil
}

func (m *MockProjectRepository) ListProjects(ctx context.Context, tenantID, userID string) ([]*project.Project, error) {
	return []*project.Project{}, nil
}

func (m *MockProjectRepository) ListTenantProjects(ctx context.Context, tenantID string) ([]*project.Project, error) {
	return []*project.Project{}, nil
}

func (m *MockProjectRepository) GetMember(ctx context.Context, tenantID, projectID, userID string) (*project.ProjectMember, error) {
	return nil, project.ErrNotFound
}

func (m *MockProjectRepository) ListMembers(ctx context.Context, tenantID, projectID string) ([]*project.ProjectMember, error) {
	return []*project.ProjectMember{}, nil
}

func (m *MockProjectRepository) AddMember(ctx context.Context, tenantID, projectID string, pm *project.ProjectMember) error {
	return nil
}

func (m *MockProjectRepository) AddMemberByEmail(ctx context.Context, tenantID, projectID, email string, role project.ProjectRole) error {
	return nil
}

func (m *MockProjectRepository) UpdateMemberRole(ctx context.Context, tenantID, projectID, userID string, role project.ProjectRole) error {
	return nil
}

func (m *MockProjectRepository) RemoveMember(ctx context.Context, tenantID, projectID, userID string) error {
	return nil
}

func (m *MockProjectRepository) AddComment(ctx context.Context, tenantID, projectID string, c *project.Comment) (*project.Comment, error) {
	return c, nil
}

func (m *MockProjectRepository) ListComments(ctx context.Context, tenantID, projectID string) ([]*project.Comment, error) {
	return []*project.Comment{}, nil
}

func (m *MockProjectRepository) DeleteComment(ctx context.Context, tenantID, projectID, commentID, actorID string) error {
	return nil
}

func (m *MockProjectRepository) RequestReview(ctx context.Context, tenantID, projectID, requesterID, comment string) (*project.ProjectReview, error) {
	return &project.ProjectReview{}, nil
}

func (m *MockProjectRepository) DecideReview(ctx context.Context, tenantID, projectID, reviewID, reviewerID, decision, comment string) (*project.ProjectReview, error) {
	return &project.ProjectReview{}, nil
}

func (m *MockProjectRepository) ListReviews(ctx context.Context, tenantID, projectID string) ([]*project.ProjectReview, error) {
	return []*project.ProjectReview{}, nil
}

func (m *MockProjectRepository) ApproveConfiguration(ctx context.Context, tenantID, projectID, configurationID, approvedByID, comment string) (*project.ConfigurationApproval, error) {
	return &project.ConfigurationApproval{}, nil
}

func (m *MockProjectRepository) GetConfigurationApproval(ctx context.Context, tenantID, projectID, configurationID string) (*project.ConfigurationApproval, error) {
	return nil, project.ErrNotFound
}

func (m *MockProjectRepository) ListApprovals(ctx context.Context, tenantID, projectID string) ([]*project.ConfigurationApproval, error) {
	return []*project.ConfigurationApproval{}, nil
}

func (m *MockProjectRepository) SaveConfiguration(ctx context.Context, c *project.StairConfiguration) error {
	return nil
}

func (m *MockProjectRepository) GetLatestConfiguration(ctx context.Context, tenantID, projectID string) (*project.StairConfiguration, error) {
	return nil, project.ErrNotFound
}

func (m *MockProjectRepository) ListConfigurations(ctx context.Context, tenantID, projectID string) ([]*project.StairConfiguration, error) {
	return []*project.StairConfiguration{}, nil
}

func (m *MockProjectRepository) GetConfigurationByID(ctx context.Context, tenantID, projectID, configurationID string) (*project.StairConfiguration, error) {
	return nil, project.ErrNotFound
}

func (m *MockProjectRepository) RestoreConfiguration(ctx context.Context, tenantID, projectID, configurationID string) error {
	return nil
}

func (m *MockProjectRepository) SaveCalculationWithConfig(ctx context.Context, tenantID string, cfg *project.StairConfiguration, snap project.Snapshot) (*project.Calculation, error) {
	return &project.Calculation{}, nil
}

func (m *MockProjectRepository) GetLatestCalculation(ctx context.Context, tenantID, projectID string) (*project.Calculation, error) {
	return nil, project.ErrNotFound
}

func (m *MockProjectRepository) HasConfigAccess(ctx context.Context, userID, configurationID string) (bool, error) {
	return false, nil
}
