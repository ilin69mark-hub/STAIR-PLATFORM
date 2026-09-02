package manufacturing

import (
	"context"

	domevents "stairplatform/internal/domain/events"
)

// EventPublisher — интерфейс для публикации событий.
type EventPublisher interface {
	Publish(ctx context.Context, event domevents.Event)
}

// ManufacturingWithEvents — обёртка над Manufacturing с публикацией событий.
type ManufacturingWithEvents struct {
	publisher EventPublisher
}

// NewManufacturingWithEvents создаёт Manufacturing с событиями.
func NewManufacturingWithEvents(publisher EventPublisher) *ManufacturingWithEvents {
	return &ManufacturingWithEvents{publisher: publisher}
}

// PublishManufacturingStarted публикует событие начала генерации данных.
func (m *ManufacturingWithEvents) PublishManufacturingStarted(ctx context.Context, configID string) {
	if m.publisher == nil {
		return
	}
	m.publisher.Publish(ctx, domevents.BOMGenerated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventBOMGenerated, "manufacturing", "system", "", "", ""),
		},
		ProjectID: configID,
	})
}

// PublishManufacturingCompleted публикует событие завершения генерации данных.
func (m *ManufacturingWithEvents) PublishManufacturingCompleted(ctx context.Context, configID string, partCount int) {
	if m.publisher == nil {
		return
	}
	m.publisher.Publish(ctx, domevents.PackageCompleted{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventPackageCompleted, "manufacturing", "system", "", "", ""),
		},
		ProjectID:  configID,
		PartCount:  partCount,
		SheetCount: 0,
	})
}

// PublishManufacturingFailed публикует событие ошибки генерации данных.
func (m *ManufacturingWithEvents) PublishManufacturingFailed(ctx context.Context, configID, errorMsg string) {
	if m.publisher == nil {
		return
	}
	m.publisher.Publish(ctx, domevents.EngineFinished{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventEngineManufacturingFinished, "manufacturing", "system", "", "", ""),
		},
		Engine:   "manufacturing",
		ConfigID: configID,
		Success:  false,
		ErrorMsg: errorMsg,
	})
}
