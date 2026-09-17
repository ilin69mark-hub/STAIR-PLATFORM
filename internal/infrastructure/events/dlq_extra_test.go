package events

import (
	"context"
	"testing"

	domevents "stairplatform/internal/domain/events"
)

func dlqEntry() DLQEntry {
	return DLQEntry{
		Event: domevents.ProjectCreated{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
			},
		},
		HandlerID: "h1",
	}
}

func TestNewDLQDefaultCapacity(t *testing.T) {
	if q := NewDLQ(0); q.capacity != 1000 {
		t.Fatalf("NewDLQ(0) capacity = %d, want 1000", q.capacity)
	}
	if q := NewDLQ(-5); q.capacity != 1000 {
		t.Fatalf("NewDLQ(-5) capacity = %d, want 1000", q.capacity)
	}
}

func TestDLQClear(t *testing.T) {
	q := NewDLQ(4)
	q.Push(dlqEntry())
	q.Push(dlqEntry())
	q.Clear()
	if q.Len() != 0 {
		t.Fatalf("clear left %d entries", q.Len())
	}
	if _, ok := q.Pop(); ok {
		t.Fatal("pop after clear must return false")
	}
}

func TestDLQRetryAllEmpty(t *testing.T) {
	q := NewDLQ(4)
	bus := NewBus(WithRetryPolicy(RetryPolicy{MaxAttempts: 1}))
	if got := q.RetryAll(context.Background(), bus); got != 0 {
		t.Fatalf("empty retry returned %d successes", got)
	}
}

func TestDLQRetryAllCanceled(t *testing.T) {
	q := NewDLQ(4)
	q.Push(dlqEntry())
	q.Push(dlqEntry())
	bus := NewBus(WithRetryPolicy(RetryPolicy{MaxAttempts: 1}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := q.RetryAll(ctx, bus); got != 0 {
		t.Fatalf("canceled retry returned %d successes", got)
	}
	if q.Len() != 2 {
		t.Fatalf("canceled retry must keep entries, have %d", q.Len())
	}
}

func TestDLQRetryAllFailure(t *testing.T) {
	q := NewDLQ(4)
	q.Push(dlqEntry())
	bus := NewBus(WithRetryPolicy(RetryPolicy{MaxAttempts: 0}))
	if got := q.RetryAll(context.Background(), bus); got != 0 {
		t.Fatalf("failed retry returned %d successes", got)
	}
	if q.Len() != 1 {
		t.Fatalf("failed retry must keep entry, have %d", q.Len())
	}
	if e := q.Entries()[0]; e.RetryCount != 1 {
		t.Fatalf("retry count = %d, want 1", e.RetryCount)
	}
}
