package project

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
)

// fakeRepo — тестовая реализация Repository в памяти.
type fakeRepo struct {
	projects     map[string]*Project                  // ключ: tenantID + "/" + id
	members      map[string]map[string]*ProjectMember // ключ: projectID → userID → member
	usersByEmail map[string]string                    // email → userID (в tenant, C2)
	configs      map[string][]*StairConfiguration
	calculations map[string][]*Calculation
	comments     map[string][]*Comment       // ключ: projectID → []*Comment
	reviews      map[string][]*ProjectReview // ключ: projectID → []*ProjectReview
	next         int
	err          error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		projects:     map[string]*Project{},
		members:      map[string]map[string]*ProjectMember{},
		usersByEmail: map[string]string{"editor@test.dev": "u-editor", "viewer@test.dev": "u-viewer"},
		configs:      map[string][]*StairConfiguration{},
		calculations: map[string][]*Calculation{},
		comments:     map[string][]*Comment{},
		reviews:      map[string][]*ProjectReview{},
	}
}

const testTenant = "t-1"

// testOwner — фиксированный ID владельца в тестах.
const testOwner = "u-owner"

func key(tenantID, id string) string { return tenantID + "/" + id }

func (f *fakeRepo) memberLookup(tenantID, projectID, userID string) (*ProjectMember, bool) {
	if _, ok := f.projects[key(tenantID, projectID)]; !ok {
		return nil, false
	}
	ms, ok := f.members[projectID]
	if !ok {
		return nil, false
	}
	m, ok := ms[userID]
	return m, ok
}

func (f *fakeRepo) CreateProject(ctx context.Context, tenantID, ownerID string, p *Project) error {
	if f.err != nil {
		return f.err
	}
	f.next++
	p.ID = itoa(f.next)
	p.OwnerID = ownerID
	f.projects[key(tenantID, p.ID)] = p
	if f.members[p.ID] == nil {
		f.members[p.ID] = map[string]*ProjectMember{}
	}
	f.members[p.ID][ownerID] = &ProjectMember{ProjectID: p.ID, UserID: ownerID, Role: RoleOwner}
	return nil
}

func (f *fakeRepo) GetProject(ctx context.Context, tenantID, userID, id string) (*Project, error) {
	p, ok := f.projects[key(tenantID, id)]
	if !ok {
		return nil, ErrNotFound
	}
	if _, ok := f.memberLookup(tenantID, id, userID); !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (f *fakeRepo) ListProjects(ctx context.Context, tenantID, userID string) ([]*Project, error) {
	var out []*Project
	prefix := tenantID + "/"
	for k, p := range f.projects {
		if strings.HasPrefix(k, prefix) {
			if _, ok := f.memberLookup(tenantID, p.ID, userID); ok {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) GetMember(ctx context.Context, tenantID, projectID, userID string) (*ProjectMember, error) {
	m, ok := f.memberLookup(tenantID, projectID, userID)
	if !ok {
		return nil, ErrNotFound
	}
	return m, nil
}

func (f *fakeRepo) ListMembers(ctx context.Context, tenantID, projectID string) ([]*ProjectMember, error) {
	if _, ok := f.projects[key(tenantID, projectID)]; !ok {
		return nil, ErrNotFound
	}
	var out []*ProjectMember
	for _, m := range f.members[projectID] {
		out = append(out, m)
	}
	return out, nil
}

func (f *fakeRepo) AddMember(ctx context.Context, tenantID, projectID string, m *ProjectMember) error {
	if _, ok := f.projects[key(tenantID, projectID)]; !ok {
		return ErrNotFound
	}
	if f.members[projectID] == nil {
		f.members[projectID] = map[string]*ProjectMember{}
	}
	f.members[projectID][m.UserID] = m
	return nil
}

func (f *fakeRepo) AddMemberByEmail(ctx context.Context, tenantID, projectID, email string, role ProjectRole) error {
	uid, ok := f.usersByEmail[email]
	if !ok {
		return ErrNotFound
	}
	return f.AddMember(ctx, tenantID, projectID, &ProjectMember{ProjectID: projectID, UserID: uid, Role: role})
}

func (f *fakeRepo) UpdateMemberRole(ctx context.Context, tenantID, projectID, userID string, role ProjectRole) error {
	m, ok := f.memberLookup(tenantID, projectID, userID)
	if !ok {
		return ErrNotFound
	}
	if m.Role == RoleOwner {
		return fmt.Errorf("%v", ErrNotFound)
	}
	m.Role = role
	return nil
}

func (f *fakeRepo) RemoveMember(ctx context.Context, tenantID, projectID, userID string) error {
	m, ok := f.memberLookup(tenantID, projectID, userID)
	if !ok {
		return ErrNotFound
	}
	if m.Role == RoleOwner {
		return fmt.Errorf("%v", ErrNotFound)
	}
	delete(f.members[projectID], userID)
	return nil
}

func (f *fakeRepo) SaveConfiguration(ctx context.Context, c *StairConfiguration) error {
	f.configs[c.ProjectID] = append(f.configs[c.ProjectID], c)
	return nil
}

func (f *fakeRepo) GetLatestConfiguration(ctx context.Context, tenantID, projectID string) (*StairConfiguration, error) {
	cfgs := f.configs[projectID]
	if len(cfgs) == 0 {
		return nil, ErrNotFound
	}
	return cfgs[len(cfgs)-1], nil
}

func (f *fakeRepo) SaveCalculationWithConfig(ctx context.Context, tenantID string, cfg *StairConfiguration, snap Snapshot) (*Calculation, error) {
	if err := f.SaveConfiguration(ctx, cfg); err != nil {
		return nil, err
	}
	calc := &Calculation{
		ProjectID: cfg.ProjectID,
		Valid:     snap.Validation.Valid,
		Blocking:  snap.Validation.Blocking,
		Result:    []byte("{}"),
	}
	calc.ConfigurationID = cfg.ID
	if err := f.SaveCalculation(ctx, calc); err != nil {
		return nil, err
	}
	return calc, nil
}

func (f *fakeRepo) SaveCalculation(ctx context.Context, c *Calculation) error {
	if c.ConfigurationID == "" {
		c.ConfigurationID = "cfg-" + c.ProjectID
	}
	f.calculations[c.ProjectID] = append(f.calculations[c.ProjectID], c)
	return nil
}

func (f *fakeRepo) GetLatestCalculation(ctx context.Context, tenantID, projectID string) (*Calculation, error) {
	cs := f.calculations[projectID]
	if len(cs) == 0 {
		return nil, ErrNotFound
	}
	return cs[len(cs)-1], nil
}

func (f *fakeRepo) AddComment(ctx context.Context, tenantID, projectID string, c *Comment) (*Comment, error) {
	if _, ok := f.projects[key(tenantID, projectID)]; !ok {
		return nil, ErrNotFound
	}
	f.next++
	c.ID = itoa(f.next)
	c.CreatedAt = time.Now()
	f.comments[projectID] = append(f.comments[projectID], c)
	return c, nil
}

func (f *fakeRepo) ListComments(ctx context.Context, tenantID, projectID string) ([]*Comment, error) {
	if _, ok := f.projects[key(tenantID, projectID)]; !ok {
		return nil, ErrNotFound
	}
	return f.comments[projectID], nil
}

func (f *fakeRepo) DeleteComment(ctx context.Context, tenantID, projectID, commentID, actorID string) error {
	if _, ok := f.projects[key(tenantID, projectID)]; !ok {
		return ErrNotFound
	}
	for i, c := range f.comments[projectID] {
		if c.ID != commentID {
			continue
		}
		owner := false
		if m, ok := f.memberLookup(tenantID, projectID, actorID); ok {
			owner = m.Role == RoleOwner
		}
		if c.AuthorID != actorID && !owner {
			return ErrForbidden
		}
		f.comments[projectID] = append(f.comments[projectID][:i], f.comments[projectID][i+1:]...)
		return nil
	}
	return ErrNotFound
}

// setStatus форсирует статус проекта в тестовом репозитории (вспомогательное).
func (f *fakeRepo) setStatus(tenantID, projectID, status string) {
	if p, ok := f.projects[key(tenantID, projectID)]; ok {
		p.Status = status
	}
}

func (f *fakeRepo) RequestReview(ctx context.Context, tenantID, projectID, requesterID, comment string) (*ProjectReview, error) {
	p, ok := f.projects[key(tenantID, projectID)]
	if !ok {
		return nil, ErrNotFound
	}
	if p.Status != StatusDraft && p.Status != StatusChangesRequested {
		return nil, ErrConflict
	}
	f.next++
	rv := &ProjectReview{
		ID: itoa(f.next), ProjectID: projectID, RequesterID: requesterID,
		Decision: ReviewRequested, Comment: comment, CreatedAt: time.Now(),
	}
	f.reviews[projectID] = append(f.reviews[projectID], rv)
	p.Status = StatusInReview
	return rv, nil
}

func (f *fakeRepo) DecideReview(ctx context.Context, tenantID, projectID, reviewID, reviewerID, decision, comment string) (*ProjectReview, error) {
	p, ok := f.projects[key(tenantID, projectID)]
	if !ok {
		return nil, ErrNotFound
	}
	for _, rv := range f.reviews[projectID] {
		if rv.ID != reviewID || rv.Decision != ReviewRequested || rv.DecidedAt != nil {
			continue
		}
		if rv.RequesterID == reviewerID {
			return nil, ErrForbidden
		}
		if p.Status != StatusInReview {
			return nil, ErrConflict
		}
		rv.Decision = decision
		rv.ReviewerID = reviewerID
		rv.Comment = comment
		now := time.Now()
		rv.DecidedAt = &now
		if decision == ReviewApproved {
			p.Status = StatusApproved
		} else {
			p.Status = StatusChangesRequested
		}
		return rv, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) ListReviews(ctx context.Context, tenantID, projectID string) ([]*ProjectReview, error) {
	if _, ok := f.projects[key(tenantID, projectID)]; !ok {
		return nil, ErrNotFound
	}
	return f.reviews[projectID], nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func testConfig() stair.Config {
	return stair.Config{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2700),
		Flight:            engineering.FlightStraight,
		StepHeight:        engineering.Length(180),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		Clearance:         engineering.Length(80),
		RailingHeight:     engineering.Length(900),
	}
}

func TestCreateProject(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())

	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Лестница на 2 этаж", "Заказ 1")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected assigned ID")
	}
	if p.Status != "draft" {
		t.Fatalf("expected status draft, got %q", p.Status)
	}
	if p.OwnerID != testOwner {
		t.Fatalf("expected owner %q, got %q", testOwner, p.OwnerID)
	}

	got, err := svc.GetProject(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Лестница на 2 этаж" {
		t.Fatalf("name mismatch: %q", got.Name)
	}
}

func TestCreateProjectRequiresName(t *testing.T) {
	svc := NewService(newFakeRepo(), stair.NewService(), DefaultRules())
	if _, err := svc.CreateProject(context.Background(), testTenant, testOwner, "", ""); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetProjectNotFound(t *testing.T) {
	svc := NewService(newFakeRepo(), stair.NewService(), DefaultRules())
	if _, err := svc.GetProject(context.Background(), testTenant, testOwner, "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCalculateSavesConfigAndResult(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())

	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Тест", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	calc, err := svc.Calculate(context.Background(), testTenant, testOwner, p.ID, testConfig(), stair.Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if !calc.Valid {
		t.Fatal("expected valid calculation")
	}
	if calc.Blocking {
		t.Fatal("expected non-blocking")
	}
	if len(calc.Result) == 0 {
		t.Fatal("expected result snapshot")
	}

	cfg, err := svc.GetLatestConfig(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("GetLatestConfig: %v", err)
	}
	if cfg.WidthMM != 900 || cfg.HeightMM != 2700 {
		t.Fatalf("config mismatch: %+v", cfg)
	}

	got, err := svc.GetResult(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if got.ConfigurationID != calc.ConfigurationID {
		t.Fatalf("calculation not linked to config: %s vs %s", got.ConfigurationID, calc.ConfigurationID)
	}
}

func TestCalculateProjectNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	if _, err := svc.Calculate(context.Background(), testTenant, testOwner, "missing", testConfig(), stair.Options{}); err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestCalculateBlockingValidation(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Тест", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	cfg := testConfig()
	cfg.StepHeight = engineering.Length(60) // вне допустимого диапазона
	calc, err := svc.Calculate(context.Background(), testTenant, testOwner, p.ID, cfg, stair.Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if calc.Valid || !calc.Blocking {
		t.Fatalf("expected blocking validation (Valid=%v Blocking=%v)", calc.Valid, calc.Blocking)
	}
}

// TestTenantIsolation — SEC-0005: проект tenant'а A недоступен из tenant'а B.
func TestTenantIsolation(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())

	p, err := svc.CreateProject(context.Background(), "t-a", testOwner, "Секрет", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := svc.GetProject(context.Background(), "t-b", testOwner, p.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for other tenant, got %v", err)
	}
	list, err := svc.ListProjects(context.Background(), "t-b", testOwner)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B must not see tenant A projects, got %d", len(list))
	}
	if _, err := svc.Calculate(context.Background(), "t-b", testOwner, p.ID, testConfig(), stair.Options{}); err == nil {
		t.Fatal("cross-tenant calculate must fail")
	}
}

// TestAddMemberOwnerOnly — изменение состава участников доступно только owner.
func TestAddMemberOwnerOnly(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Тест", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := svc.AddMember(context.Background(), testTenant, testOwner, p.ID, "u-editor", RoleEditor); err != nil {
		t.Fatalf("owner AddMember: %v", err)
	}
	// Редактор не может добавлять членов.
	if err := svc.AddMember(context.Background(), testTenant, "u-editor", p.ID, "u-viewer", RoleViewer); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for editor, got %v", err)
	}
	// Не-член получает ErrNotFound.
	if err := svc.AddMember(context.Background(), testTenant, "u-stranger", p.ID, "u-x", RoleViewer); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for non-member, got %v", err)
	}
}

// TestAddMemberUnknownProject — добавление в несуществующий проект.
// TestAddMemberByEmail — приглашение по email (C2): резолв в пользователя
// tenant, только owner, неизвестный email → ErrNotFound.
func TestAddMemberByEmail(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Тест", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := svc.AddMemberByEmail(context.Background(), testTenant, testOwner, p.ID, "editor@test.dev", RoleEditor); err != nil {
		t.Fatalf("owner invite: %v", err)
	}
	members, err := svc.ListMembers(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	// редактор может редактировать (роль назначена), но not owner
	if err := svc.AddMemberByEmail(context.Background(), testTenant, "u-editor", p.ID, "viewer@test.dev", RoleViewer); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for editor, got %v", err)
	}
	// неизвестный email
	if err := svc.AddMemberByEmail(context.Background(), testTenant, testOwner, p.ID, "nobody@test.dev", RoleViewer); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	// пустой email
	if err := svc.AddMemberByEmail(context.Background(), testTenant, testOwner, p.ID, "", RoleViewer); err == nil {
		t.Fatal("expected error for empty email")
	}
}

func TestAddMemberUnknownProject(t *testing.T) {
	svc := NewService(newFakeRepo(), stair.NewService(), DefaultRules())
	if err := svc.AddMember(context.Background(), testTenant, testOwner, "missing", "u-x", RoleViewer); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestMemberRoleLifecycle — owner добавляет/viewer'а, повышает до editor,
// затем удаляет; после удаления доступ к проекту исчезает.
func TestMemberRoleLifecycle(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Тест", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := svc.AddMember(context.Background(), testTenant, testOwner, p.ID, "u-viewer", RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	members, err := svc.ListMembers(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members (owner+viewer), got %d", len(members))
	}

	// Viewer видит проект, но не может рассчитать (нет права на изменение).
	if _, err := svc.GetProject(context.Background(), testTenant, "u-viewer", p.ID); err != nil {
		t.Fatalf("viewer must see project: %v", err)
	}
	if _, err := svc.Calculate(context.Background(), testTenant, "u-viewer", p.ID, testConfig(), stair.Options{}); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for viewer, got %v", err)
	}

	// Повышаем до editor — теперь можно рассчитывать.
	if err := svc.UpdateMemberRole(context.Background(), testTenant, testOwner, p.ID, "u-viewer", RoleEditor); err != nil {
		t.Fatalf("UpdateMemberRole: %v", err)
	}
	if _, err := svc.Calculate(context.Background(), testTenant, "u-viewer", p.ID, testConfig(), stair.Options{}); err != nil {
		t.Fatalf("editor calculate: %v", err)
	}

	// Удаляем участника — доступ пропадает.
	if err := svc.RemoveMember(context.Background(), testTenant, testOwner, p.ID, "u-viewer"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if _, err := svc.GetProject(context.Background(), testTenant, "u-viewer", p.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after removal, got %v", err)
	}
}

// TestRemoveMemberRequiresOwner — изменение ролей/удаление доступно только owner.
func TestUpdateMemberRoleOwnerOnly(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Тест", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := svc.AddMember(context.Background(), testTenant, testOwner, p.ID, "u-editor", RoleEditor); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if err := svc.UpdateMemberRole(context.Background(), testTenant, "u-editor", p.ID, "u-x", RoleViewer); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if err := svc.RemoveMember(context.Background(), testTenant, "u-editor", p.ID, "u-x"); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

// TestParseProjectRole — разбор допустимых и недопустимых ролей.
func TestParseProjectRole(t *testing.T) {
	for _, ok := range []struct {
		in    string
		valid bool
	}{
		{"owner", true}, {"editor", true}, {"viewer", true},
		{"admin", false}, {"", false}, {"OWNER", false},
	} {
		_, err := ParseProjectRole(ok.in)
		if (err == nil) != ok.valid {
			t.Fatalf("role %q: valid=%v, err=%v", ok.in, ok.valid, err)
		}
	}
	if !RoleOwner.CanEdit() || !RoleEditor.CanEdit() {
		t.Fatal("owner/editor must CanEdit")
	}
	if RoleViewer.CanEdit() {
		t.Fatal("viewer must not CanEdit")
	}
	if !RoleOwner.CanManage() {
		t.Fatal("owner must CanManage")
	}
	if RoleEditor.CanManage() || RoleViewer.CanManage() {
		t.Fatal("only owner CanManage")
	}
}

// TestCommentThread — комментарии (EDR-0009): член добавляет, список
// сортирован по времени, пустое тело отклоняется.
func TestCommentThread(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Обсуждение", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	c, err := svc.AddComment(context.Background(), testTenant, testOwner, p.ID, "Сделать перила выше?")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if c.ID == "" || c.AuthorID != testOwner {
		t.Fatalf("comment = %+v", c)
	}
	if _, err := svc.AddComment(context.Background(), testTenant, testOwner, p.ID, ""); err == nil {
		t.Fatal("expected error for empty comment body")
	}

	list, err := svc.ListComments(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(list) != 1 || list[0].Body != "Сделать перила выше?" {
		t.Fatalf("list = %+v", list)
	}
}

// TestCommentRequiresMembership — не-член не может читать/писать комментарии.
func TestCommentRequiresMembership(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Приватный", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := svc.AddComment(context.Background(), testTenant, "u-stranger", p.ID, "хак"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for non-member add, got %v", err)
	}
	if _, err := svc.ListComments(context.Background(), testTenant, "u-stranger", p.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for non-member list, got %v", err)
	}
}

// TestDeleteComment — автор или владелец удаляют; чужой член — ErrForbidden.
func TestDeleteComment(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Удаление", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := svc.AddMemberByEmail(context.Background(), testTenant, testOwner, p.ID, "editor@test.dev", RoleEditor); err != nil {
		t.Fatalf("AddMemberByEmail: %v", err)
	}

	// Комментарий от редактора.
	ec, err := svc.AddComment(context.Background(), testTenant, "u-editor", p.ID, "моё замечание")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	// Чужой комментарий от владельца.
	oc, err := svc.AddComment(context.Background(), testTenant, testOwner, p.ID, "комментарий владельца")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	// Редактор удаляет чужой (владельческий) комментарий — запрещено.
	if err := svc.DeleteComment(context.Background(), testTenant, "u-editor", p.ID, oc.ID); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-author, got %v", err)
	}
	// Редактор удаляет свой — ок.
	if err := svc.DeleteComment(context.Background(), testTenant, "u-editor", p.ID, ec.ID); err != nil {
		t.Fatalf("author delete: %v", err)
	}
	// Владелец удаляет чужой — ок (CanManage).
	if err := svc.DeleteComment(context.Background(), testTenant, testOwner, p.ID, oc.ID); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	// Повторное удаление — ErrNotFound.
	if err := svc.DeleteComment(context.Background(), testTenant, testOwner, p.ID, ec.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing comment, got %v", err)
	}
}

// TestReviewLifecycle (EDR-0010): editor запрашивает ревью, owner
// подписывает; статусы проходят draft → in_review → approved; история
// содержит обе строки.
func TestReviewLifecycle(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Ревью", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := svc.AddMemberByEmail(context.Background(), testTenant, testOwner, p.ID, "editor@test.dev", RoleEditor); err != nil {
		t.Fatalf("AddMemberByEmail: %v", err)
	}

	rv, err := svc.RequestReview(context.Background(), testTenant, "u-editor", p.ID, "проверьте расчёт")
	if err != nil {
		t.Fatalf("RequestReview: %v", err)
	}
	if rv.Decision != ReviewRequested || rv.RequesterID != "u-editor" {
		t.Fatalf("review = %+v", rv)
	}
	got, err := svc.GetProject(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Status != StatusInReview {
		t.Fatalf("status = %q, want in_review", got.Status)
	}

	rv2, err := svc.SignOffReview(context.Background(), testTenant, testOwner, p.ID, rv.ID, "ок")
	if err != nil {
		t.Fatalf("SignOffReview: %v", err)
	}
	if rv2.Decision != ReviewApproved || rv2.ReviewerID != testOwner || rv2.DecidedAt == nil {
		t.Fatalf("signed review = %+v", rv2)
	}
	got, err = svc.GetProject(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Status != StatusApproved {
		t.Fatalf("status = %q, want approved", got.Status)
	}

	reviews, err := svc.ListReviews(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("ListReviews: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("reviews len = %d, want 1", len(reviews))
	}
	if reviews[0].Decision != ReviewApproved || reviews[0].RequesterID != "u-editor" {
		t.Fatalf("review[0] = %+v", reviews[0])
	}
}

// TestReviewPermissions (EDR-0010): viewer не может запросить ревью;
// editor не может подписать; self-approve запрещён; подпись из draft —
// ErrConflict.
func TestReviewPermissions(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Права ревью", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := svc.AddMemberByEmail(context.Background(), testTenant, testOwner, p.ID, "viewer@test.dev", RoleViewer); err != nil {
		t.Fatalf("AddMemberByEmail: %v", err)
	}
	if err := svc.AddMemberByEmail(context.Background(), testTenant, testOwner, p.ID, "editor@test.dev", RoleEditor); err != nil {
		t.Fatalf("AddMemberByEmail: %v", err)
	}

	// Viewer не может запросить ревью.
	if _, err := svc.RequestReview(context.Background(), testTenant, "u-viewer", p.ID, "x"); err != ErrForbidden {
		t.Fatalf("viewer request: want ErrForbidden, got %v", err)
	}
	// Editor запрашивает.
	rv, err := svc.RequestReview(context.Background(), testTenant, "u-editor", p.ID, "")
	if err != nil {
		t.Fatalf("RequestReview: %v", err)
	}
	// Editor (не owner) не может подписать.
	if _, err := svc.SignOffReview(context.Background(), testTenant, "u-editor", p.ID, rv.ID, ""); err != ErrForbidden {
		t.Fatalf("editor sign-off: want ErrForbidden, got %v", err)
	}
	// Owner не может подписать собственный запрос (self-approve).
	repo.setStatus(testTenant, p.ID, StatusDraft)
	own, err := svc.RequestReview(context.Background(), testTenant, testOwner, p.ID, "мой запрос")
	if err != nil {
		t.Fatalf("RequestReview(owner): %v", err)
	}
	if _, err := svc.SignOffReview(context.Background(), testTenant, testOwner, p.ID, own.ID, ""); err != ErrForbidden {
		t.Fatalf("self-approve: want ErrForbidden, got %v", err)
	}
	// Подпись из draft — ErrConflict.
	repo.setStatus(testTenant, p.ID, StatusDraft)
	ed, err := svc.RequestReview(context.Background(), testTenant, "u-editor", p.ID, "ещё раз")
	if err != nil {
		t.Fatalf("RequestReview: %v", err)
	}
	repo.setStatus(testTenant, p.ID, StatusDraft)
	if _, err := svc.SignOffReview(context.Background(), testTenant, testOwner, p.ID, ed.ID, ""); err != ErrConflict {
		t.Fatalf("sign-off from draft: want ErrConflict, got %v", err)
	}
}

// TestRequestChangesReturn (EDR-0010): owner возвращает ревью на доработку;
// проект становится changes_requested и снова доступен для запроса.
// Calculate в in_review блокируется.
func TestRequestChangesReturn(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Доработка", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := svc.AddMemberByEmail(context.Background(), testTenant, testOwner, p.ID, "editor@test.dev", RoleEditor); err != nil {
		t.Fatalf("AddMemberByEmail: %v", err)
	}

	// Editor запрашивает, owner возвращает на доработку.
	rv, err := svc.RequestReview(context.Background(), testTenant, "u-editor", p.ID, "запрос")
	if err != nil {
		t.Fatalf("RequestReview: %v", err)
	}
	rv2, err := svc.RequestChanges(context.Background(), testTenant, testOwner, p.ID, rv.ID, "исправьте марш")
	if err != nil {
		t.Fatalf("RequestChanges: %v", err)
	}
	if rv2.Decision != ReviewChangesRequest || rv2.ReviewerID != testOwner {
		t.Fatalf("changes review = %+v", rv2)
	}
	got, err := svc.GetProject(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Status != StatusChangesRequested {
		t.Fatalf("status = %q, want changes_requested", got.Status)
	}

	// Повторный запрос из changes_requested допустим.
	if _, err := svc.RequestReview(context.Background(), testTenant, "u-editor", p.ID, "снова"); err != nil {
		t.Fatalf("RequestReview from changes_requested: %v", err)
	}

	// Calculate в in_review блокируется (ErrConflict).
	repo.setStatus(testTenant, p.ID, StatusInReview)
	cfg := testConfig()
	if _, err := svc.Calculate(context.Background(), testTenant, testOwner, p.ID, cfg, stair.Options{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("Calculate in_review: want ErrConflict, got %v", err)
	}
	// Calculate из approved-статуса после решения допустим.
	repo.setStatus(testTenant, p.ID, StatusApproved)
	if _, err := svc.Calculate(context.Background(), testTenant, testOwner, p.ID, cfg, stair.Options{}); err != nil {
		t.Fatalf("Calculate approved: %v", err)
	}
}

// TestReviewNonMember (EDR-0010): не-член не видит историю ревью.
func TestReviewNonMember(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "Приватное ревью", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := svc.RequestReview(context.Background(), testTenant, "u-stranger", p.ID, ""); err != ErrNotFound {
		t.Fatalf("stranger request: want ErrNotFound, got %v", err)
	}
	if _, err := svc.ListReviews(context.Background(), testTenant, "u-stranger", p.ID); err != ErrNotFound {
		t.Fatalf("stranger list: want ErrNotFound, got %v", err)
	}
}
