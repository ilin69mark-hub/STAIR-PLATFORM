package order

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeRepo struct {
	created   *Order
	get       *Order
	byUser    []*Order
	all       []*Order
	updated   bool
	getErr    error
	updateErr error
}

func (f *fakeRepo) Create(_ context.Context, o *Order) error {
	o.ID = "ord-1"
	f.created = o
	return nil
}
func (f *fakeRepo) Get(_ context.Context, tenantID, id string) (*Order, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.get, nil
}
func (f *fakeRepo) ListByUser(_ context.Context, tenantID, userID string) ([]*Order, error) {
	return f.byUser, nil
}
func (f *fakeRepo) ListAll(_ context.Context, tenantID string) ([]*Order, error) {
	return f.all, nil
}
func (f *fakeRepo) UpdateStatus(_ context.Context, tenantID, id string, s Status) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = true
	return nil
}

func sampleJSON() json.RawMessage { return json.RawMessage(`{"width_mm":900}`) }

func TestCreateValid(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	o, err := svc.Create(context.Background(), "t-1", "u-1",
		Contact{Name: "Иван", Email: "i@ex.ru", Phone: "+7"}, sampleJSON(), sampleJSON())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.created == nil {
		t.Fatal("repo.Create not called")
	}
	if o.Status != StatusNew {
		t.Fatalf("status = %s, want new", o.Status)
	}
	if o.TenantID != "t-1" || o.UserID != "u-1" {
		t.Fatalf("tenant/user mismatch: %+v", o)
	}
}

func TestCreateRequiresUser(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if _, err := svc.Create(context.Background(), "t-1", "", Contact{}, nil, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for empty user, got %v", err)
	}
}

func TestCreateRequiresContact(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if _, err := svc.Create(context.Background(), "t-1", "u-1", Contact{}, sampleJSON(), sampleJSON()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for empty contact, got %v", err)
	}
}

func TestCreateRequiresSnapshots(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.Create(context.Background(), "t-1", "u-1",
		Contact{Name: "Иван", Email: "i@ex.ru"}, json.RawMessage{}, sampleJSON())
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for empty config, got %v", err)
	}
}

func TestUpdateStatusValid(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.UpdateStatus(context.Background(), "t-1", "ord-1", StatusConfirmed); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !repo.updated {
		t.Fatal("repo.UpdateStatus not called")
	}
}

func TestUpdateStatusInvalid(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if err := svc.UpdateStatus(context.Background(), "t-1", "ord-1", Status("bogus")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestStatusIsValid(t *testing.T) {
	for _, s := range AllStatuses {
		if !s.IsValid() {
			t.Errorf("%s must be valid", s)
		}
	}
	if Status("bogus").IsValid() {
		t.Error("bogus must be invalid")
	}
}

func TestListByUserAndAll(t *testing.T) {
	repo := &fakeRepo{byUser: []*Order{{ID: "o1"}}, all: []*Order{{ID: "o2"}}}
	svc := NewService(repo)
	u, err := svc.ListByUser(context.Background(), "t-1", "u-1")
	if err != nil || len(u) != 1 {
		t.Fatalf("list by user: %v %d", err, len(u))
	}
	a, err := svc.ListAll(context.Background(), "t-1")
	if err != nil || len(a) != 1 {
		t.Fatalf("list all: %v %d", err, len(a))
	}
}
