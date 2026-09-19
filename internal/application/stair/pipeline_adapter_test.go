package stair

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
	domevents "stairplatform/internal/domain/events"
	engmfg "stairplatform/internal/engine/manufacturing"
	engprc "stairplatform/internal/engine/pricing"
	engsolver "stairplatform/internal/engine/solver"
	"stairplatform/internal/infrastructure/events"
)

func TestPipelineAdapterCreation(t *testing.T) {
	service := NewService()
	bus := events.NewBus()
	adapter := NewPipelineAdapter(service, &busAdapter{bus: bus})
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
}

func TestPipelineAdapterEventPublishing(t *testing.T) {
	service := NewService()
	bus := events.NewBus()
	adapter := NewPipelineAdapter(service, &busAdapter{bus: bus})

	var receivedEvents []domevents.EventType
	bus.Subscribe("*", func(ctx context.Context, e domevents.Event) error {
		receivedEvents = append(receivedEvents, e.EventType())
		return nil
	})

	adapter.bus.Publish(context.Background(), domevents.PipelineCompleted{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventPipelineCompleted, "test", "system", "", "", ""),
		},
		ConfigID: "test-config",
		Duration: 100,
	})

	if len(receivedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(receivedEvents))
	}
	if receivedEvents[0] != domevents.EventPipelineCompleted {
		t.Fatalf("expected PipelineCompleted event, got %v", receivedEvents[0])
	}
}

func TestSolverWithEvents(t *testing.T) {
	bus := events.NewBus()
	solver := engsolver.NewSolverWithEvents(&busAdapter{bus: bus})

	var receivedEvents []domevents.EventType
	bus.Subscribe("*", func(ctx context.Context, e domevents.Event) error {
		receivedEvents = append(receivedEvents, e.EventType())
		return nil
	})

	solver.PublishAnalysisStarted(context.Background(), "config-1", "straight")
	solver.PublishAnalysisCompleted(context.Background(), "config-1", "straight", 15, 33.5)

	if len(receivedEvents) != 2 {
		t.Fatalf("expected 2 events, got %d", len(receivedEvents))
	}
}

func TestManufacturingWithEvents(t *testing.T) {
	bus := events.NewBus()
	mfg := engmfg.NewManufacturingWithEvents(&busAdapter{bus: bus})

	var receivedEvents []domevents.EventType
	bus.Subscribe("*", func(ctx context.Context, e domevents.Event) error {
		receivedEvents = append(receivedEvents, e.EventType())
		return nil
	})

	mfg.PublishManufacturingStarted(context.Background(), "config-1")
	mfg.PublishManufacturingCompleted(context.Background(), "config-1", 10)

	if len(receivedEvents) != 2 {
		t.Fatalf("expected 2 events, got %d", len(receivedEvents))
	}
}

func TestPricingWithEvents(t *testing.T) {
	bus := events.NewBus()
	pricing := engprc.NewPricingWithEvents(&busAdapter{bus: bus})

	var receivedEvents []domevents.EventType
	bus.Subscribe("*", func(ctx context.Context, e domevents.Event) error {
		receivedEvents = append(receivedEvents, e.EventType())
		return nil
	})

	pricing.PublishPricingStarted(context.Background(), "config-1")
	pricing.PublishPriceCalculated(context.Background(), "config-1", 150000, "RUB")

	if len(receivedEvents) != 2 {
		t.Fatalf("expected 2 events, got %d", len(receivedEvents))
	}
}

func TestPipelineAdapterWithConfig(t *testing.T) {
	service := NewService()
	bus := events.NewBus()
	adapter := NewPipelineAdapter(service, &busAdapter{bus: bus})

	cfg := Config{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2700),
		Flight:            engineering.FlightStraight,
		StepHeight:        engineering.Length(180),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		Clearance:         engineering.Length(80),
		RailingHeight:     engineering.Length(900),
	}

	ctx := context.Background()
	result, err := adapter.RunPipelineWithResult(ctx, "config-1", cfg, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// busAdapter — адаптер для Bus, реализующий EventPublisher.
type busAdapter struct {
	bus *events.Bus
}

func (b *busAdapter) Publish(ctx context.Context, event domevents.Event) {
	if err := b.bus.Publish(ctx, event); err != nil {
		panic(err)
	}
}
