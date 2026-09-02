package events

import (
	"testing"

	domevents "stairplatform/internal/domain/events"
)

func TestStoreAppendAndLen(t *testing.T) {
	store := NewStore(0)

	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
		ProjectID: "proj-1",
	}

	store.Append(event)

	if store.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", store.Len())
	}
}

func TestStoreEventsFilter(t *testing.T) {
	store := NewStore(0)

	store.Append(domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	})
	store.Append(domevents.GeometryUpdated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventGeometryUpdated, "test", "user-1", "t1", "", ""),
		},
	})
	store.Append(domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	})

	events := store.Events(domevents.EventProjectCreated)
	if len(events) != 2 {
		t.Fatalf("expected 2 project events, got %d", len(events))
	}
}

func TestStoreCapacity(t *testing.T) {
	store := NewStore(3)

	for i := 0; i < 5; i++ {
		store.Append(domevents.ProjectCreated{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
			},
		})
	}

	if store.Len() != 3 {
		t.Fatalf("expected 3 entries (capacity), got %d", store.Len())
	}
}

func TestStoreReplaySince(t *testing.T) {
	store := NewStore(0)

	for i := 0; i < 5; i++ {
		store.Append(domevents.ProjectCreated{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
			},
		})
	}

	var replayed []StoreEntry
	store.ReplaySince(3, func(entry StoreEntry) bool {
		replayed = append(replayed, entry)
		return true
	})

	if len(replayed) != 2 {
		t.Fatalf("expected 2 entries replayed, got %d", len(replayed))
	}
}

func TestStoreReplayAll(t *testing.T) {
	store := NewStore(0)

	store.Append(domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	})
	store.Append(domevents.GeometryUpdated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventGeometryUpdated, "test", "user-1", "t1", "", ""),
		},
	})

	var count int
	store.ReplayAll(func(entry StoreEntry) bool {
		count++
		return true
	})

	if count != 2 {
		t.Fatalf("expected 2 entries replayed, got %d", count)
	}
}

func TestStoreClear(t *testing.T) {
	store := NewStore(0)

	store.Append(domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	})

	store.Clear()

	if store.Len() != 0 {
		t.Fatalf("expected 0 after clear, got %d", store.Len())
	}
}

func TestDLQPushAndPop(t *testing.T) {
	dlq := NewDLQ(10)

	entry := DLQEntry{
		Event: domevents.ProjectCreated{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
			},
		},
		HandlerID: "h1",
	}

	dlq.Push(entry)

	if dlq.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", dlq.Len())
	}

	popped, ok := dlq.Pop()
	if !ok {
		t.Fatal("Pop returned false")
	}
	if popped.HandlerID != "h1" {
		t.Fatalf("expected handler h1, got %s", popped.HandlerID)
	}
}

func TestDLQCapacity(t *testing.T) {
	dlq := NewDLQ(3)

	for i := 0; i < 5; i++ {
		dlq.Push(DLQEntry{
			Event: domevents.ProjectCreated{
				BaseEvent: domevents.BaseEvent{
					Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
				},
			},
			HandlerID: "h",
		})
	}

	if dlq.Len() != 3 {
		t.Fatalf("expected 3 entries (capacity), got %d", dlq.Len())
	}
}

func TestDLQPopEmpty(t *testing.T) {
	dlq := NewDLQ(10)

	_, ok := dlq.Pop()
	if ok {
		t.Fatal("Pop should return false on empty DLQ")
	}
}

func TestDLQEntries(t *testing.T) {
	dlq := NewDLQ(10)

	dlq.Push(DLQEntry{Event: domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	}, HandlerID: "h1"})
	dlq.Push(DLQEntry{Event: domevents.GeometryUpdated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventGeometryUpdated, "test", "user-1", "t1", "", ""),
		},
	}, HandlerID: "h2"})

	entries := dlq.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}
