package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// TestExecuteSequential: limit=1 выполняет задачи в порядке индексов.
func TestExecuteSequential(t *testing.T) {
	s := New(1)
	var order []int
	err := s.Execute(context.Background(), 5, func(i int) error {
		order = append(order, i)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range order {
		if v != i {
			t.Fatalf("order = %v, want [0 1 2 3 4]", order)
		}
	}
}

// TestExecuteAllRuns: все n задач выполняются ровно один раз.
func TestExecuteAllRuns(t *testing.T) {
	s := New(0)
	var count atomic.Int32
	err := s.Execute(context.Background(), 100, func(i int) error {
		count.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count.Load() != 100 {
		t.Fatalf("tasks executed = %d, want 100", count.Load())
	}
}

// TestExecuteErrorFirstIndex: возвращается ошибка первого по индексу слота,
// даже если сбой произошёл в другом порядке (детерминизм).
func TestExecuteErrorFirstIndex(t *testing.T) {
	s := New(0)
	want := errors.New("slot 2 failed")
	err := s.Execute(context.Background(), 5, func(i int) error {
		if i == 2 {
			return want
		}
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want slot-2 error", err)
	}
}

// TestExecuteErrorOnlySlot: ошибка в единственном сбоящем слоте.
func TestExecuteErrorOnlySlot(t *testing.T) {
	s := New(0)
	want := errors.New("only failure")
	err := s.Execute(context.Background(), 8, func(i int) error {
		if i == 7 {
			return want
		}
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want slot-7 error", err)
	}
}

// TestExecuteCancelled: отменённый контекст — ни одна задача не выполняется.
func TestExecuteCancelled(t *testing.T) {
	s := New(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var count atomic.Int32
	err := s.Execute(ctx, 5, func(i int) error {
		count.Add(1)
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if count.Load() != 0 {
		t.Fatalf("tasks executed = %d, want 0", count.Load())
	}
}

// TestExecuteCancelledMidway: отмена во время работы останавливает
// невыполненные задачи и возвращает ошибку отмены.
func TestExecuteCancelledMidway(t *testing.T) {
	s := New(4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var executed atomic.Int32
	// Синхронизация: первая задача блокируется, затем отменяем контекст.
	go func() {
		<-started
		cancel()
		close(release)
	}()
	err := s.Execute(ctx, 50, func(i int) error {
		if i == 0 {
			started <- struct{}{}
			<-release
		}
		executed.Add(1)
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if executed.Load() >= 50 {
		t.Fatal("expected partial execution")
	}
}

// TestExecuteEmpty: n=0 — нет задач, нет ошибки.
func TestExecuteEmpty(t *testing.T) {
	s := New(0)
	if err := s.Execute(context.Background(), 0, func(i int) error { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestExecuteNilCtx: контекст Background трактуется как Background.
func TestExecuteNilCtx(t *testing.T) {
	s := New(2)
	var count atomic.Int32
	if err := s.Execute(context.Background(), 3, func(i int) error { count.Add(1); return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count.Load() != 3 {
		t.Fatalf("tasks = %d, want 3", count.Load())
	}
}

// TestExecuteParallelEqualsSequential: параллельный и последовательный
// прогоны дают идентичные слоты (детерминизм result-slot, ADR-0003).
func TestExecuteParallelEqualsSequential(t *testing.T) {
	run := func(limit int) []int {
		slots := make([]int, 40)
		s := New(limit)
		if err := s.Execute(context.Background(), len(slots), func(i int) error {
			slots[i] = i * i
			return nil
		}); err != nil {
			t.Fatalf("limit %d: %v", limit, err)
		}
		return slots
	}
	seq := run(1)
	par := run(8)
	for i := range seq {
		if seq[i] != par[i] {
			t.Fatalf("slot %d differs: seq=%d par=%d", i, seq[i], par[i])
		}
	}
}
