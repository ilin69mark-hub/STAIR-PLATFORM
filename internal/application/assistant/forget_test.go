package assistant

import (
	"context"
	"errors"
	"testing"
	"time"

	"stairplatform/internal/application/audit"
)

// Тесты Forget (S-148, S-141 №13): право на забвение conversation-memory.

func TestForgetMemberPurges(t *testing.T) {
	mem := &fakeMemory{msgs: []MemoryMessage{
		{TenantID: "t1", ProjectID: "p1", Role: "user", Content: "m1", CreatedAt: time.Now()},
		{TenantID: "t1", ProjectID: "p1", Role: "assistant", Content: "m2", CreatedAt: time.Now()},
		{TenantID: "t1", ProjectID: "p-other", Role: "user", Content: "чужой", CreatedAt: time.Now()},
	}}
	svc := NewService(nil).
		WithMemory(mem).
		WithProjectAuthz(&fakeAuthz{members: map[string]bool{"t1/u1/p1": true}})

	n, err := svc.Forget(context.Background(), "t1", "u1", "p1")
	if err != nil {
		t.Fatalf("Forget member: %v", err)
	}
	if n != 2 {
		t.Fatalf("deleted = %d, want 2", n)
	}
	mem.mu.Lock()
	defer mem.mu.Unlock()
	if len(mem.msgs) != 1 || mem.msgs[0].ProjectID != "p-other" {
		t.Fatalf("foreign project messages must survive: %+v", mem.msgs)
	}
}

func TestForgetNonMemberForbidden(t *testing.T) {
	mem := &fakeMemory{msgs: []MemoryMessage{
		{TenantID: "t1", ProjectID: "p1", Role: "user", Content: "m1", CreatedAt: time.Now()},
	}}
	spy := &auditSpy{}
	svc := NewService(nil, audit.NewService(spy)).
		WithMemory(mem).
		WithProjectAuthz(&fakeAuthz{members: map[string]bool{}})

	if _, err := svc.Forget(context.Background(), "t1", "u-stranger", "p1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	mem.mu.Lock()
	defer mem.mu.Unlock()
	if len(mem.msgs) != 1 {
		t.Fatal("memory must not be touched on forbidden purge")
	}
	if len(spy.events) != 1 {
		t.Fatalf("denial must be audited, events = %d", len(spy.events))
	}
}

func TestForgetRequiresScopeAndMemory(t *testing.T) {
	svc := NewService(nil).
		WithMemory(&fakeMemory{}).
		WithProjectAuthz(&fakeAuthz{members: map[string]bool{"t1/u1/p1": true}})

	if _, err := svc.Forget(context.Background(), "t1", "u1", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty project: expected ErrInvalid, got %v", err)
	}
	bare := NewService(nil).WithProjectAuthz(&fakeAuthz{members: map[string]bool{"t1/u1/p1": true}})
	if _, err := bare.Forget(context.Background(), "t1", "u1", "p1"); err == nil {
		t.Fatal("memory-less service must fail closed")
	}
	noAuthz := NewService(nil).WithMemory(&fakeMemory{})
	if _, err := noAuthz.Forget(context.Background(), "t1", "u1", "p1"); err == nil {
		t.Fatal("authorizer-less service must fail closed")
	}
}
