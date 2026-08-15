package queue

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

func TestMemoryQueueFIFO(t *testing.T) {
	q := NewMemoryQueue()
	ctx := context.Background()

	j1 := mustJob(t, "typ.one", map[string]string{"a": "1"})
	j2 := mustJob(t, "typ.two", nil)

	if err := q.Enqueue(ctx, j1); err != nil {
		t.Fatalf("enqueue j1: %v", err)
	}
	if err := q.Enqueue(ctx, j2); err != nil {
		t.Fatalf("enqueue j2: %v", err)
	}

	got1, ok, err := q.Dequeue(ctx)
	if err != nil || !ok {
		t.Fatalf("dequeue first: ok=%v err=%v", ok, err)
	}
	if got1.ID != j1.ID || got1.Type != "typ.one" {
		t.Fatalf("expected first job %s (typ.one), got %s (%s)", j1.ID, got1.ID, got1.Type)
	}

	got2, ok, err := q.Dequeue(ctx)
	if err != nil || !ok {
		t.Fatalf("dequeue second: ok=%v err=%v", ok, err)
	}
	if got2.ID != j2.ID || got2.Type != "typ.two" {
		t.Fatalf("expected second job %s (typ.two), got %s (%s)", j2.ID, got2.ID, got2.Type)
	}

	// Пустая очередь → ok=false.
	_, ok, err = q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue empty err: %v", err)
	}
	if ok {
		t.Fatalf("expected empty queue, got ok=true")
	}
}

func TestMemoryQueueConcurrent(t *testing.T) {
	q := NewMemoryQueue()
	ctx := context.Background()

	const n = 200
	var wg sync.WaitGroup
	// Производители.
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < n/4; i++ {
				j := mustJob(t, "typ.concurrent", map[string]int{"i": i, "w": id})
				if err := q.Enqueue(ctx, j); err != nil {
					t.Errorf("enqueue: %v", err)
					return
				}
			}
		}(worker)
	}
	// Потребитель считает задания.
	go func() {
		count := 0
		for count < n {
			_, ok, err := q.Dequeue(ctx)
			if err != nil {
				t.Errorf("dequeue: %v", err)
				return
			}
			if ok {
				count++
			}
		}
	}()
	wg.Wait()
}

func TestJobRoundTrip(t *testing.T) {
	j := mustJob(t, "typ.round", map[string]int{"x": 42})
	if j.MaxAttempts != DefaultMaxAttempts {
		t.Fatalf("default max attempts = %d, want %d", j.MaxAttempts, DefaultMaxAttempts)
	}
	b, err := j.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := UnmarshalJob(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != j.ID || got.Type != j.Type {
		t.Fatalf("round trip mismatch: %+v vs %+v", got, j)
	}
	var payload map[string]int
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if payload["x"] != 42 {
		t.Fatalf("payload x=%v, want 42", payload["x"])
	}
}

func TestNewJobInvalid(t *testing.T) {
	if _, err := NewJob("", nil); err == nil {
		t.Fatalf("expected error for empty type")
	}
}

func TestUnmarshalJobInvalid(t *testing.T) {
	if _, err := UnmarshalJob([]byte(`{}`)); err == nil {
		t.Fatalf("expected error for empty job")
	}
	if _, err := UnmarshalJob([]byte(`not json`)); err == nil {
		t.Fatalf("expected error for invalid json")
	}
}

func mustJob(t *testing.T, typ string, payload any) Job {
	t.Helper()
	j, err := NewJob(typ, payload)
	if err != nil {
		t.Fatalf("new job %s: %v", typ, err)
	}
	return j
}
