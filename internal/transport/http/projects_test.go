package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/auth"
	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
	cadexp "stairplatform/internal/infrastructure/cad"
)

// fakeProjectService — тестовая реализация ProjectService.
type fakeProjectService struct {
	projects         map[string]*project.Project
	members          []*project.ProjectMember
	comments         []*project.Comment
	reviews          []*project.ProjectReview
	approvals        []*project.ConfigurationApproval
	configs          []*project.StairConfiguration
	calc             *project.Calculation
	cadMesh          *kerngeo.Mesh
	createErr        error
	calculateErr     error
	getErr           error
	reviewErr        error
	listTenantErr    error
	getProjectErr    error // GetProject → произвольная ошибка
	getForbidden     bool  // GetProject → ErrForbidden
	listErr          error // ListProjects → ошибка
	membersListErr   error // ListMembers → ошибка
	membershipErr    error // Add/Update/Remove member → ошибка
	addCommentErr    error // AddComment → ошибка
	commentsListErr  error // ListComments → ошибка
	deleteCommentErr error // DeleteComment → ошибка
}

func newFakeProjectService() *fakeProjectService {
	return &fakeProjectService{projects: map[string]*project.Project{}}
}

func (f *fakeProjectService) CreateProject(ctx context.Context, tenantID, ownerID, name, description string) (*project.Project, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	p := &project.Project{ID: "p-1", Name: name, Description: description,
		Status: "draft", OwnerID: ownerID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.projects[p.ID] = p
	return p, nil
}

func (f *fakeProjectService) GetProject(ctx context.Context, tenantID, userID, id string) (*project.Project, error) {
	if f.getProjectErr != nil {
		return nil, f.getProjectErr
	}
	if f.getForbidden {
		return nil, project.ErrForbidden
	}
	p, ok := f.projects[id]
	if !ok {
		return nil, project.ErrNotFound
	}
	return p, nil
}

func (f *fakeProjectService) ListProjects(ctx context.Context, tenantID, userID string) ([]*project.Project, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]*project.Project, 0, len(f.projects))
	for _, p := range f.projects {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeProjectService) ListTenantProjects(ctx context.Context, tenantID string) ([]*project.Project, error) {
	if f.listTenantErr != nil {
		return nil, f.listTenantErr
	}
	out := make([]*project.Project, 0, len(f.projects))
	for _, p := range f.projects {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeProjectService) ListMembers(ctx context.Context, tenantID, userID, projectID string) ([]*project.ProjectMember, error) {
	if f.membersListErr != nil {
		return nil, f.membersListErr
	}
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	return f.members, nil
}

func (f *fakeProjectService) AddMember(ctx context.Context, tenantID, actorID, projectID, userID string, role project.ProjectRole) error {
	if f.membershipErr != nil {
		return f.membershipErr
	}
	return nil
}

func (f *fakeProjectService) AddMemberByEmail(ctx context.Context, tenantID, actorID, projectID, email string, role project.ProjectRole) error {
	if f.membershipErr != nil {
		return f.membershipErr
	}
	return nil
}

func (f *fakeProjectService) UpdateMemberRole(ctx context.Context, tenantID, actorID, projectID, userID string, role project.ProjectRole) error {
	if f.membershipErr != nil {
		return f.membershipErr
	}
	return nil
}

func (f *fakeProjectService) RemoveMember(ctx context.Context, tenantID, actorID, projectID, userID string) error {
	if f.membershipErr != nil {
		return f.membershipErr
	}
	return nil
}

func (f *fakeProjectService) AddComment(ctx context.Context, tenantID, userID, projectID, body string) (*project.Comment, error) {
	if f.addCommentErr != nil {
		return nil, f.addCommentErr
	}
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	return &project.Comment{ID: "c-1", ProjectID: projectID, AuthorID: userID, Body: body, CreatedAt: time.Now()}, nil
}

func (f *fakeProjectService) ListComments(ctx context.Context, tenantID, userID, projectID string) ([]*project.Comment, error) {
	if f.commentsListErr != nil {
		return nil, f.commentsListErr
	}
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	return f.comments, nil
}

func (f *fakeProjectService) DeleteComment(ctx context.Context, tenantID, userID, projectID, commentID string) error {
	if _, ok := f.projects[projectID]; !ok {
		return project.ErrNotFound
	}
	if f.deleteCommentErr != nil {
		return f.deleteCommentErr
	}
	return nil
}

func (f *fakeProjectService) RequestReview(ctx context.Context, tenantID, userID, projectID, comment string) (*project.ProjectReview, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.reviewErr != nil {
		return nil, f.reviewErr
	}
	rv := &project.ProjectReview{ID: "rv-1", ProjectID: projectID, RequesterID: userID,
		Decision: project.ReviewRequested, Comment: comment, CreatedAt: time.Now()}
	rv2 := *rv
	f.reviews = append(f.reviews, &rv2)
	return &rv2, nil
}

func (f *fakeProjectService) SignOffReview(ctx context.Context, tenantID, userID, projectID, reviewID, comment string) (*project.ProjectReview, error) {
	if f.reviewErr != nil {
		return nil, f.reviewErr
	}
	return &project.ProjectReview{ID: reviewID, ProjectID: projectID, RequesterID: "u-request",
		ReviewerID: userID, Decision: project.ReviewApproved, Comment: comment,
		CreatedAt: time.Now(), DecidedAt: &time.Time{}}, nil
}

func (f *fakeProjectService) RequestChanges(ctx context.Context, tenantID, userID, projectID, reviewID, comment string) (*project.ProjectReview, error) {
	if f.reviewErr != nil {
		return nil, f.reviewErr
	}
	return &project.ProjectReview{ID: reviewID, ProjectID: projectID, RequesterID: "u-request",
		ReviewerID: userID, Decision: project.ReviewChangesRequest, Comment: comment,
		CreatedAt: time.Now(), DecidedAt: &time.Time{}}, nil
}

func (f *fakeProjectService) ListReviews(ctx context.Context, tenantID, userID, projectID string) ([]*project.ProjectReview, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	return f.reviews, nil
}

func (f *fakeProjectService) ApproveConfiguration(ctx context.Context, tenantID, userID, projectID, configurationID, comment string) (*project.ConfigurationApproval, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.reviewErr != nil {
		return nil, f.reviewErr
	}
	a := &project.ConfigurationApproval{ID: "a-1", ProjectID: projectID, ConfigurationID: configurationID,
		ApprovedByID: userID, Comment: comment, CreatedAt: time.Now()}
	a2 := *a
	f.approvals = append(f.approvals, &a2)
	return &a2, nil
}

func (f *fakeProjectService) GetConfigurationApproval(ctx context.Context, tenantID, userID, projectID, configurationID string) (*project.ConfigurationApproval, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.reviewErr != nil {
		return nil, f.reviewErr
	}
	if len(f.approvals) == 0 {
		return nil, project.ErrNotFound
	}
	return f.approvals[len(f.approvals)-1], nil
}

func (f *fakeProjectService) ListApprovals(ctx context.Context, tenantID, userID, projectID string) ([]*project.ConfigurationApproval, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	return f.approvals, nil
}

func (f *fakeProjectService) Calculate(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options) (*project.Calculation, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.calculateErr != nil {
		return nil, f.calculateErr
	}
	return &project.Calculation{
		ID: "c-1", ProjectID: projectID, ConfigurationID: "cfg-1",
		Valid: true, Result: []byte(`{"project_id":"` + projectID + `"}`),
		CreatedAt: time.Now(),
	}, nil
}

func (f *fakeProjectService) Optimize(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options, oreq stair.OptimizeRequest) (*project.OptimizeOutcome, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.calculateErr != nil {
		return nil, f.calculateErr
	}
	out := &stair.OptimizeResult{
		Valid:      true,
		Target:     oreq.Target,
		Objective:  1000,
		BestConfig: cfg,
		BestResult: &stair.Result{
			StepThickness: engineering.Length(40),
			Riser:         true,
		},
	}
	snap := project.NewSnapshot(projectID, out.BestResult)
	raw, err := json.Marshal(snap)
	if err != nil {
		panic(err)
	}
	calc := &project.Calculation{
		ID: "c-opt", ProjectID: projectID, ConfigurationID: "cfg-opt",
		Valid: true, Result: raw,
		CreatedAt: time.Now(),
	}
	return &project.OptimizeOutcome{Result: out, Calculation: calc}, nil
}

func (f *fakeProjectService) GetResult(ctx context.Context, tenantID, userID, projectID string) (*project.Calculation, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.calc == nil {
		return nil, project.ErrNotFound
	}
	return f.calc, nil
}

func (f *fakeProjectService) Preview(ctx context.Context, tenantID, userID, projectID string, cfg stair.Config, opts stair.Options) (*project.Snapshot, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.calculateErr != nil {
		return nil, f.calculateErr
	}
	snap := project.NewSnapshot(projectID, &stair.Result{})
	return &snap, nil
}

func (f *fakeProjectService) ExportCAD(ctx context.Context, tenantID, userID, projectID string) (*kerngeo.Mesh, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.cadMesh == nil {
		return nil, project.ErrNotFound
	}
	return f.cadMesh, nil
}

func (f *fakeProjectService) ListConfigurations(ctx context.Context, tenantID, userID, projectID string) ([]*project.StairConfiguration, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.reviewErr != nil {
		return nil, f.reviewErr
	}
	return f.configs, nil
}

func (f *fakeProjectService) GetConfiguration(ctx context.Context, tenantID, userID, projectID, configurationID string) (*project.StairConfiguration, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	for _, c := range f.configs {
		if c.ID == configurationID {
			return c, nil
		}
	}
	return nil, project.ErrNotFound
}

func (f *fakeProjectService) RestoreConfiguration(ctx context.Context, tenantID, userID, projectID, configurationID string) (*project.StairConfiguration, error) {
	if _, ok := f.projects[projectID]; !ok {
		return nil, project.ErrNotFound
	}
	if f.reviewErr != nil {
		return nil, f.reviewErr
	}
	for _, c := range f.configs {
		if c.ID == configurationID {
			return c, nil
		}
	}
	return nil, project.ErrNotFound
}

// testAuth — фиктивный AuthService для тестов: любой токен валиден,
// пользователь принадлежит tenant'у "t-1".
type testAuth struct{}

func (testAuth) Register(ctx context.Context, email, name, password string) (*auth.User, string, error) {
	return &auth.User{ID: "u-1", Email: email, Name: name, Role: auth.RoleUser, TenantID: "t-1"}, "token-1", nil
}

func (testAuth) Login(ctx context.Context, email, password string) (*auth.User, string, error) {
	return &auth.User{ID: "u-1", Email: email, Role: auth.RoleUser, TenantID: "t-1"}, "token-1", nil
}

func (testAuth) Authenticate(ctx context.Context, token string) (*auth.User, string, error) {
	if token == "" {
		return nil, "", auth.ErrSessionExpired
	}
	return &auth.User{ID: "u-1", Email: "test@example.com", Role: auth.RoleUser, TenantID: "t-1"}, "", nil
}

func (testAuth) Logout(ctx context.Context, token string) error { return nil }

func (testAuth) ListUsers(ctx context.Context, tenantID string) ([]*auth.User, error) {
	return []*auth.User{{ID: "u-2", TenantID: tenantID, Email: "member@example.com", Role: auth.RoleUser}}, nil
}

func (testAuth) UpdateUserRole(ctx context.Context, tenantID, actorID, userID string, role auth.Role) error {
	return nil
}

func (testAuth) UpdateUser(ctx context.Context, tenantID, actorID, userID string, role *auth.Role, status *auth.Status) error {
	return nil
}

func (testAuth) GetPolicy(ctx context.Context, tenantID string) (auth.Policy, error) {
	return auth.DefaultPolicy(), nil
}

func (testAuth) UpdatePolicy(ctx context.Context, tenantID, actorID string, p auth.Policy) error {
	return nil
}

func (testAuth) AuthenticateApiKey(ctx context.Context, token string) (*auth.ApiKey, error) {
	if token == "" {
		return nil, auth.ErrSessionExpired
	}
	return &auth.ApiKey{TenantID: "t-1", Scopes: []string{string(auth.PermissionUsersList)}}, nil
}

func (testAuth) CreateApiKey(ctx context.Context, tenantID, actorID, name string, scopes []auth.Permission) (*auth.ApiKey, string, error) {
	sc := make([]string, 0, len(scopes))
	for _, s := range scopes {
		sc = append(sc, string(s))
	}
	return &auth.ApiKey{ID: "key-1", TenantID: tenantID, Name: name, Scopes: sc}, "secret-token", nil
}

func (testAuth) ListApiKeys(ctx context.Context, tenantID string) ([]*auth.ApiKey, error) {
	return nil, nil
}

func (testAuth) RevokeApiKey(ctx context.Context, tenantID, actorID, keyID string) error {
	return nil
}

func (testAuth) SsoAuthorizeURL(_ context.Context, _ string) (string, error) {
	return "https://idp.example/authorize", nil
}

func (testAuth) SsoCallback(_ context.Context, code, state string) (*auth.User, string, error) {
	return &auth.User{ID: "u-1", Email: "sso@example.com", Role: auth.RoleUser, TenantID: "t-1"}, "token-1", nil
}

func (testAuth) SsoEnabled() auth.SsoConfig { return auth.SsoConfig{} }

func (testAuth) DefaultTenant(_ context.Context) (*auth.Tenant, error) {
	return &auth.Tenant{ID: "t-1", Slug: "default"}, nil
}

func testRouterWithProjects(p ProjectService) http.Handler {
	return NewRouter(stair.NewService(), p, testAuth{}, DefaultConfig())
}

// authedRequest строит запрос с session+csrf cookie и заголовком CSRF
// (S-144: Content-Type application/json — decodeJSON требует его для тел).
func authedRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.AddCookie(testCookie(sessionCookieName, "token-1"))
	r.AddCookie(testCookie(csrfCookieName, "csrf-1"))
	r.Header.Set(csrfHeader, "csrf-1")
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestCreateProject(t *testing.T) {
	svc := newFakeProjectService()
	body := `{"name": "Лестница", "description": "описание"}`
	req := authedRequest(http.MethodPost, "/api/v1/projects", body)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var p projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Name != "Лестница" || p.Description != "описание" || p.Status != "draft" {
		t.Fatalf("unexpected project: %+v", p)
	}
}

func TestCreateProjectInvalidJSON(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodPost, "/api/v1/projects", "{bad")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetProject(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-42"] = &project.Project{ID: "p-42", Name: "А", Status: "draft"}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-42", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var p projectDTO
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Name != "А" {
		t.Fatalf("name = %q", p.Name)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-404", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListProjects(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	req := authedRequest(http.MethodGet, "/api/v1/projects", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp PaginatedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	list, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be array, got %T", resp.Data)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
}

func TestCalculateProject(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/calculate",
		referenceJSON)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var c calculationDTO
	if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.CalculationID != "c-1" || c.ConfigurationID != "cfg-1" || !c.Valid {
		t.Fatalf("unexpected calculation: %+v", c)
	}
	if len(c.Result) == 0 {
		t.Fatal("expected result snapshot")
	}
}

func TestCalculateProjectNotFound(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-404/calculate",
		referenceJSON)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCalculateProjectInvalidJSON(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/calculate",
		"{bad")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestOptimizeProject(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	body := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000,
		"target": "cost"
	}`
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/optimize", body)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp projectOptimizeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Valid || !resp.Saved {
		t.Fatalf("expected valid+saved, got %+v", resp)
	}
	if resp.CalculationID != "c-opt" || resp.ConfigurationID != "cfg-opt" {
		t.Fatalf("unexpected saved refs: %+v", resp)
	}
	if resp.Target != "cost" {
		t.Fatalf("target = %q, want cost", resp.Target)
	}
	if resp.Best == nil {
		t.Fatal("expected best candidate")
	}
	// best.result должен быть Snapshot-формой (camelCase: measurement,
	// step_thickness), а не calculateResponse: фронтенд читает project.Snapshot
	// (см. handleOptimize в ProjectDetail.tsx и types.ts Snapshot).
	var snap project.Snapshot
	if err := json.Unmarshal(resp.Best.Result, &snap); err != nil {
		t.Fatalf("best.result is not a project.Snapshot: %v", err)
	}
	if snap.StepThickness != 40 || !snap.Riser {
		t.Fatalf("best.result snapshot lost echo params: %+v", snap)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(resp.Best.Result, &raw); err != nil {
		t.Fatalf("best.result invalid JSON: %v", err)
	}
	if _, ok := raw["measurement"]; !ok {
		t.Fatalf("best.result must use measurement key (not geometry), got keys: %v", keysOf(raw))
	}
}

// keysOf возвращает отсортированные ключи JSON-объекта (для диагностики).
func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestOptimizeProjectNotFound(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-404/optimize",
		referenceJSON)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestOptimizeProjectForbidden(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.calculateErr = project.ErrForbidden
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/optimize",
		referenceJSON)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestOptimizeProjectInvalidJSON(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/optimize",
		"{bad")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestExportProject(t *testing.T) {
	svc := newFakeProjectService()
	svc.calc = &project.Calculation{
		ID: "c-1", ProjectID: "p-1",
		Result: []byte(`{"project_id":"p-1","validation":{"valid":true}}`),
	}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("export must be valid JSON: %v", err)
	}
	if doc["project_id"] != "p-1" {
		t.Fatalf("project_id = %v", doc["project_id"])
	}
}

func TestExportProjectNoCalculation(t *testing.T) {
	svc := newFakeProjectService()
	svc.getErr = errors.New("boom")
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// ---- CAD export (EDR-0022) ----

func cadTestMesh() *kerngeo.Mesh {
	return &kerngeo.Mesh{
		Vertices: []kerngeo.Point3{
			kerngeo.NewPoint3(0, 0, 0), kerngeo.NewPoint3(10, 0, 0), kerngeo.NewPoint3(10, 10, 0),
		},
		Triangles: [][3]int{{0, 1, 2}},
	}
}

func TestExportCAD(t *testing.T) {
	for _, format := range []cadexp.Format{cadexp.DXF, cadexp.STL, cadexp.SVG} {
		t.Run(string(format), func(t *testing.T) {
			svc := newFakeProjectService()
			svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
			svc.cadMesh = cadTestMesh()

			req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export/cad?format="+string(format), "")
			rec := httptest.NewRecorder()
			testRouterWithProjects(svc).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != format.MIME() {
				t.Errorf("Content-Type = %q, want %q", ct, format.MIME())
			}
			if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, format.Extension()) {
				t.Errorf("Content-Disposition = %q, want %q in it", cd, format.Extension())
			}
			if rec.Body.Len() == 0 {
				t.Errorf("empty body")
			}
		})
	}
}

func TestExportCADBadFormat(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.cadMesh = cadTestMesh()

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export/cad?format=obj", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestExportCADNoMesh(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.cadMesh = nil

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export/cad?format=dxf", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestListMembers — GET /members возвращает роли участников.
func TestListMembers(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.members = []*project.ProjectMember{
		{ProjectID: "p-1", UserID: "u-1", Role: project.RoleOwner, CreatedAt: time.Now()},
		{ProjectID: "p-1", UserID: "u-2", Role: project.RoleViewer, CreatedAt: time.Now()},
	}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/members", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []memberDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 2 || list[0].Role != "owner" || list[1].Role != "viewer" {
		t.Fatalf("unexpected members: %+v", list)
	}
}

// TestListMembersForbidden — не-член проекта получает 404.
func TestListMembersForbidden(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-404/members", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestAddMemberValid — owner добавляет участника (201).
func TestAddMemberValid(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	body := `{"user_id": "u-2", "role": "editor"}`
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/members", body)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAddMemberByEmail — приглашение по email (201).
func TestAddMemberByEmail(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	body := `{"email": "editor@test.dev", "role": "editor"}`
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/members", body)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAddMemberInvalidRole — 422 при неизвестной роли.
func TestAddMemberInvalidRole(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	body := `{"user_id": "u-2", "role": "admin"}`
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/members", body)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

// TestUpdateMemberRoleValid — 200 при смене роли.
func TestUpdateMemberRoleValid(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	body := `{"role": "viewer"}`
	req := authedRequest(http.MethodPatch, "/api/v1/projects/p-1/members/u-2", body)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRemoveMemberNoContent — 204 при удалении участника.
func TestRemoveMemberNoContent(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	req := authedRequest(http.MethodDelete, "/api/v1/projects/p-1/members/u-2", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAddComment — POST /comments (201) возвращает созданный комментарий.
func TestAddComment(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/comments", `{"body": "проверка"}`)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var c commentDTO
	if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.Body != "проверка" || c.AuthorID != "u-1" {
		t.Fatalf("unexpected comment: %+v", c)
	}
}

// TestListComments — GET /comments возвращает список.
func TestListComments(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.comments = []*project.Comment{
		{ID: "c-1", ProjectID: "p-1", AuthorID: "u-1", Body: "первый", CreatedAt: time.Now()},
		{ID: "c-2", ProjectID: "p-1", AuthorID: "u-2", Body: "второй", CreatedAt: time.Now()},
	}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/comments", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp PaginatedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	list, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be array, got %T", resp.Data)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(list))
	}
}

// TestDeleteCommentNoContent — DELETE /comments/{id} (204).
func TestDeleteCommentNoContent(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	req := authedRequest(http.MethodDelete, "/api/v1/projects/p-1/comments/c-1", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestListCommentsNotFound — 404 для несуществующего проекта.
func TestListCommentsNotFound(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-404/comments", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestRequestReview — POST /review (201) возвращает запись ревью.
func TestRequestReview(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/review", `{"comment": "проверьте"}`)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var rv reviewDTO
	if err := json.NewDecoder(rec.Body).Decode(&rv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rv.Decision != project.ReviewRequested || rv.Comment != "проверьте" || rv.RequesterID != "u-1" {
		t.Fatalf("unexpected review: %+v", rv)
	}
}

// TestRequestReviewConflict — неверный переход → 422.
func TestRequestReviewConflict(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "in_review"}
	svc.reviewErr = project.ErrConflict

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/review", `{}`)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

// TestSignOffReview — POST /reviews/{id}/sign-off (200).
func TestSignOffReview(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "in_review"}

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/reviews/rv-1/sign-off", `{"comment": "ок"}`)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var rv reviewDTO
	if err := json.NewDecoder(rec.Body).Decode(&rv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rv.Decision != project.ReviewApproved || rv.ReviewerID != "u-1" {
		t.Fatalf("unexpected review: %+v", rv)
	}
}

// TestRequestChanges — POST /reviews/{id}/changes (200).
func TestRequestChanges(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "in_review"}

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/reviews/rv-1/changes", `{"comment": "доработать"}`)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var rv reviewDTO
	if err := json.NewDecoder(rec.Body).Decode(&rv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rv.Decision != project.ReviewChangesRequest || rv.Comment != "доработать" {
		t.Fatalf("unexpected review: %+v", rv)
	}
}

// TestListReviews — GET /reviews возвращает историю.
func TestListReviews(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "approved"}
	svc.reviews = []*project.ProjectReview{
		{ID: "rv-1", ProjectID: "p-1", RequesterID: "u-2", Decision: project.ReviewRequested, CreatedAt: time.Now()},
		{ID: "rv-2", ProjectID: "p-1", RequesterID: "u-2", ReviewerID: "u-1",
			Decision: project.ReviewApproved, CreatedAt: time.Now(), DecidedAt: &time.Time{}},
	}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/reviews", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp PaginatedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	list, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be array, got %T", resp.Data)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(list))
	}
}

// TestListReviewsNotFound — 404 для несуществующего проекта.
func TestListReviewsNotFound(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-404/reviews", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestApproveConfiguration — POST /configurations/{id}/approve (201).
func TestApproveConfiguration(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "approved"}

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/configurations/cfg-9/approve", `{"comment": "итоговая"}`)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var a approvalDTO
	if err := json.NewDecoder(rec.Body).Decode(&a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if a.ConfigurationID != "cfg-9" || a.Comment != "итоговая" || a.ApprovedByID != "u-1" {
		t.Fatalf("unexpected approval: %+v", a)
	}
}

// TestApproveConfigurationConflict — повторное утверждение → 422.
func TestApproveConfigurationConflict(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "approved"}
	svc.reviewErr = project.ErrConflict

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/configurations/cfg-9/approve", `{}`)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

// TestApproveConfigurationNotFound — 404 для несуществующего проекта.
func TestApproveConfigurationNotFound(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodPost, "/api/v1/projects/p-404/configurations/cfg-9/approve", `{}`)
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestGetConfigurationApproval — GET /configurations/{id}/approval (200).
func TestGetConfigurationApproval(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "approved"}
	svc.approvals = []*project.ConfigurationApproval{
		{ID: "a-1", ProjectID: "p-1", ConfigurationID: "cfg-9", ApprovedByID: "u-1",
			Comment: "итоговая", CreatedAt: time.Now()},
	}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/configurations/cfg-9/approval", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var a approvalDTO
	if err := json.NewDecoder(rec.Body).Decode(&a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if a.ConfigurationID != "cfg-9" || a.ApprovedByID != "u-1" {
		t.Fatalf("unexpected approval: %+v", a)
	}
}

// TestListApprovals — GET /approvals возвращает историю.
func TestListApprovals(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "approved"}
	svc.approvals = []*project.ConfigurationApproval{
		{ID: "a-1", ProjectID: "p-1", ConfigurationID: "cfg-8", ApprovedByID: "u-2", CreatedAt: time.Now()},
		{ID: "a-2", ProjectID: "p-1", ConfigurationID: "cfg-9", ApprovedByID: "u-1", CreatedAt: time.Now()},
	}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/approvals", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp PaginatedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	list, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected data to be array, got %T", resp.Data)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 approvals, got %d", len(list))
	}
}

// TestListApprovalsNotFound — 404 для несуществующего проекта.
func TestListApprovalsNotFound(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodGet, "/api/v1/projects/p-404/approvals", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestListConfigurations — GET /configurations возвращает историю ревизий.
func TestListConfigurations(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.configs = []*project.StairConfiguration{
		{ID: "cfg-1", ProjectID: "p-1", Revision: 1, WidthMM: 1000, HeightMM: 2600, Flight: "straight", CreatedAt: time.Now()},
		{ID: "cfg-2", ProjectID: "p-1", Revision: 2, WidthMM: 1100, HeightMM: 2600, Flight: "straight", CreatedAt: time.Now()},
	}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/configurations", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []configurationDTO
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 2 || list[0].Revision != 1 || list[1].Revision != 2 {
		t.Fatalf("unexpected list: %+v", list)
	}
}

// TestGetConfiguration — GET /configurations/{configID} возвращает ревизию.
func TestGetConfiguration(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.configs = []*project.StairConfiguration{
		{ID: "cfg-2", ProjectID: "p-1", Revision: 2, WidthMM: 1100, HeightMM: 2600, Flight: "straight", CreatedAt: time.Now()},
	}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/configurations/cfg-2", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var c configurationDTO
	if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.ID != "cfg-2" || c.Revision != 2 || c.Flight != "straight" {
		t.Fatalf("unexpected config: %+v", c)
	}
}

// TestRestoreConfiguration — POST /configurations/{configID}/restore (200).
func TestRestoreConfiguration(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.configs = []*project.StairConfiguration{
		{ID: "cfg-1", ProjectID: "p-1", Revision: 1, WidthMM: 1000, HeightMM: 2600, Flight: "straight", CreatedAt: time.Now()},
	}

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/configurations/cfg-1/restore", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var c configurationDTO
	if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.ID != "cfg-1" || c.Revision != 1 {
		t.Fatalf("unexpected config: %+v", c)
	}
}

// TestRestoreConfigurationForbidden — viewer не может восстановить ревизию → 403.
func TestRestoreConfigurationForbidden(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.reviewErr = project.ErrForbidden

	req := authedRequest(http.MethodPost, "/api/v1/projects/p-1/configurations/cfg-1/restore", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

// TestConfigurationNotFound — 404 для несуществующего проекта/ревизии.
func TestConfigurationNotFound(t *testing.T) {
	svc := newFakeProjectService()
	for _, path := range []string{
		"/api/v1/projects/p-404/configurations",
		"/api/v1/projects/p-404/configurations/cfg-1",
		"/api/v1/projects/p-404/configurations/cfg-1/restore",
	} {
		req := authedRequest(http.MethodGet, path, "")
		rec := httptest.NewRecorder()
		testRouterWithProjects(svc).ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: expected 404, got %d", path, rec.Code)
		}
	}
}
