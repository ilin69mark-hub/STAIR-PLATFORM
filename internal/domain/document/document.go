// Package document реализует EDM для Document Context (BC-008).
// Документы — это output pipeline: спецификации, чертежи, письма для
// rms и CNC-пакеты. Домен не зависит от HTTP, БД, ORM, UI и AI (ADR-0006).
// Единицы: мм/кг по ADR-0008.
package document

import (
	"fmt"
	"time"
)

// DocumentID — уникальный идентификатор документа.
type DocumentID string

// DocumentType — тип документа (DOC-0002).
type DocumentType string

const (
	DocumentTypeTechnicalSpec DocumentType = "technical_spec"
	DocumentTypeDrawing       DocumentType = "drawing"
	DocumentTypeLetter        DocumentType = "letter"
	DocumentTypeManufacturing DocumentType = "manufacturing"
	DocumentTypeAssembly      DocumentType = "assembly"
	DocumentTypeCostEstimate  DocumentType = "cost_estimate"
)

// IsValid проверяет, что тип документа известен.
func (t DocumentType) IsValid() bool {
	switch t {
	case DocumentTypeTechnicalSpec, DocumentTypeDrawing, DocumentTypeLetter,
		DocumentTypeManufacturing, DocumentTypeAssembly, DocumentTypeCostEstimate:
		return true
	}
	return false
}

// DocumentFormat — формат вывода документа.
type DocumentFormat string

const (
	FormatPDF  DocumentFormat = "pdf"
	FormatDOCX DocumentFormat = "docx"
	FormatHTML DocumentFormat = "html"
	FormatJSON DocumentFormat = "json"
)

// IsValid проверяет, что формат документа известен.
func (f DocumentFormat) IsValid() bool {
	switch f {
	case FormatPDF, FormatDOCX, FormatHTML, FormatJSON:
		return true
	}
	return false
}

// TemplateID — уникальный идентификатор шаблона документа.
type TemplateID string

// DocumentStatus — статус документа (DOC-0003).
type DocumentStatus string

const (
	DocumentStatusDraft    DocumentStatus = "draft"
	DocumentStatusRendered DocumentStatus = "rendered"
	DocumentStatusArchived DocumentStatus = "archived"
)

// IsValid проверяет, что статус документа известен.
func (s DocumentStatus) IsValid() bool {
	switch s {
	case DocumentStatusDraft, DocumentStatusRendered, DocumentStatusArchived:
		return true
	}
	return false
}

// DocumentMetadata — метаданные документа.
type DocumentMetadata struct {
	Title       string
	Author      string
	Version     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ContentType string
}

// Document — единица хранения документа (DOC-0001).
// Содержит метаданные, шаблон, данные для рендеринга и контент.
type Document struct {
	ID         DocumentID
	Type       DocumentType
	Format     DocumentFormat
	Status     DocumentStatus
	TemplateID TemplateID
	ConfigID   string // ID конфигурации лестницы
	Metadata   DocumentMetadata
	Data       map[string]interface{} // данные для рендеринга шаблона
	Content    []byte                 // сгенерированный контент
}

// Validate проверяет инварианты документа (DOC-0001).
func (d *Document) Validate() error {
	if d == nil {
		return fmt.Errorf("document: document is required")
	}
	if d.ID == "" {
		return fmt.Errorf("document: id is required")
	}
	if !d.Type.IsValid() {
		return fmt.Errorf("document: invalid type %q", d.Type)
	}
	if !d.Format.IsValid() {
		return fmt.Errorf("document: invalid format %q", d.Format)
	}
	if !d.Status.IsValid() {
		return fmt.Errorf("document: invalid status %q", d.Status)
	}
	if d.TemplateID == "" {
		return fmt.Errorf("document: template_id is required")
	}
	if d.ConfigID == "" {
		return fmt.Errorf("document: config_id is required")
	}
	if d.Metadata.Title == "" {
		return fmt.Errorf("document: metadata.title is required")
	}
	return nil
}

// TransitionTo выполняет переход статуса документа.
func (d *Document) TransitionTo(newStatus DocumentStatus) error {
	if d == nil {
		return fmt.Errorf("document: document is required")
	}
	if !newStatus.IsValid() {
		return fmt.Errorf("document: invalid target status %q", newStatus)
	}
	if !d.canTransitionTo(newStatus) {
		return fmt.Errorf("document: cannot transition from %q to %q", d.Status, newStatus)
	}
	d.Status = newStatus
	d.Metadata.UpdatedAt = time.Now()
	return nil
}

func (d *Document) canTransitionTo(newStatus DocumentStatus) bool {
	switch d.Status {
	case DocumentStatusDraft:
		return newStatus == DocumentStatusRendered
	case DocumentStatusRendered:
		return newStatus == DocumentStatusArchived
	case DocumentStatusArchived:
		return false
	}
	return false
}

// Template — описание шаблона документа (DOC-0002).
type Template struct {
	ID          TemplateID
	Type        DocumentType
	Name        string
	ContentType string // MIME-тип выходного контента
	Fields      []TemplateField
}

// TemplateField — описание поля шаблона.
type TemplateField struct {
	Name        string
	Type        string // "string", "number", "date", "table"
	Required    bool
	Description string
}

// Validate проверяет инварианты шаблона.
func (t *Template) Validate() error {
	if t == nil {
		return fmt.Errorf("document: template is required")
	}
	if t.ID == "" {
		return fmt.Errorf("document: template id is required")
	}
	if !t.Type.IsValid() {
		return fmt.Errorf("document: template has invalid type %q", t.Type)
	}
	if t.Name == "" {
		return fmt.Errorf("document: template name is required")
	}
	if t.ContentType == "" {
		return fmt.Errorf("document: template content_type is required")
	}
	return nil
}
