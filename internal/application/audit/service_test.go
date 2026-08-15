package audit

import (
	"context"
	"errors"
	"testing"
)

// fakeRepo — тестовый репозиторий аудита.
type fakeRepo struct {
	events []*Event
	err    error
}

func (f *fakeRepo) Insert(_ context.Context, e *Event) error {
	if f.err != nil {
		return f.err
	}
	e.ID = "evt-1"
	f.events = append(f.events, e)
	return nil
}

func (f *fakeRepo) ListByProject(_ context.Context, _, projectID string) ([]*Event, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []*Event
	for _, e := range f.events {
		if e.ProjectID == projectID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListByTenant(_ context.Context, tenantID string) ([]*Event, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []*Event
	for _, e := range f.events {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, nil
}

func TestRecord(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	err := svc.Record(context.Background(), &Event{
		ActorID: "u1", TenantID: "t1", ProjectID: "p1",
		Action: ActionProjectCreated, Result: ResultOK,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(repo.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(repo.events))
	}
	if repo.events[0].Action != ActionProjectCreated {
		t.Fatalf("unexpected action %q", repo.events[0].Action)
	}
}

func TestRecordPropagatesRepoError(t *testing.T) {
	repo := &fakeRepo{err: errors.New("db down")}
	svc := NewService(repo)
	if err := svc.Record(context.Background(), &Event{TenantID: "t1"}); err == nil {
		t.Fatal("expected error from repo")
	}
}

func TestListProjectAudit(t *testing.T) {
	repo := &fakeRepo{
		events: []*Event{
			{TenantID: "t1", ProjectID: "p1", Action: ActionProjectCreated},
			{TenantID: "t1", ProjectID: "p2", Action: ActionProjectCreated},
			{TenantID: "t1", ProjectID: "p1", Action: ActionProjectModified},
		},
	}
	svc := NewService(repo)
	events, err := svc.ListProjectAudit(context.Background(), "t1", "p1")
	if err != nil {
		t.Fatalf("ListProjectAudit: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestListTenantAudit(t *testing.T) {
	repo := &fakeRepo{
		events: []*Event{
			{TenantID: "t1", Action: ActionAuthLogin},
			{TenantID: "t2", Action: ActionAuthLogin},
		},
	}
	svc := NewService(repo)
	events, err := svc.ListTenantAudit(context.Background(), "t1")
	if err != nil {
		t.Fatalf("ListTenantAudit: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}
