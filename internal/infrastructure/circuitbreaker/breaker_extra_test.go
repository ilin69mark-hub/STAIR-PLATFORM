package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestStateString(t *testing.T) {
	if StateClosed.String() != "closed" || StateOpen.String() != "open" || StateHalfOpen.String() != "half_open" {
		t.Fatal("state strings mismatch")
	}
	if got := State(99).String(); got != "unknown" {
		t.Fatalf("unknown state string = %q", got)
	}
}

func TestCircuitBreakerCounts(t *testing.T) {
	cb := New("counts", Settings{FailureThreshold: 5, SuccessThreshold: 10})
	if err := cb.Execute(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := cb.Execute(func() error { return errors.New("boom") }); err == nil {
		t.Fatal("err from fn must propagate")
	}
	f, s := cb.Counts()
	if f != 1 || s != 1 {
		t.Fatalf("counts = failures %d, successes %d, want 1,1", f, s)
	}
}

func TestCircuitBreakerClosedSuccessResetsFailures(t *testing.T) {
	cb := New("reset", Settings{FailureThreshold: 5, SuccessThreshold: 2})
	_ = cb.Execute(func() error { return errors.New("f") })
	_ = cb.Execute(func() error { return nil })
	_ = cb.Execute(func() error { return nil })
	_ = cb.Execute(func() error { return nil })
	f, _ := cb.Counts()
	if f != 0 {
		t.Fatalf("failure count = %d, want 0 after successes", f)
	}
}

func TestSetStateNoOp(t *testing.T) {
	cb := New("noop", DefaultSettings)
	cb.setState(StateClosed)
	if cb.State() != StateClosed {
		t.Fatalf("state = %v, want closed", cb.State())
	}
}

func TestExecuteWithRetrySuccess(t *testing.T) {
	cb := New("retry-ok", Settings{FailureThreshold: 3, Timeout: time.Second, MaxRequests: 2})
	if err := cb.ExecuteWithRetry(func() error { return nil }, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteWithRetryGenericError(t *testing.T) {
	cb := New("retry-gen", Settings{FailureThreshold: 3, Timeout: time.Second, MaxRequests: 2})
	want := errors.New("permanent")
	if err := cb.ExecuteWithRetry(func() error { return want }, 3); !errors.Is(err, want) {
		t.Fatalf("generic error must be returned as-is, got %v", err)
	}
}

func TestExecuteWithRetryErrCircuitOpen(t *testing.T) {
	cb := New("retry-open", Settings{FailureThreshold: 2, Timeout: 0, MaxRequests: 2, SuccessThreshold: 5})
	_ = cb.Execute(func() error { return errors.New("f") })
	_ = cb.Execute(func() error { return errors.New("f") })
	cb.lastFailureTime = time.Now().Add(time.Hour)
	err := cb.ExecuteWithRetry(func() error { return errors.New("never called") }, 2)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestExecuteWithRetryTooManyRequests(t *testing.T) {
	cb := New("retry-tm", Settings{FailureThreshold: 2, SuccessThreshold: 10, Timeout: 0, MaxRequests: 1})
	_ = cb.Execute(func() error { return errors.New("f") })
	_ = cb.Execute(func() error { return errors.New("f") })
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected half_open, got %v", cb.State())
	}
	if err := cb.Execute(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	err := cb.ExecuteWithRetry(func() error { return errors.New("never called") }, 0)
	if !errors.Is(err, ErrTooManyRequests) {
		t.Fatalf("expected ErrTooManyRequests, got %v", err)
	}
}

func TestCircuitBreakerFullCycle(t *testing.T) {
	cb := New("cycle", Settings{FailureThreshold: 2, SuccessThreshold: 2, Timeout: 0, MaxRequests: 3})
	_ = cb.Execute(func() error { return errors.New("f") })
	_ = cb.Execute(func() error { return errors.New("f") })
	cb.lastFailureTime = time.Now().Add(time.Hour)
	if cb.State() != StateOpen {
		t.Fatalf("want open, got %v", cb.State())
	}
	cb.lastFailureTime = time.Now().Add(-time.Hour)
	if cb.State() != StateHalfOpen {
		t.Fatalf("open must lapse to half_open, got %v", cb.State())
	}
	_ = cb.Execute(func() error { return errors.New("f") })
	cb.lastFailureTime = time.Now().Add(time.Hour)
	if cb.State() != StateOpen {
		t.Fatalf("half_open failure must re-open, got %v", cb.State())
	}
	cb.lastFailureTime = time.Now().Add(-time.Hour)
	if cb.State() != StateHalfOpen {
		t.Fatalf("open must lapse to half_open again, got %v", cb.State())
	}
	if err := cb.Execute(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := cb.Execute(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if cb.State() != StateClosed {
		t.Fatalf("half_open successes must close, got %v", cb.State())
	}
}
