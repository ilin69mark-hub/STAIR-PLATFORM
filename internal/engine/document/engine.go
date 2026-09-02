// Package document реализует Document Engine (BC-008): генерация
// и рендеринг документов из шаблонов. Домен не зависит от HTTP,
// БД, ORM, UI и AI (ADR-0006).
package document

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"stairplatform/internal/domain/document"
	"stairplatform/internal/domain/manufacturing"
)

// Engine — Document Engine (BC-008).
// Генерирует документы из шаблонов, заполняя данными конфигурации.
type Engine struct {
	templates map[document.TemplateID]*template.Template
}

// NewEngine создаёт Document Engine с предзагруженными шаблонами.
func NewEngine() *Engine {
	return &Engine{
		templates: make(map[document.TemplateID]*template.Template),
	}
}

// RegisterTemplate регистрирует шаблон документа.
func (e *Engine) RegisterTemplate(tpl *document.Template, content string) error {
	if tpl == nil {
		return fmt.Errorf("document: template is required")
	}
	if err := tpl.Validate(); err != nil {
		return err
	}
	parsed, err := template.New(string(tpl.ID)).Parse(content)
	if err != nil {
		return fmt.Errorf("document: failed to parse template %q: %v", tpl.ID, err)
	}
	e.templates[tpl.ID] = parsed
	return nil
}

// RenderContext — контекст для рендеринга документа.
type RenderContext struct {
	ConfigID      string
	ProjectName   string
	Materials     []MaterialData
	Parts         []PartData
	BOM           []BOMLineData
	Pricing       *PricingData
	Manufacturing *ManufacturingData
	GeneratedAt   time.Time
}

// MaterialData — данные о материале для шаблона.
type MaterialData struct {
	Code      string
	Name      string
	Category  string
	Thickness float64
}

// PartData — данные о детали для шаблона.
type PartData struct {
	Number    string
	Kind      string
	Material  string
	Length    float64
	Width     float64
	Thickness float64
}

// BOMLineData — строка BOM для шаблона.
type BOMLineData struct {
	Number     int
	PartNumber string
	Quantity   int
	Material   string
}

// PricingData — данные о цене для шаблона.
type PricingData struct {
	MaterialCost float64
	LaborCost    float64
	TotalCost    float64
	Currency     string
}

// ManufacturingData — производственные данные для шаблона.
type ManufacturingData struct {
	TotalParts    int
	TotalSheets   int
	EstimatedTime float64 // минуты
}

// Render рендерит документ из шаблона (DOC-0004).
func (e *Engine) Render(doc *document.Document, ctx *RenderContext) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("document: document is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("document: render context is required")
	}
	tpl, ok := e.templates[doc.TemplateID]
	if !ok {
		return nil, fmt.Errorf("document: template %q not found", doc.TemplateID)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("document: template execution failed: %v", err)
	}
	return buf.Bytes(), nil
}

// GenerateDocument генерирует документ из данных конфигурации.
func (e *Engine) GenerateDocument(
	configID string,
	tplID document.TemplateID,
	format document.DocumentFormat,
	ctx *RenderContext,
) (*document.Document, error) {
	if ctx == nil {
		return nil, fmt.Errorf("document: render context is required")
	}

	doc := &document.Document{
		ID:         document.DocumentID(fmt.Sprintf("doc-%s-%d", configID, time.Now().UnixNano())),
		Type:       document.DocumentTypeTechnicalSpec,
		Format:     format,
		Status:     document.DocumentStatusDraft,
		TemplateID: tplID,
		ConfigID:   configID,
		Metadata: document.DocumentMetadata{
			Title:     fmt.Sprintf("Document for config %s", configID),
			Author:    "system",
			Version:   "1.0",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	content, err := e.Render(doc, ctx)
	if err != nil {
		return nil, err
	}
	doc.Content = content
	doc.Status = document.DocumentStatusRendered

	return doc, nil
}

// GenerateFromManufacturing генерирует документ из ManufacturingPackage.
func (e *Engine) GenerateFromManufacturing(
	configID string,
	tplID document.TemplateID,
	format document.DocumentFormat,
	pkg *manufacturing.ManufacturingPackage,
) (*document.Document, error) {
	if pkg == nil {
		return nil, fmt.Errorf("document: manufacturing package is required")
	}

	ctx := &RenderContext{
		ConfigID:    configID,
		GeneratedAt: time.Now(),
	}

	// Конвертируем Parts
	for _, p := range pkg.Parts {
		ctx.Parts = append(ctx.Parts, PartData{
			Number:    string(p.Number),
			Kind:      string(p.Kind),
			Material:  string(p.Material),
			Length:    p.Length.Millimeters(),
			Width:     p.Width.Millimeters(),
			Thickness: p.Thickness.Millimeters(),
		})
	}

	// Конвертируем BOM
	for _, line := range pkg.BOM.Lines {
		ctx.BOM = append(ctx.BOM, BOMLineData{
			Number:     line.Number,
			PartNumber: string(line.PartNumber),
			Quantity:   int(line.Quantity),
			Material:   string(line.MaterialCode),
		})
	}

	// Manufacturing summary
	if pkg.Nesting != nil {
		ctx.Manufacturing = &ManufacturingData{
			TotalParts:  int(pkg.Nesting.PartCount),
			TotalSheets: len(pkg.Nesting.Sheets),
		}
	}

	return e.GenerateDocument(configID, tplID, format, ctx)
}
