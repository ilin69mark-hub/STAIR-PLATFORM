package document

import (
	"testing"
	"time"
)

func TestDocumentValidation(t *testing.T) {
	doc := &Document{
		ID:         "doc-1",
		Type:       DocumentTypeTechnicalSpec,
		Format:     FormatJSON,
		Status:     DocumentStatusDraft,
		TemplateID: "tpl-1",
		ConfigID:   "config-1",
		Metadata: DocumentMetadata{
			Title: "Test Document",
		},
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDocumentValidationEmptyID(t *testing.T) {
	doc := &Document{
		Type:       DocumentTypeTechnicalSpec,
		Format:     FormatJSON,
		Status:     DocumentStatusDraft,
		TemplateID: "tpl-1",
		ConfigID:   "config-1",
		Metadata: DocumentMetadata{
			Title: "Test",
		},
	}
	if err := doc.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestDocumentValidationInvalidType(t *testing.T) {
	doc := &Document{
		ID:         "doc-1",
		Type:       "invalid",
		Format:     FormatJSON,
		Status:     DocumentStatusDraft,
		TemplateID: "tpl-1",
		ConfigID:   "config-1",
		Metadata: DocumentMetadata{
			Title: "Test",
		},
	}
	if err := doc.Validate(); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestDocumentTransitionToRendered(t *testing.T) {
	doc := &Document{
		ID:         "doc-1",
		Type:       DocumentTypeTechnicalSpec,
		Format:     FormatJSON,
		Status:     DocumentStatusDraft,
		TemplateID: "tpl-1",
		ConfigID:   "config-1",
		Metadata: DocumentMetadata{
			Title: "Test",
		},
	}
	if err := doc.TransitionTo(DocumentStatusRendered); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Status != DocumentStatusRendered {
		t.Fatalf("expected status %q, got %q", DocumentStatusRendered, doc.Status)
	}
}

func TestDocumentInvalidTransition(t *testing.T) {
	doc := &Document{
		ID:         "doc-1",
		Type:       DocumentTypeTechnicalSpec,
		Format:     FormatJSON,
		Status:     DocumentStatusArchived,
		TemplateID: "tpl-1",
		ConfigID:   "config-1",
		Metadata: DocumentMetadata{
			Title: "Test",
		},
	}
	if err := doc.TransitionTo(DocumentStatusDraft); err == nil {
		t.Fatal("expected error for invalid transition")
	}
}

func TestDocumentTypeIsValid(t *testing.T) {
	valid := []DocumentType{
		DocumentTypeTechnicalSpec,
		DocumentTypeDrawing,
		DocumentTypeLetter,
		DocumentTypeManufacturing,
		DocumentTypeAssembly,
		DocumentTypeCostEstimate,
	}
	for _, dt := range valid {
		if !dt.IsValid() {
			t.Errorf("expected type %q to be valid", dt)
		}
	}
	if DocumentType("invalid").IsValid() {
		t.Error("expected 'invalid' type to be invalid")
	}
}

func TestDocumentFormatIsValid(t *testing.T) {
	valid := []DocumentFormat{FormatPDF, FormatDOCX, FormatHTML, FormatJSON}
	for _, f := range valid {
		if !f.IsValid() {
			t.Errorf("expected format %q to be valid", f)
		}
	}
	if DocumentFormat("invalid").IsValid() {
		t.Error("expected 'invalid' format to be invalid")
	}
}

func TestDocumentStatusIsValid(t *testing.T) {
	valid := []DocumentStatus{DocumentStatusDraft, DocumentStatusRendered, DocumentStatusArchived}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("expected status %q to be valid", s)
		}
	}
	if DocumentStatus("invalid").IsValid() {
		t.Error("expected 'invalid' status to be invalid")
	}
}

func TestTemplateValidation(t *testing.T) {
	tpl := &Template{
		ID:          "tpl-1",
		Type:        DocumentTypeTechnicalSpec,
		Name:        "Technical Spec Template",
		ContentType: "application/json",
	}
	if err := tpl.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplateValidationEmptyID(t *testing.T) {
	tpl := &Template{
		Type:        DocumentTypeTechnicalSpec,
		Name:        "Test",
		ContentType: "application/json",
	}
	if err := tpl.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestDocumentPackageValidation(t *testing.T) {
	pkg := &DocumentPackage{
		ID:       "pkg-1",
		ConfigID: "config-1",
		Status:   PackageStatusPending,
		Documents: []*Document{
			{
				ID:         "doc-1",
				Type:       DocumentTypeTechnicalSpec,
				Format:     FormatJSON,
				Status:     DocumentStatusDraft,
				TemplateID: "tpl-1",
				ConfigID:   "config-1",
				Metadata: DocumentMetadata{
					Title: "Test",
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := pkg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDocumentPackageAddDocument(t *testing.T) {
	pkg := &DocumentPackage{
		ID:        "pkg-1",
		ConfigID:  "config-1",
		Status:    PackageStatusPending,
		Documents: []*Document{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	doc := &Document{
		ID:         "doc-1",
		Type:       DocumentTypeTechnicalSpec,
		Format:     FormatJSON,
		Status:     DocumentStatusDraft,
		TemplateID: "tpl-1",
		ConfigID:   "config-1",
		Metadata: DocumentMetadata{
			Title: "Test",
		},
	}
	if err := pkg.AddDocument(doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(pkg.Documents))
	}
}

func TestDocumentPackageFindDocument(t *testing.T) {
	pkg := &DocumentPackage{
		ID:       "pkg-1",
		ConfigID: "config-1",
		Status:   PackageStatusPending,
		Documents: []*Document{
			{
				ID:         "doc-1",
				Type:       DocumentTypeTechnicalSpec,
				Format:     FormatJSON,
				Status:     DocumentStatusDraft,
				TemplateID: "tpl-1",
				ConfigID:   "config-1",
				Metadata: DocumentMetadata{
					Title: "Test",
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	doc, ok := pkg.FindDocument("doc-1")
	if !ok || doc == nil {
		t.Fatal("expected to find document")
	}
	_, ok = pkg.FindDocument("doc-2")
	if ok {
		t.Fatal("expected not to find document")
	}
}

func TestDocumentPackageByType(t *testing.T) {
	pkg := &DocumentPackage{
		ID:       "pkg-1",
		ConfigID: "config-1",
		Status:   PackageStatusPending,
		Documents: []*Document{
			{
				ID:         "doc-1",
				Type:       DocumentTypeTechnicalSpec,
				Format:     FormatJSON,
				Status:     DocumentStatusDraft,
				TemplateID: "tpl-1",
				ConfigID:   "config-1",
				Metadata: DocumentMetadata{
					Title: "Spec",
				},
			},
			{
				ID:         "doc-2",
				Type:       DocumentTypeDrawing,
				Format:     FormatPDF,
				Status:     DocumentStatusDraft,
				TemplateID: "tpl-2",
				ConfigID:   "config-1",
				Metadata: DocumentMetadata{
					Title: "Drawing",
				},
			},
			{
				ID:         "doc-3",
				Type:       DocumentTypeTechnicalSpec,
				Format:     FormatJSON,
				Status:     DocumentStatusDraft,
				TemplateID: "tpl-1",
				ConfigID:   "config-1",
				Metadata: DocumentMetadata{
					Title: "Spec 2",
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	docs := pkg.DocumentsByType(DocumentTypeTechnicalSpec)
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
}
