package events

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domevents "stairplatform/internal/domain/events"
)

func TestBusSubscribeAndPublish(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var received domevents.Event
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		received = e
		return nil
	})

	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
		ProjectID: "proj-1",
		Owner:     "user-1",
	}

	if err := bus.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if received == nil {
		t.Fatal("handler was not called")
	}
	if received.EventType() != domevents.EventProjectCreated {
		t.Fatalf("expected event type %s, got %s", domevents.EventProjectCreated, received.EventType())
	}
}

func TestBusMultipleHandlers(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var count atomic.Int32
	bus.Subscribe(domevents.EventGeometryUpdated, func(ctx context.Context, e domevents.Event) error {
		count.Add(1)
		return nil
	})
	bus.Subscribe(domevents.EventGeometryUpdated, func(ctx context.Context, e domevents.Event) error {
		count.Add(1)
		return nil
	})

	event := domevents.GeometryUpdated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventGeometryUpdated, "test", "user-1", "t1", "", ""),
		},
		GeometryID: "geo-1",
	}

	if err := bus.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if count.Load() != 2 {
		t.Fatalf("expected 2 handler calls, got %d", count.Load())
	}
}

func TestBusWildcardSubscription(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var received domevents.Event
	bus.Subscribe("*", func(ctx context.Context, e domevents.Event) error {
		received = e
		return nil
	})

	event := domevents.PriceCalculated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventPriceCalculated, "test", "user-1", "t1", "", ""),
		},
		ProjectID:  "proj-1",
		FinalPrice: 100000,
		Currency:   "RUB",
	}

	if err := bus.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if received == nil {
		t.Fatal("wildcard handler was not called")
	}
}

func TestBusUnsubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var count atomic.Int32
	id := bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		count.Add(1)
		return nil
	})

	if !bus.Unsubscribe(domevents.EventProjectCreated, id) {
		t.Fatal("unsubscribe returned false")
	}

	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	}

	_ = bus.Publish(context.Background(), event)

	if count.Load() != 0 {
		t.Fatalf("handler should not be called after unsubscribe, got %d", count.Load())
	}
}

func TestBusRetryOnFailure(t *testing.T) {
	bus := NewBus(WithRetryPolicy(RetryPolicy{
		MaxAttempts: 3,
		MinDelay:    10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		Multiplier:  2.0,
	}))
	defer bus.Close()

	var attempts atomic.Int32
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		attempts.Add(1)
		return fmt.Errorf("handler error")
	})

	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	}

	_ = bus.Publish(context.Background(), event)

	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestBusDLQ(t *testing.T) {
	bus := NewBus(
		WithRetryPolicy(RetryPolicy{
			MaxAttempts: 2,
			MinDelay:    5 * time.Millisecond,
			MaxDelay:    50 * time.Millisecond,
			Multiplier:  2.0,
		}),
		WithDLQ(100),
	)
	defer bus.Close()

	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		return fmt.Errorf("always fail")
	})

	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	}

	_ = bus.Publish(context.Background(), event)

	if bus.DLQCount() != 1 {
		t.Fatalf("expected 1 DLQ entry, got %d", bus.DLQCount())
	}
}

func TestBusPublishAsync(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var received atomic.Bool
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		received.Store(true)
		return nil
	})

	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	}

	bus.PublishAsync(context.Background(), event)

	// Ждём асинхронного выполнения.
	time.Sleep(50 * time.Millisecond)
	if !received.Load() {
		t.Fatal("async handler was not called")
	}
}

func TestBusClosedRejects(t *testing.T) {
	bus := NewBus()
	bus.Close()

	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	}

	err := bus.Publish(context.Background(), event)
	if err == nil {
		t.Fatal("expected error on closed bus")
	}
}

func TestBusContextCancellation(t *testing.T) {
	bus := NewBus(WithRetryPolicy(RetryPolicy{
		MaxAttempts: 5,
		MinDelay:    100 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		Multiplier:  2.0,
	}))
	defer bus.Close()

	var attempts atomic.Int32
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		attempts.Add(1)
		return fmt.Errorf("fail")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	}

	_ = bus.Publish(ctx, event)

	// Должен остановиться до 5 попыток из-за таймаута контекста.
	if attempts.Load() >= 5 {
		t.Fatalf("should have stopped before 5 attempts due to context cancellation, got %d", attempts.Load())
	}
}

func TestBusPriorityOrder(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var order []int
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		order = append(order, 2)
		return nil
	}, 2)
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		order = append(order, 1)
		return nil
	}, 1)
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		order = append(order, 3)
		return nil
	}, 3)

	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
	}

	_ = bus.Publish(context.Background(), event)

	expected := []int{1, 2, 3}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("expected order %v, got %v", expected, order)
		}
	}
}

func TestBusConcurrentPublish(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var count atomic.Int32
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		count.Add(1)
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := domevents.ProjectCreated{
				BaseEvent: domevents.BaseEvent{
					Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", "", ""),
				},
			}
			_ = bus.Publish(context.Background(), event)
		}()
	}
	wg.Wait()

	if count.Load() != 100 {
		t.Fatalf("expected 100 handler calls, got %d", count.Load())
	}
}

func TestBusCorrelationID(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var receivedCorrelationID string
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error {
		receivedCorrelationID = e.Metadata().CorrelationID
		return nil
	})

	correlationID := domevents.NewCorrelationID()
	event := domevents.ProjectCreated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventProjectCreated, "test", "user-1", "t1", correlationID, ""),
		},
	}

	_ = bus.Publish(context.Background(), event)

	if receivedCorrelationID != correlationID {
		t.Fatalf("expected correlation ID %s, got %s", correlationID, receivedCorrelationID)
	}
}

func TestBusHandlerCount(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	if bus.HandlerCount(domevents.EventProjectCreated) != 0 {
		t.Fatal("expected 0 handlers")
	}

	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error { return nil })
	bus.Subscribe(domevents.EventProjectCreated, func(ctx context.Context, e domevents.Event) error { return nil })

	if bus.HandlerCount(domevents.EventProjectCreated) != 2 {
		t.Fatalf("expected 2 handlers, got %d", bus.HandlerCount(domevents.EventProjectCreated))
	}
}
