package document

import (
	"fmt"
	"time"
)

// PackageID — уникальный идентификатор комплекта документов.
type PackageID string

// PackageStatus — статус комплекта документов.
type PackageStatus string

const (
	PackageStatusPending   PackageStatus = "pending"
	PackageStatusGenerated PackageStatus = "generated"
	PackageStatusDelivered PackageStatus = "delivered"
)

// IsValid проверяет, что статус комплекта известен.
func (s PackageStatus) IsValid() bool {
	switch s {
	case PackageStatusPending, PackageStatusGenerated, PackageStatusDelivered:
		return true
	}
	return false
}

// DocumentPackage — aggregate root Document (BC-008): комплект
// документов для одной конфигурации лестницы.
type DocumentPackage struct {
	ID        PackageID
	ConfigID  string
	Status    PackageStatus
	Documents []*Document
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate проверяет инварианты комплекта документов.
func (p *DocumentPackage) Validate() error {
	if p == nil {
		return fmt.Errorf("document: package is required")
	}
	if p.ID == "" {
		return fmt.Errorf("document: package id is required")
	}
	if p.ConfigID == "" {
		return fmt.Errorf("document: package config_id is required")
	}
	if !p.Status.IsValid() {
		return fmt.Errorf("document: package has invalid status %q", p.Status)
	}
	if len(p.Documents) == 0 {
		return fmt.Errorf("document: package has no documents")
	}
	for i, doc := range p.Documents {
		if err := doc.Validate(); err != nil {
			return fmt.Errorf("document: package document %d: %v", i, err)
		}
	}
	return nil
}

// AddDocument добавляет документ в комплект.
func (p *DocumentPackage) AddDocument(doc *Document) error {
	if p == nil {
		return fmt.Errorf("document: package is required")
	}
	if doc == nil {
		return fmt.Errorf("document: document is required")
	}
	if err := doc.Validate(); err != nil {
		return err
	}
	for _, existing := range p.Documents {
		if existing.ID == doc.ID {
			return fmt.Errorf("document: duplicate document id %q", doc.ID)
		}
	}
	p.Documents = append(p.Documents, doc)
	p.UpdatedAt = time.Now()
	return nil
}

// FindDocument ищет документ по ID.
func (p *DocumentPackage) FindDocument(id DocumentID) (*Document, bool) {
	if p == nil {
		return nil, false
	}
	for _, doc := range p.Documents {
		if doc.ID == id {
			return doc, true
		}
	}
	return nil, false
}

// DocumentsByType возвращает документы указанного типа.
func (p *DocumentPackage) DocumentsByType(dt DocumentType) []*Document {
	if p == nil {
		return nil
	}
	var result []*Document
	for _, doc := range p.Documents {
		if doc.Type == dt {
			result = append(result, doc)
		}
	}
	return result
}
