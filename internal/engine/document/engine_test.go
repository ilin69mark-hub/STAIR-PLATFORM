package document

import (
	"testing"
	"time"

	docdomain "stairplatform/internal/domain/document"
	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/domain/manufacturing"
)

func TestEngineRender(t *testing.T) {
	engine := NewEngine()

	tpl := &docdomain.Template{
		ID:          "test-tpl",
		Type:        docdomain.DocumentTypeTechnicalSpec,
		Name:        "Test Template",
		ContentType: "application/json",
	}
	content := `{"config": "{{.ConfigID}}", "parts": {{len .Parts}}}`
	if err := engine.RegisterTemplate(tpl, content); err != nil {
		t.Fatalf("failed to register template: %v", err)
	}

	doc := &docdomain.Document{
		ID:         "doc-1",
		Type:       docdomain.DocumentTypeTechnicalSpec,
		Format:     docdomain.FormatJSON,
		Status:     docdomain.DocumentStatusDraft,
		TemplateID: "test-tpl",
		ConfigID:   "config-1",
		Metadata: docdomain.DocumentMetadata{
			Title: "Test",
		},
	}

	ctx := &RenderContext{
		ConfigID: "config-1",
		Parts: []PartData{
			{Number: "P-001", Kind: "tread", Material: "steel"},
		},
		GeneratedAt: time.Now(),
	}

	result, err := engine.Render(doc, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestEngineRenderMissingTemplate(t *testing.T) {
	engine := NewEngine()

	doc := &docdomain.Document{
		ID:         "doc-1",
		Type:       docdomain.DocumentTypeTechnicalSpec,
		Format:     docdomain.FormatJSON,
		Status:     docdomain.DocumentStatusDraft,
		TemplateID: "missing-tpl",
		ConfigID:   "config-1",
		Metadata: docdomain.DocumentMetadata{
			Title: "Test",
		},
	}

	ctx := &RenderContext{
		ConfigID: "config-1",
	}

	_, err := engine.Render(doc, ctx)
	if err == nil {
		t.Fatal("expected error for missing template")
	}
}

func TestEngineGenerateDocument(t *testing.T) {
	engine := NewEngine()

	tpl := &docdomain.Template{
		ID:          "tech-spec",
		Type:        docdomain.DocumentTypeTechnicalSpec,
		Name:        "Tech Spec",
		ContentType: "application/json",
	}
	content := `{"config": "{{.ConfigID}}"}`
	if err := engine.RegisterTemplate(tpl, content); err != nil {
		t.Fatalf("failed to register template: %v", err)
	}

	ctx := &RenderContext{
		ConfigID:    "config-1",
		GeneratedAt: time.Now(),
	}

	doc, err := engine.GenerateDocument("config-1", "tech-spec", docdomain.FormatJSON, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Status != docdomain.DocumentStatusRendered {
		t.Fatalf("expected status %q, got %q", docdomain.DocumentStatusRendered, doc.Status)
	}
	if len(doc.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
}

func TestEngineGenerateFromManufacturing(t *testing.T) {
	engine := NewEngine()

	tpl := &docdomain.Template{
		ID:          "tech-spec",
		Type:        docdomain.DocumentTypeTechnicalSpec,
		Name:        "Tech Spec",
		ContentType: "application/json",
	}
	content := `{"config": "{{.ConfigID}}", "parts": {{len .Parts}}}`
	if err := engine.RegisterTemplate(tpl, content); err != nil {
		t.Fatalf("failed to register template: %v", err)
	}

	pkg := &manufacturing.ManufacturingPackage{
		Parts: []manufacturing.Part{
			{
				Number:     "P-001",
				Kind:       manufacturing.PartTread,
				Material:   "steel",
				Thickness:  engineering.Length(10),
				Length:     engineering.Length(300),
				Width:      engineering.Length(250),
				SolidIndex: 0,
			},
		},
		BOM: manufacturing.BOM{
			Lines: []manufacturing.BOMLine{
				{
					Number:       1,
					PartNumber:   "P-001",
					Quantity:     1,
					MaterialCode: "steel",
				},
			},
		},
		Nesting: &manufacturing.NestingResult{
			PartCount: 1,
			Sheets:    []manufacturing.SheetLayout{},
		},
	}

	doc, err := engine.GenerateFromManufacturing("config-1", "tech-spec", docdomain.FormatJSON, pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.ConfigID != "config-1" {
		t.Fatalf("expected config_id 'config-1', got %q", doc.ConfigID)
	}
}

func TestEngineRegisterTemplateInvalid(t *testing.T) {
	engine := NewEngine()

	tpl := &docdomain.Template{
		Type:        "invalid",
		Name:        "Test",
		ContentType: "application/json",
	}
	if err := engine.RegisterTemplate(tpl, "test"); err == nil {
		t.Fatal("expected error for invalid template")
	}
}
