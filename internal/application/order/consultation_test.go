package order

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestCreateConsultationValid(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	o, err := svc.CreateConsultation(context.Background(), "t-1", Contact{Name: "Иван", Email: "i@ex.ru"}, "Сколько стоит?")
	if err != nil {
		t.Fatalf("create consultation: %v", err)
	}
	if o.Kind != KindConsultation {
		t.Fatalf("kind=%s want consultation", o.Kind)
	}
	if o.Status != StatusNew {
		t.Fatalf("status=%s want new", o.Status)
	}
	if repo.created == nil || repo.created.ID != "ord-1" {
		t.Fatal("repo.Create not called")
	}
	var cfg map[string]string
	if err := json.Unmarshal(o.ConfigJSON, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg["question"] != "Сколько стоит?" {
		t.Fatalf("question=%q", cfg["question"])
	}
}

func TestCreateConsultationRequiresTenant(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.CreateConsultation(context.Background(), "", Contact{Name: "a", Email: "b@ex.ru"}, "q")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}

func TestCreateConsultationRequiresContact(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.CreateConsultation(context.Background(), "t-1", Contact{}, "q")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}

func TestCreateConsultationRequiresQuestion(t *testing.T) {
	svc := NewService(&fakeRepo{})
	for _, q := range []string{"", "   "} {
		_, err := svc.CreateConsultation(context.Background(), "t-1", Contact{Name: "a", Email: "b@ex.ru"}, q)
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("want ErrInvalid for question %q, got %v", q, err)
		}
	}
}

func TestCreateConsultationTrimsQuestion(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	o, _ := svc.CreateConsultation(context.Background(), "t-1", Contact{Name: "a", Email: "b@ex.ru"}, "  hi  ")
	var cfg map[string]string
	_ = json.Unmarshal(o.ConfigJSON, &cfg)
	if cfg["question"] != "hi" {
		t.Fatalf("got %q", cfg["question"])
	}
}

func TestKindIsValid(t *testing.T) {
	if !KindOrder.IsValid() || !KindConsultation.IsValid() {
		t.Fatal("known kinds must be valid")
	}
	if Kind("bogus").IsValid() {
		t.Fatal("bogus must be invalid")
	}
}

func TestGetFoundAndNotFound(t *testing.T) {
	want := &Order{ID: "o1"}
	svc := NewService(&fakeRepo{get: want})
	got, err := svc.Get(context.Background(), "t-1", "o1")
	if err != nil || got != want {
		t.Fatalf("got %v %v", got, err)
	}
	svc2 := NewService(&fakeRepo{getErr: ErrNotFound})
	if _, err := svc2.Get(context.Background(), "t-1", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
