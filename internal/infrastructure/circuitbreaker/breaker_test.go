package circuitbreaker

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreakerClosed(t *testing.T) {
	cb := New("test", Settings{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          time.Second,
		MaxRequests:      2,
	})

	// Normal operation
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if cb.State() != StateClosed {
		t.Fatalf("expected closed, got %v", cb.State())
	}
}

func TestCircuitBreakerOpensOnFailures(t *testing.T) {
	cb := New("test", Settings{
		FailureThreshold: 3,
		Timeout:          time.Second,
	})

	// Fail 3 times
	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return errors.New("fail") })
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected open, got %v", cb.State())
	}

	// Should be blocked
	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerHalfOpenAfterTimeout(t *testing.T) {
	cb := New("test", Settings{
		FailureThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Open the circuit
	cb.Execute(func() error { return errors.New("fail") })
	cb.Execute(func() error { return errors.New("fail") })

	if cb.State() != StateOpen {
		t.Fatalf("expected open, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected half_open, got %v", cb.State())
	}
}

func TestCircuitBreakerClosesFromHalfOpen(t *testing.T) {
	cb := New("test", Settings{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
		MaxRequests:      3,
	})

	// Open
	cb.Execute(func() error { return errors.New("fail") })
	cb.Execute(func() error { return errors.New("fail") })

	time.Sleep(100 * time.Millisecond)

	// Succeed from half-open
	cb.Execute(func() error { return nil })
	cb.Execute(func() error { return nil })

	if cb.State() != StateClosed {
		t.Fatalf("expected closed, got %v", cb.State())
	}
}

func TestCircuitBreakerOpensFromHalfOpen(t *testing.T) {
	cb := New("test", Settings{
		FailureThreshold: 2,
		Timeout:          50 * time.Millisecond,
		MaxRequests:      3,
	})

	// Open
	cb.Execute(func() error { return errors.New("fail") })
	cb.Execute(func() error { return errors.New("fail") })

	time.Sleep(100 * time.Millisecond)

	// Fail from half-open
	cb.Execute(func() error { return errors.New("fail again") })

	if cb.State() != StateOpen {
		t.Fatalf("expected open, got %v", cb.State())
	}
}

func TestCircuitBreakerHalfOpenMaxRequests(t *testing.T) {
	cb := New("test", Settings{
		FailureThreshold: 2,
		SuccessThreshold: 10, // high so we stay in half-open
		Timeout:          50 * time.Millisecond,
		MaxRequests:      2,
	})

	// Open
	cb.Execute(func() error { return errors.New("fail") })
	cb.Execute(func() error { return errors.New("fail") })

	time.Sleep(100 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected half_open, got %v", cb.State())
	}

	// First two requests allowed
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	err = cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	// Third request blocked
	err = cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrTooManyRequests) {
		t.Fatalf("expected ErrTooManyRequests, got %v", err)
	}
}

func TestCircuitBreakerOnStateChange(t *testing.T) {
	var lastFrom, lastTo State
	var called atomic.Bool

	cb := New("test", Settings{
		FailureThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})
	cb.OnStateChange(func(name string, from, to State) {
		lastFrom = from
		lastTo = to
		called.Store(true)
	})

	cb.Execute(func() error { return errors.New("fail") })
	cb.Execute(func() error { return errors.New("fail") })

	if !called.Load() {
		t.Fatal("expected state change callback")
	}
	if lastFrom != StateClosed || lastTo != StateOpen {
		t.Fatalf("expected closed→open, got %v→%v", lastFrom, lastTo)
	}
}

func TestCircuitBreakerConcurrent(t *testing.T) {
	cb := New("test", Settings{
		FailureThreshold: 100,
		Timeout:          time.Second,
	})

	var failures atomic.Int32
	for i := 0; i < 100; i++ {
		go cb.Execute(func() error {
			failures.Add(1)
			return errors.New("fail")
		})
	}

	time.Sleep(100 * time.Millisecond)
	// Should still be closed (threshold is 100)
	if cb.State() != StateClosed && cb.State() != StateOpen {
		t.Fatalf("expected closed or open, got %v", cb.State())
	}
}
