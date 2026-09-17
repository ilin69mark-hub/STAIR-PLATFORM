package project

import (
	"context"
	"errors"
	"testing"

	"stairplatform/internal/application/audit"
	"stairplatform/internal/application/stair"
)

// errBoom — сентинельная ошибка репозитория для веток распространения.
var errBoom = errors.New("project: repository boom")

// memberErrRepo — репозиторий, у которого GetMember всегда падает.
type memberErrRepo struct {
	*fakeRepo
	err error
}

func (r memberErrRepo) GetMember(_ context.Context, _, _, _ string) (*ProjectMember, error) {
	return nil, r.err
}

// getProjectErrRepo — репозиторий с рабочим членством и падающим GetProject.
type getProjectErrRepo struct {
	*fakeRepo
	err error
}

func (r getProjectErrRepo) GetProject(_ context.Context, _, _, _ string) (*Project, error) {
	return nil, r.err
}

func assertBoom(t *testing.T, name string, err error) {
	t.Helper()
	if !errors.Is(err, errBoom) {
		t.Fatalf("%s: expected errBoom, got %v", name, err)
	}
}

func TestServiceMemberErrorPropagates(t *testing.T) {
	repo := memberErrRepo{fakeRepo: newFakeRepo(), err: errBoom}
	svc := NewService(repo, stair.NewService(), DefaultRules())
	ctx := context.Background()
	const p = "p-1"

	if _, err := svc.ListMembers(ctx, testTenant, testOwner, p); err == nil {
		t.Fatal("ListMembers: expected error")
	} else {
		assertBoom(t, "ListMembers", err)
	}
	assertBoom(t, "AddMember", svc.AddMember(ctx, testTenant, testOwner, p, "u-x", RoleViewer))
	assertBoom(t, "AddMemberByEmail", svc.AddMemberByEmail(ctx, testTenant, testOwner, p, "x@test.dev", RoleViewer))
	assertBoom(t, "UpdateMemberRole", svc.UpdateMemberRole(ctx, testTenant, testOwner, p, "u-x", RoleViewer))
	assertBoom(t, "RemoveMember", svc.RemoveMember(ctx, testTenant, testOwner, p, "u-x"))
	assertBoom(t, "DeleteComment", svc.DeleteComment(ctx, testTenant, testOwner, p, "c-1"))

	if _, err := svc.AddComment(ctx, testTenant, testOwner, p, "hi"); err == nil {
		t.Fatal("AddComment: expected error")
	} else {
		assertBoom(t, "AddComment", err)
	}
	if _, err := svc.ListComments(ctx, testTenant, testOwner, p); err == nil {
		t.Fatal("ListComments: expected error")
	} else {
		assertBoom(t, "ListComments", err)
	}
	if _, err := svc.Calculate(ctx, testTenant, testOwner, p, testConfig(), stair.Options{}); err == nil {
		t.Fatal("Calculate: expected error")
	} else {
		assertBoom(t, "Calculate", err)
	}
	if _, err := svc.Preview(ctx, testTenant, testOwner, p, testConfig(), stair.Options{}); err == nil {
		t.Fatal("Preview: expected error")
	} else {
		assertBoom(t, "Preview", err)
	}
	if _, err := svc.GetResult(ctx, testTenant, testOwner, p); err == nil {
		t.Fatal("GetResult: expected error")
	} else {
		assertBoom(t, "GetResult", err)
	}
	if _, err := svc.ExportCAD(ctx, testTenant, testOwner, p); err == nil {
		t.Fatal("ExportCAD: expected error")
	} else {
		assertBoom(t, "ExportCAD", err)
	}
	if _, err := svc.GetLatestConfig(ctx, testTenant, testOwner, p); err == nil {
		t.Fatal("GetLatestConfig: expected error")
	} else {
		assertBoom(t, "GetLatestConfig", err)
	}
	if _, err := svc.ListConfigurations(ctx, testTenant, testOwner, p); err == nil {
		t.Fatal("ListConfigurations: expected error")
	} else {
		assertBoom(t, "ListConfigurations", err)
	}
	if _, err := svc.GetConfiguration(ctx, testTenant, testOwner, p, "cfg-1"); err == nil {
		t.Fatal("GetConfiguration: expected error")
	} else {
		assertBoom(t, "GetConfiguration", err)
	}
	if _, err := svc.RestoreConfiguration(ctx, testTenant, testOwner, p, "cfg-1"); err == nil {
		t.Fatal("RestoreConfiguration: expected error")
	} else {
		assertBoom(t, "RestoreConfiguration", err)
	}
	if _, err := svc.RequestReview(ctx, testTenant, testOwner, p, "ok"); err == nil {
		t.Fatal("RequestReview: expected error")
	} else {
		assertBoom(t, "RequestReview", err)
	}
	if _, err := svc.SignOffReview(ctx, testTenant, testOwner, p, "r-1", "ok"); err == nil {
		t.Fatal("SignOffReview: expected error")
	} else {
		assertBoom(t, "SignOffReview", err)
	}
	if _, err := svc.RequestChanges(ctx, testTenant, testOwner, p, "r-1", "no"); err == nil {
		t.Fatal("RequestChanges: expected error")
	} else {
		assertBoom(t, "RequestChanges", err)
	}
	if _, err := svc.ListReviews(ctx, testTenant, testOwner, p); err == nil {
		t.Fatal("ListReviews: expected error")
	} else {
		assertBoom(t, "ListReviews", err)
	}
	if _, err := svc.ApproveConfiguration(ctx, testTenant, testOwner, p, "cfg-1", "ok"); err == nil {
		t.Fatal("ApproveConfiguration: expected error")
	} else {
		assertBoom(t, "ApproveConfiguration", err)
	}
	if _, err := svc.GetConfigurationApproval(ctx, testTenant, testOwner, p, "cfg-1"); err == nil {
		t.Fatal("GetConfigurationApproval: expected error")
	} else {
		assertBoom(t, "GetConfigurationApproval", err)
	}
	if _, err := svc.ListApprovals(ctx, testTenant, testOwner, p); err == nil {
		t.Fatal("ListApprovals: expected error")
	} else {
		assertBoom(t, "ListApprovals", err)
	}
}

func TestCalculateProjectLookupError(t *testing.T) {
	base := newFakeRepo()
	svc := NewService(base, stair.NewService(), DefaultRules())
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "P", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	broken := NewService(getProjectErrRepo{fakeRepo: base, err: errBoom}, stair.NewService(), DefaultRules())
	if _, err := broken.Calculate(context.Background(), testTenant, testOwner, p.ID, testConfig(), stair.Options{}); !errors.Is(err, errBoom) {
		t.Fatalf("Calculate: expected errBoom, got %v", err)
	}
}

func TestListTenantProjects(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	if _, err := svc.CreateProject(context.Background(), testTenant, testOwner, "A", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := svc.CreateProject(context.Background(), testTenant, "u-other", "B", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	list, err := svc.ListTenantProjects(context.Background(), testTenant)
	if err != nil {
		t.Fatalf("ListTenantProjects: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(list))
	}
}

func TestPreviewMembershipAndResult(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	ctx := context.Background()
	p, err := svc.CreateProject(ctx, testTenant, testOwner, "P", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	snap, err := svc.Preview(ctx, testTenant, testOwner, p.ID, testConfig(), stair.Options{})
	if err != nil {
		t.Fatalf("Preview owner: %v", err)
	}
	if snap.ProjectID != p.ID {
		t.Fatalf("expected snapshot for %q, got %q", p.ID, snap.ProjectID)
	}

	if err := svc.AddMember(ctx, testTenant, testOwner, p.ID, "u-viewer", RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if _, err := svc.Preview(ctx, testTenant, "u-viewer", p.ID, testConfig(), stair.Options{}); err != nil {
		t.Fatalf("Preview viewer: %v", err)
	}

	if _, err := svc.Preview(ctx, testTenant, "u-stranger", p.ID, testConfig(), stair.Options{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Preview stranger: expected ErrNotFound, got %v", err)
	}
	if _, err := svc.Preview(ctx, testTenant, testOwner, "missing", testConfig(), stair.Options{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Preview missing project: expected ErrNotFound, got %v", err)
	}
}

// auditRepo — in-memory audit.Repository для проверки записи событий.
type auditRepo struct {
	events []*audit.Event
	err    error
}

func (a *auditRepo) Insert(_ context.Context, e *audit.Event) error {
	if a.err != nil {
		return a.err
	}
	a.events = append(a.events, e)
	return nil
}

func (a *auditRepo) ListByProject(_ context.Context, _, _ string) ([]*audit.Event, error) {
	return nil, a.err
}

func (a *auditRepo) ListByTenant(_ context.Context, _ string) ([]*audit.Event, error) {
	return nil, a.err
}

func TestServiceRecordsAuditEvents(t *testing.T) {
	ar := &auditRepo{}
	auditSvc := audit.NewService(ar)
	svc := NewService(newFakeRepo(), stair.NewService(), DefaultRules(), auditSvc)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, testTenant, testOwner, "P", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if len(ar.events) != 1 {
		t.Fatalf("expected 1 audit event after create, got %d", len(ar.events))
	}
	if ar.events[0].Action != audit.ActionProjectCreated || ar.events[0].ProjectID != p.ID {
		t.Fatalf("unexpected audit event: %+v", ar.events[0])
	}

	if _, err := svc.Calculate(ctx, testTenant, testOwner, p.ID, testConfig(), stair.Options{}); err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if len(ar.events) != 2 {
		t.Fatalf("expected 2 audit events after calculate, got %d", len(ar.events))
	}

	if err := svc.AddMember(ctx, testTenant, "u-stranger", p.ID, "u-x", RoleViewer); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AddMember stranger: expected ErrNotFound, got %v", err)
	}
	if err := svc.AddMember(ctx, testTenant, testOwner, p.ID, "u-v", RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if len(ar.events) != 3 {
		t.Fatalf("expected 3 audit events after add member, got %d", len(ar.events))
	}

	ar.err = errBoom
	if err := svc.RemoveMember(ctx, testTenant, testOwner, p.ID, "u-v"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if len(ar.events) != 3 {
		t.Fatalf("record must be best-effort on audit error, got %d events", len(ar.events))
	}
}

func TestServiceNonMemberIsNotFound(t *testing.T) {
	svc := NewService(newFakeRepo(), stair.NewService(), DefaultRules())
	ctx := context.Background()
	p, err := svc.CreateProject(ctx, testTenant, testOwner, "P", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	const stranger = "u-stranger"

	notFound := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s: expected ErrNotFound, got %v", name, err)
		}
	}

	if _, err := svc.ListMembers(ctx, testTenant, stranger, p.ID); true {
		notFound("ListMembers", err)
	}
	if _, err := svc.AddComment(ctx, testTenant, stranger, p.ID, "hi"); true {
		notFound("AddComment", err)
	}
	if _, err := svc.ListComments(ctx, testTenant, stranger, p.ID); true {
		notFound("ListComments", err)
	}
	if _, err := svc.Calculate(ctx, testTenant, stranger, p.ID, testConfig(), stair.Options{}); true {
		notFound("Calculate", err)
	}
	if _, err := svc.Preview(ctx, testTenant, stranger, p.ID, testConfig(), stair.Options{}); true {
		notFound("Preview", err)
	}
	if _, err := svc.GetResult(ctx, testTenant, stranger, p.ID); true {
		notFound("GetResult", err)
	}
	if _, err := svc.ExportCAD(ctx, testTenant, stranger, p.ID); true {
		notFound("ExportCAD", err)
	}
	if _, err := svc.GetLatestConfig(ctx, testTenant, stranger, p.ID); true {
		notFound("GetLatestConfig", err)
	}
	if _, err := svc.ListConfigurations(ctx, testTenant, stranger, p.ID); true {
		notFound("ListConfigurations", err)
	}
	if _, err := svc.GetConfiguration(ctx, testTenant, stranger, p.ID, "cfg-1"); true {
		notFound("GetConfiguration", err)
	}
	if _, err := svc.RestoreConfiguration(ctx, testTenant, stranger, p.ID, "cfg-1"); true {
		notFound("RestoreConfiguration", err)
	}
	if _, err := svc.RequestReview(ctx, testTenant, stranger, p.ID, "ok"); true {
		notFound("RequestReview", err)
	}
	if _, err := svc.SignOffReview(ctx, testTenant, stranger, p.ID, "r-1", "ok"); true {
		notFound("SignOffReview", err)
	}
	if _, err := svc.RequestChanges(ctx, testTenant, stranger, p.ID, "r-1", "no"); true {
		notFound("RequestChanges", err)
	}
	if _, err := svc.ListReviews(ctx, testTenant, stranger, p.ID); true {
		notFound("ListReviews", err)
	}
	if _, err := svc.ApproveConfiguration(ctx, testTenant, stranger, p.ID, "cfg-1", "ok"); true {
		notFound("ApproveConfiguration", err)
	}
	if _, err := svc.GetConfigurationApproval(ctx, testTenant, stranger, p.ID, "cfg-1"); true {
		notFound("GetConfigurationApproval", err)
	}
	if _, err := svc.ListApprovals(ctx, testTenant, stranger, p.ID); true {
		notFound("ListApprovals", err)
	}

	notFound("AddMember", svc.AddMember(ctx, testTenant, stranger, p.ID, "u-x", RoleViewer))
	notFound("AddMemberByEmail", svc.AddMemberByEmail(ctx, testTenant, stranger, p.ID, "x@test.dev", RoleViewer))
	notFound("UpdateMemberRole", svc.UpdateMemberRole(ctx, testTenant, stranger, p.ID, "u-x", RoleViewer))
	notFound("RemoveMember", svc.RemoveMember(ctx, testTenant, stranger, p.ID, "u-x"))
	notFound("DeleteComment", svc.DeleteComment(ctx, testTenant, stranger, p.ID, "c-1"))
}

func TestCreateProjectRequiresOwner(t *testing.T) {
	svc := NewService(newFakeRepo(), stair.NewService(), DefaultRules())
	if _, err := svc.CreateProject(context.Background(), testTenant, "", "P", ""); err == nil {
		t.Fatal("expected error for empty owner")
	}
}
