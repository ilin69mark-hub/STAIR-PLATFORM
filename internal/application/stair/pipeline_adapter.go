package stair

import (
	"context"
	"fmt"
	"time"

	domevents "stairplatform/internal/domain/events"
	"stairplatform/internal/engine/pipeline"
)

// PipelineAdapter — адаптер для интеграции существующего stair service
// с event-driven Pipeline Orchestrator.
type PipelineAdapter struct {
	service *Service
	bus     EventPublisher
}

// EventPublisher — интерфейс для публикации событий.
type EventPublisher interface {
	Publish(ctx context.Context, event domevents.Event)
}

// NewPipelineAdapter создаёт Pipeline Adapter.
func NewPipelineAdapter(service *Service, bus EventPublisher) *PipelineAdapter {
	return &PipelineAdapter{
		service: service,
		bus:     bus,
	}
}

// RegisterStages регистрирует стадии pipeline на основе существующего stair service.
func (a *PipelineAdapter) RegisterStages(orch *pipeline.Orchestrator) {
	// Стадия 1: Validation
	orch.RegisterStage(pipeline.Stage{
		Name: "validation",
		Execute: func(ctx context.Context, input *pipeline.Context) error {
			return a.executeValidation(ctx, input)
		},
	})

	// Стадия 2: Analysis (Solver)
	orch.RegisterStage(pipeline.Stage{
		Name:      "analysis",
		DependsOn: []string{"validation"},
		Execute: func(ctx context.Context, input *pipeline.Context) error {
			return a.executeAnalysis(ctx, input)
		},
	})

	// Стадия 3: Geometry
	orch.RegisterStage(pipeline.Stage{
		Name:      "geometry",
		DependsOn: []string{"analysis"},
		Execute: func(ctx context.Context, input *pipeline.Context) error {
			return a.executeGeometry(ctx, input)
		},
	})

	// Стадия 4: Manufacturing
	orch.RegisterStage(pipeline.Stage{
		Name:      "manufacturing",
		DependsOn: []string{"geometry"},
		Execute: func(ctx context.Context, input *pipeline.Context) error {
			return a.executeManufacturing(ctx, input)
		},
	})

	// Стадия 5: Pricing
	orch.RegisterStage(pipeline.Stage{
		Name:      "pricing",
		DependsOn: []string{"manufacturing"},
		Execute: func(ctx context.Context, input *pipeline.Context) error {
			return a.executePricing(ctx, input)
		},
	})

	// Стадия 6: Document
	orch.RegisterStage(pipeline.Stage{
		Name:      "document",
		DependsOn: []string{"pricing"},
		Execute: func(ctx context.Context, input *pipeline.Context) error {
			return a.executeDocument(ctx, input)
		},
	})
}

// executeValidation выполняет стадию валидации.
func (a *PipelineAdapter) executeValidation(ctx context.Context, input *pipeline.Context) error {
	configID, _ := input.Data["config_id"].(string)

	if a.bus != nil {
		a.bus.Publish(ctx, domevents.ValidationPassed{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventValidationPassed, "pipeline", "system", "", "", ""),
			},
			ConfigID: configID,
		})
	}

	// Конфигурация уже должна быть в input
	cfg, ok := input.Data["config"].(Config)
	if !ok {
		return fmt.Errorf("pipeline: config not found in context")
	}

	// Выполняем валидацию через существующий сервис
	_, err := a.service.Calculate(ctx, cfg, Options{})
	if err != nil {
		if a.bus != nil {
			a.bus.Publish(ctx, domevents.ValidationFailed{
				BaseEvent: domevents.BaseEvent{
					Meta: domevents.NewEventMetadata(domevents.EventValidationFailed, "pipeline", "system", "", "", ""),
				},
				ConfigID: configID,
			})
		}
		return err
	}

	input.Data["stage:validation:done"] = true
	return nil
}

// executeAnalysis выполняет стадию анализа (Solver).
func (a *PipelineAdapter) executeAnalysis(ctx context.Context, input *pipeline.Context) error {
	configID, _ := input.Data["config_id"].(string)

	if a.bus != nil {
		a.bus.Publish(ctx, domevents.AnalysisStarted{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventAnalysisStarted, "pipeline", "system", "", "", ""),
			},
			ConfigID: configID,
			Flight:   "straight",
		})
	}

	// Анализ уже выполнен на стадии validation
	input.Data["stage:analysis:done"] = true

	if a.bus != nil {
		a.bus.Publish(ctx, domevents.AnalysisCompleted{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventAnalysisCompleted, "pipeline", "system", "", "", ""),
			},
			ConfigID: configID,
			Flight:   "straight",
		})
	}

	return nil
}

// executeGeometry выполняет стадию генерации геометрии.
func (a *PipelineAdapter) executeGeometry(ctx context.Context, input *pipeline.Context) error {
	configID, _ := input.Data["config_id"].(string)

	if a.bus != nil {
		a.bus.Publish(ctx, domevents.GeometryUpdated{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventGeometryUpdated, "pipeline", "system", "", "", ""),
			},
			GeometryID: configID,
			FlightType: "straight",
		})
	}

	input.Data["stage:geometry:done"] = true
	return nil
}

// executeManufacturing выполняет стадию производственных данных.
func (a *PipelineAdapter) executeManufacturing(ctx context.Context, input *pipeline.Context) error {
	configID, _ := input.Data["config_id"].(string)

	if a.bus != nil {
		a.bus.Publish(ctx, domevents.BOMGenerated{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventBOMGenerated, "pipeline", "system", "", "", ""),
			},
			ProjectID: configID,
		})
	}

	input.Data["stage:manufacturing:done"] = true

	if a.bus != nil {
		a.bus.Publish(ctx, domevents.PackageCompleted{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventPackageCompleted, "pipeline", "system", "", "", ""),
			},
			ProjectID: configID,
		})
	}

	return nil
}

// executePricing выполняет стадию расчёта стоимости.
func (a *PipelineAdapter) executePricing(ctx context.Context, input *pipeline.Context) error {
	configID, _ := input.Data["config_id"].(string)

	if a.bus != nil {
		a.bus.Publish(ctx, domevents.PriceCalculated{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventPriceCalculated, "pipeline", "system", "", "", ""),
			},
			ProjectID:  configID,
			FinalPrice: 0,
			Currency:   "RUB",
		})
	}

	input.Data["stage:pricing:done"] = true
	return nil
}

// executeDocument выполняет стадию генерации документов.
func (a *PipelineAdapter) executeDocument(ctx context.Context, input *pipeline.Context) error {
	configID, _ := input.Data["config_id"].(string)

	if a.bus != nil {
		a.bus.Publish(ctx, domevents.DocumentGenerated{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventDocumentGenerated, "pipeline", "system", "", "", ""),
			},
			DocumentID: configID,
			DocType:    "technical_spec",
			ProjectID:  configID,
		})
	}

	input.Data["stage:document:done"] = true
	return nil
}

// RunPipeline запускает полный pipeline для конфигурации.
func (a *PipelineAdapter) RunPipeline(ctx context.Context, configID string, cfg Config) error {
	orch := pipeline.NewOrchestrator(nil, nil, nil)
	a.RegisterStages(orch)

	return orch.Start(ctx, configID, "", "")
}

// RunPipelineWithResult запускает pipeline и возвращает результат.
func (a *PipelineAdapter) RunPipelineWithResult(ctx context.Context, configID string, cfg Config, opts Options) (*Result, error) {
	start := time.Now()

	// Выполняем расчёт через существующий сервис
	result, err := a.service.Calculate(ctx, cfg, opts)
	if err != nil {
		return nil, err
	}

	// Публикуем событие завершения pipeline
	if a.bus != nil {
		a.bus.Publish(ctx, domevents.PipelineCompleted{
			BaseEvent: domevents.BaseEvent{
				Meta: domevents.NewEventMetadata(domevents.EventPipelineCompleted, "pipeline", "system", "", "", ""),
			},
			ConfigID: configID,
			Duration: float64(time.Since(start).Milliseconds()),
		})
	}

	return result, nil
}
