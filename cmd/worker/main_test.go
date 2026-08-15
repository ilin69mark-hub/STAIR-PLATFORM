package main

import (
	"context"
	"testing"
	"time"

	"stairplatform/internal/infrastructure/queue"
)

// TestRegistryUnknownJobType: неизвестный тип задания → ошибка (retry).
func TestRegistryUnknownJobType(t *testing.T) {
	r := newRegistry(nil, nil, 90)
	job, err := queue.NewJob("unknown.type", nil)
	if err != nil {
		t.Fatalf("new job: %v", err)
	}
	if err := r.Handle(context.Background(), job); err == nil {
		t.Fatalf("expected error for unknown job type")
	}
}

// TestEnqueueCleanup: schedule ставит все три типа заданий очистки.
func TestEnqueueCleanup(t *testing.T) {
	q := queue.NewMemoryQueue()
	ctx := context.Background()

	enqueueCleanup(ctx, q)

	want := map[string]int{
		queue.JobCleanupSessions:  1,
		queue.JobCleanupSsoStates: 1,
		queue.JobCleanupAudit:     1,
	}
	for i := 0; i < len(want); i++ {
		job, ok, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("dequeue: %v", err)
		}
		if !ok {
			t.Fatalf("expected a cleanup job, queue empty after %d dequeues", i)
		}
		want[job.Type]--
	}
	for typ, n := range want {
		if n != 0 {
			t.Fatalf("type %q enqueued %d times, want 1", typ, n)
		}
	}
}

// TestProcessJobPermanentFailure: задание с попытками == max не реенкьюится
// и не паникует (окончательный отказ, EDR-0020 инвариант 3).
func TestProcessJobPermanentFailure(t *testing.T) {
	q := queue.NewMemoryQueue()
	ctx := context.Background()

	job := queue.Job{ID: "j1", Type: "unknown.type", Attempts: queue.DefaultMaxAttempts, MaxAttempts: queue.DefaultMaxAttempts, CreatedAt: time.Now().UTC()}
	// Не должен зависнуть: процесс просто залогирует permanent failure.
	done := make(chan struct{})
	go func() {
		r := newRegistry(nil, nil, 90)
		processJob(ctx, q, r, job)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("processJob hung on permanent failure")
	}
}

// TestConsumeEmptyQueue: consume не блокируется на пустой in-memory очереди
// (внешний ctx с таймаутом — consume выходит по cancel).
func TestConsumeStopsOnCancel(t *testing.T) {
	q := queue.NewMemoryQueue()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r := newRegistry(nil, nil, 90)
	done := make(chan struct{})
	go func() {
		consume(ctx, q, r)
		close(done)
	}()
	select {
	case <-done:
		// Ок: consume завершился по отмене контекста.
	case <-time.After(2 * time.Second):
		t.Fatalf("consume should stop on ctx cancel")
	}
}
