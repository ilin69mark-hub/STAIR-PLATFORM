package document

import "testing"

func TestDocumentRenderedToDraftBlocked(t *testing.T) {
	d := &Document{ID: "d1", Type: DocumentTypeTechnicalSpec, Format: FormatPDF, Status: DocumentStatusRendered, TemplateID: "tpl-1", ConfigID: "cfg-1", Metadata: DocumentMetadata{Title: "t"}}
	if err := d.TransitionTo(DocumentStatusDraft); err == nil {
		t.Fatal("want error for Rendered->Draft")
	}
}

func TestPackageDuplicateAndFind(t *testing.T) {
	pkg := &DocumentPackage{ID: "p1", ConfigID: "cfg1", Status: PackageStatusPending}
	d1 := &Document{ID: "d1", Type: DocumentTypeDrawing, Format: FormatPDF, Status: DocumentStatusDraft, TemplateID: "tpl-1", ConfigID: "cfg1", Metadata: DocumentMetadata{Title: "t"}}
	if err := pkg.AddDocument(d1); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := pkg.AddDocument(d1); err == nil {
		t.Fatal("want duplicate error")
	}
	if _, ok := pkg.FindDocument("missing"); ok {
		t.Fatal("want not found")
	}
	if got := pkg.DocumentsByType(DocumentTypeDrawing); len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
}
