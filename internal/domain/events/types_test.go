package events

import (
	"testing"
)

func TestEventMetadataGeneration(t *testing.T) {
	meta := NewEventMetadata(EventProjectCreated, "test", "user-1", "t1", "", "")

	if meta.ID == "" {
		t.Fatal("expected non-empty event ID")
	}
	if meta.Type != EventProjectCreated {
		t.Fatalf("expected type %s, got %s", EventProjectCreated, meta.Type)
	}
	if meta.Source != "test" {
		t.Fatalf("expected source 'test', got %s", meta.Source)
	}
	if meta.Actor != "user-1" {
		t.Fatalf("expected actor 'user-1', got %s", meta.Actor)
	}
	if meta.Tenant != "t1" {
		t.Fatalf("expected tenant 't1', got %s", meta.Tenant)
	}
	if meta.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
	if meta.Version != CurrentVersion {
		t.Fatalf("expected version %d, got %d", CurrentVersion, meta.Version)
	}
}

func TestEventMetadataCorrelationID(t *testing.T) {
	correlationID := NewCorrelationID()
	meta := NewEventMetadata(EventProjectCreated, "test", "user-1", "t1", correlationID, "")

	if meta.CorrelationID != correlationID {
		t.Fatalf("expected correlation ID %s, got %s", correlationID, meta.CorrelationID)
	}
}

func TestEventMetadataCausationID(t *testing.T) {
	causationID := NewCorrelationID()
	meta := NewEventMetadata(EventProjectCreated, "test", "user-1", "t1", "", causationID)

	if meta.CausationID != causationID {
		t.Fatalf("expected causation ID %s, got %s", causationID, meta.CausationID)
	}
}

func TestNewCorrelationID(t *testing.T) {
	id1 := NewCorrelationID()
	id2 := NewCorrelationID()

	if id1 == "" || id2 == "" {
		t.Fatal("expected non-empty correlation IDs")
	}
	if id1 == id2 {
		t.Fatal("expected different correlation IDs")
	}
}

func TestBaseEventInterface(t *testing.T) {
	event := ProjectCreated{
		BaseEvent: BaseEvent{
			Meta: NewEventMetadata(EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
		ProjectID: "proj-1",
	}

	if event.EventType() != EventProjectCreated {
		t.Fatalf("expected event type %s, got %s", EventProjectCreated, event.EventType())
	}

	meta := event.Metadata()
	if meta.Source != "test" {
		t.Fatalf("expected source 'test', got %s", meta.Source)
	}
}

func TestTypedEventProjectCreated(t *testing.T) {
	event := ProjectCreated{
		BaseEvent: BaseEvent{
			Meta: NewEventMetadata(EventProjectCreated, "test", "user-1", "t1", "", ""),
		},
		ProjectID: "proj-1",
		Owner:     "user-1",
		Name:      "My Project",
	}

	if event.EventType() != EventProjectCreated {
		t.Fatalf("expected event type %s, got %s", EventProjectCreated, event.EventType())
	}
	if event.ProjectID != "proj-1" {
		t.Fatalf("expected project ID 'proj-1', got %s", event.ProjectID)
	}
}

func TestTypedEventGeometryUpdated(t *testing.T) {
	event := GeometryUpdated{
		BaseEvent: BaseEvent{
			Meta: NewEventMetadata(EventGeometryUpdated, "geometry", "user-1", "t1", "", ""),
		},
		GeometryID: "geo-1",
		FlightType: "straight",
	}

	if event.EventType() != EventGeometryUpdated {
		t.Fatalf("expected event type %s, got %s", EventGeometryUpdated, event.EventType())
	}
}

func TestTypedEventValidationPassed(t *testing.T) {
	event := ValidationPassed{
		BaseEvent: BaseEvent{
			Meta: NewEventMetadata(EventValidationPassed, "validation", "system", "t1", "", ""),
		},
		ConfigID: "config-1",
	}

	if event.EventType() != EventValidationPassed {
		t.Fatalf("expected event type %s, got %s", EventValidationPassed, event.EventType())
	}
}

func TestTypedEventAnalysisCompleted(t *testing.T) {
	event := AnalysisCompleted{
		BaseEvent: BaseEvent{
			Meta: NewEventMetadata(EventAnalysisCompleted, "solver", "system", "t1", "", ""),
		},
		ConfigID:  "config-1",
		Flight:    "straight",
		StepCount: 15,
		Angle:     33.5,
	}

	if event.EventType() != EventAnalysisCompleted {
		t.Fatalf("expected event type %s, got %s", EventAnalysisCompleted, event.EventType())
	}
}

func TestTypedEventPriceCalculated(t *testing.T) {
	event := PriceCalculated{
		BaseEvent: BaseEvent{
			Meta: NewEventMetadata(EventPriceCalculated, "pricing", "system", "t1", "", ""),
		},
		ProjectID:  "proj-1",
		FinalPrice: 150000,
		Currency:   "RUB",
	}

	if event.EventType() != EventPriceCalculated {
		t.Fatalf("expected event type %s, got %s", EventPriceCalculated, event.EventType())
	}
}

func TestTypedEventPipelineCompleted(t *testing.T) {
	event := PipelineCompleted{
		BaseEvent: BaseEvent{
			Meta: NewEventMetadata(EventPipelineCompleted, "pipeline", "system", "t1", "", ""),
		},
		ConfigID: "config-1",
		Duration: 1234.5,
	}

	if event.EventType() != EventPipelineCompleted {
		t.Fatalf("expected event type %s, got %s", EventPipelineCompleted, event.EventType())
	}
}
