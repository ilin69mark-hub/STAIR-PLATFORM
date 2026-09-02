package document

import (
	"context"
	"testing"

	domevents "stairplatform/internal/domain/events"
	docdomain "stairplatform/internal/domain/document"
	"stairplatform/internal/domain/manufacturing"
)

// mockEventPublisher для тестов pipeline.
type mockEventPublisher struct {
	published []domevents.Event
}

func (m *mockEventPublisher) Publish(_ context.Context, event domevents.Event) {
	m.published = append(m.published, event)
}

func TestDocumentPipeline_Execute(t *testing.T) {
	engine := NewEngine()

	// Регистрируем шаблоны
	tpl1 := &docdomain.Template{
		ID:          "tech-spec",
		Type:        docdomain.DocumentTypeTechnicalSpec,
		Name:        "Tech Spec",
		ContentType: "application/json",
	}
	tpl2 := &docdomain.Template{
		ID:          "bom-spec",
		Type:        docdomain.DocumentTypeManufacturing,
		Name:        "BOM Spec",
		ContentType: "application/json",
	}
	engine.RegisterTemplate(tpl1, `{"config": "{{.ConfigID}}"}`)
	engine.RegisterTemplate(tpl2, `{"config": "{{.ConfigID}}"}`)

	events := &mockEventPublisher{}
	pipeline := NewDocumentPipeline(engine, events)

	mfgPkg := &manufacturing.ManufacturingPackage{
		Parts: []manufacturing.Part{
			{
				Number:   "P-001",
				Kind:     manufacturing.PartTread,
				Material: "steel",
				Thickness: 10,
				Length:   1000,
				Width:    300,
			},
		},
		BOM: manufacturing.BOM{
			Lines: []manufacturing.BOMLine{
				{Number: 1, PartNumber: "P-001", Quantity: 1},
			},
		},
		Nesting: &manufacturing.NestingResult{
			PartCount: 1,
			Sheets: []manufacturing.SheetLayout{
				{Placed: []manufacturing.PlacedPart{{PartNumber: "P-001"}}},
			},
		},
	}

	err := pipeline.Execute(context.Background(), "config-1", mfgPkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events.published) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events.published))
	}

	event, ok := events.published[0].(DocumentGenerated)
	if !ok {
		t.Fatalf("expected DocumentGenerated event")
	}

	if event.ConfigID != "config-1" {
		t.Errorf("expected config-1, got %s", event.ConfigID)
	}

	if event.DocCount != 2 {
		t.Errorf("expected 2 docs, got %d", event.DocCount)
	}
}

func TestDocumentPipeline_NilEngine(t *testing.T) {
	events := &mockEventPublisher{}
	pipeline := NewDocumentPipeline(nil, events)

	err := pipeline.Execute(context.Background(), "config-1", nil)
	if err == nil {
		t.Fatal("expected error for nil engine")
	}
}

func TestDocumentPipeline_NilEvents(t *testing.T) {
	engine := NewEngine()

	tpl1 := &docdomain.Template{
		ID:          "tech-spec",
		Type:        docdomain.DocumentTypeTechnicalSpec,
		Name:        "Tech Spec",
		ContentType: "application/json",
	}
	tpl2 := &docdomain.Template{
		ID:          "bom-spec",
		Type:        docdomain.DocumentTypeManufacturing,
		Name:        "BOM Spec",
		ContentType: "application/json",
	}
	engine.RegisterTemplate(tpl1, `{"config": "{{.ConfigID}}"}`)
	engine.RegisterTemplate(tpl2, `{"config": "{{.ConfigID}}"}`)

	// nil events should not panic
	pipeline := NewDocumentPipeline(engine, nil)

	mfgPkg := &manufacturing.ManufacturingPackage{
		Parts: []manufacturing.Part{
			{
				Number:   "P-001",
				Kind:     manufacturing.PartTread,
				Material: "steel",
				Thickness: 10,
				Length:   1000,
				Width:    300,
			},
		},
		BOM: manufacturing.BOM{
			Lines: []manufacturing.BOMLine{
				{Number: 1, PartNumber: "P-001", Quantity: 1},
			},
		},
		Nesting: &manufacturing.NestingResult{
			PartCount: 1,
			Sheets: []manufacturing.SheetLayout{
				{Placed: []manufacturing.PlacedPart{{PartNumber: "P-001"}}},
			},
		},
	}

	err := pipeline.Execute(context.Background(), "config-1", mfgPkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDocumentPipeline_EventType(t *testing.T) {
	event := DocumentGenerated{
		ConfigID: "config-1",
		DocCount: 2,
	}

	if event.EventType() != domevents.EventDocumentGenerated {
		t.Errorf("expected EventDocumentGenerated, got %s", event.EventType())
	}
}
