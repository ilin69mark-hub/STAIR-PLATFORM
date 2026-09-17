package pricing

import (
	"context"
	"testing"

	domevents "stairplatform/internal/domain/events"
)

type fakePublisher struct{ events []domevents.Event }

func (f *fakePublisher) Publish(_ context.Context, e domevents.Event) { f.events = append(f.events, e) }

func TestPricingEventsPublish(t *testing.T) {
	p := &fakePublisher{}
	pw := NewPricingWithEvents(p)
	pw.PublishPricingStarted(context.Background(), "cfg-1")
	if len(p.events) != 1 {
		t.Fatalf("want 1, got %d", len(p.events))
	}
	pw.PublishPriceCalculated(context.Background(), "proj-1", 12345, "RUB")
	if len(p.events) != 2 {
		t.Fatalf("want 2, got %d", len(p.events))
	}
	pw.PublishPricingFailed(context.Background(), "cfg-2", "oops")
	if len(p.events) != 3 {
		t.Fatalf("want 3, got %d", len(p.events))
	}
}

func TestPricingEventsNilPublisher(t *testing.T) {
	pw := NewPricingWithEvents(nil)
	// should not panic
	pw.PublishPricingStarted(context.Background(), "cfg-1")
	pw.PublishPriceCalculated(context.Background(), "p1", 100, "RUB")
	pw.PublishPricingFailed(context.Background(), "cfg-1", "err")
}
