package document

import (
	"context"
	"fmt"

	domevents "stairplatform/internal/domain/events"
	docdomain "stairplatform/internal/domain/document"
	"stairplatform/internal/domain/manufacturing"
)

// DocumentPipeline — стадия document в pipeline (BC-008).
// Генерирует комплект документов для валидной конфигурации.
type DocumentPipeline struct {
	engine *Engine
	events EventPublisher
}

// EventPublisher — интерфейс для публикации событий.
type EventPublisher interface {
	Publish(ctx context.Context, event domevents.Event)
}

// NewDocumentPipeline создаёт Document Pipeline.
func NewDocumentPipeline(engine *Engine, events EventPublisher) *DocumentPipeline {
	return &DocumentPipeline{
		engine: engine,
		events: events,
	}
}

// Execute выполняет стадию document pipeline.
func (p *DocumentPipeline) Execute(ctx context.Context, configID string, mfgPkg *manufacturing.ManufacturingPackage) error {
	if p.engine == nil {
		return fmt.Errorf("document: engine is required")
	}

	// Генерируем документы по умолчанию
	docIDs := []string{}

	// 1. Техническая спецификация
	doc, err := p.engine.GenerateFromManufacturing(
		configID,
		docdomain.TemplateID("tech-spec"),
		docdomain.FormatJSON,
		mfgPkg,
	)
	if err != nil {
		return fmt.Errorf("document: failed to generate tech spec: %v", err)
	}
	docIDs = append(docIDs, string(doc.ID))

	// 2. Спецификация материалов (BOM)
	doc, err = p.engine.GenerateFromManufacturing(
		configID,
		docdomain.TemplateID("bom-spec"),
		docdomain.FormatJSON,
		mfgPkg,
	)
	if err != nil {
		return fmt.Errorf("document: failed to generate BOM spec: %v", err)
	}
	docIDs = append(docIDs, string(doc.ID))

	// Публикуем событие
	if p.events != nil {
		p.events.Publish(ctx, DocumentGenerated{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(
					domevents.EventDocumentGenerated,
					"document-engine",
					"system",
					"",
					"",
					"",
				),
			},
			ConfigID: configID,
			DocCount: len(docIDs),
		})
	}

	return nil
}

// DocumentGenerated — событие генерации документа.
type DocumentGenerated struct {
	domevents.BaseEvent
	ConfigID string
	DocCount int
}

// EventType возвращает тип события.
func (e DocumentGenerated) EventType() domevents.EventType {
	return domevents.EventDocumentGenerated
}
