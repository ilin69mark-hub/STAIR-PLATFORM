package assistant

import (
	"context"
	"sync"
	"testing"
	"time"
)

// recordingBackend — primary, считающий вызовы (S-148).
type recordingBackend struct {
	mu    sync.Mutex
	calls int
	text  string
}

func (r *recordingBackend) Infer(context.Context, Prompt) (*Answer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return &Answer{Text: r.text}, nil
}

func (r *recordingBackend) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func TestBudgetAllowAndExhaust(t *testing.T) {
	b := NewBudget(2)
	if !b.Allow() || !b.Allow() {
		t.Fatal("first two calls must be allowed")
	}
	if b.Allow() {
		t.Fatal("third call must be denied (budget exhausted)")
	}
	if b.Used() != 2 {
		t.Fatalf("Used = %d, want 2", b.Used())
	}
}

func TestBudgetUnlimited(t *testing.T) {
	for _, b := range []*Budget{nil, NewBudget(0), NewBudget(-5)} {
		for i := 0; i < 10; i++ {
			if !b.Allow() {
				t.Fatalf("unlimited budget must always allow (iter %d)", i)
			}
		}
	}
}

func TestBudgetDailyRollover(t *testing.T) {
	day := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	b := NewBudget(1)
	b.now = func() time.Time { return day }
	if !b.Allow() {
		t.Fatal("first call must be allowed")
	}
	if b.Allow() {
		t.Fatal("second call same day must be denied")
	}
	day = day.Add(24 * time.Hour)
	if !b.Allow() {
		t.Fatal("next day must reset the budget")
	}
}

func TestRouterBudgetSkipsPrimary(t *testing.T) {
	// S-148 (S-141 №14): исчерпанный бюджет — primary НЕ вызывается,
	// ответ собирается локально (только local-бэкенд).
	primary := &recordingBackend{text: "from llm"}
	r := NewModelRouter(primary, localComment).WithBudget(NewBudget(1))
	resp := &Response{Recommendation: "Рекомендуется прямой марш"}
	it := &intent{Kind: "design", UserTask: "q", Context: "c"}

	if _, err := r.Infer(context.Background(), it, resp); err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if primary.count() != 1 {
		t.Fatalf("primary calls = %d, want 1", primary.count())
	}
	ans, err := r.Infer(context.Background(), it, resp)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if primary.count() != 1 {
		t.Fatalf("primary calls = %d, want 1 (budget exhausted — local only)", primary.count())
	}
	if ans.Text == "from llm" {
		t.Fatal("denied call must be served by local backend, not primary")
	}
}

func TestServiceWithBudgetOrderIndependent(t *testing.T) {
	// Порядок WithBudget/WithPrimaryBackend не важен — бюджет сохраняется.
	primary := &recordingBackend{text: "from llm"}
	a := NewService(nil).WithBudget(NewBudget(100)).WithPrimaryBackend(primary)
	if a.router.budget == nil || a.router.primary == nil {
		t.Fatal("budget and primary must both survive chaining")
	}
	b := NewService(nil).WithPrimaryBackend(primary).WithBudget(NewBudget(100))
	if b.router.budget == nil || b.router.primary == nil {
		t.Fatal("budget and primary must both survive reverse chaining")
	}
}
