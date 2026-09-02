package pricing

import (
	"context"

	domevents "stairplatform/internal/domain/events"
)

// EventPublisher — интерфейс для публикации событий.
type EventPublisher interface {
	Publish(ctx context.Context, event domevents.Event)
}

// PricingWithEvents — обёртка над Pricing с публикацией событий.
type PricingWithEvents struct {
	publisher EventPublisher
}

// NewPricingWithEvents создаёт Pricing с событиями.
func NewPricingWithEvents(publisher EventPublisher) *PricingWithEvents {
	return &PricingWithEvents{publisher: publisher}
}

// PublishPricingStarted публикует событие начала расчёта стоимости.
func (p *PricingWithEvents) PublishPricingStarted(ctx context.Context, configID string) {
	if p.publisher == nil {
		return
	}
	p.publisher.Publish(ctx, domevents.EngineFinished{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventEnginePricingFinished, "pricing", "system", "", "", ""),
		},
		Engine:   "pricing",
		ConfigID: configID,
		Success:  true,
	})
}

// PublishPriceCalculated публикует событие расчёта стоимости.
func (p *PricingWithEvents) PublishPriceCalculated(ctx context.Context, projectID string, finalPrice int64, currency string) {
	if p.publisher == nil {
		return
	}
	p.publisher.Publish(ctx, domevents.PriceCalculated{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventPriceCalculated, "pricing", "system", "", "", ""),
		},
		ProjectID:  projectID,
		FinalPrice: finalPrice,
		Currency:   currency,
	})
}

// PublishPricingFailed публикует событие ошибки расчёта стоимости.
func (p *PricingWithEvents) PublishPricingFailed(ctx context.Context, configID, errorMsg string) {
	if p.publisher == nil {
		return
	}
	p.publisher.Publish(ctx, domevents.EngineFinished{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventEnginePricingFinished, "pricing", "system", "", "", ""),
		},
		Engine:   "pricing",
		ConfigID: configID,
		Success:  false,
		ErrorMsg: errorMsg,
	})
}
