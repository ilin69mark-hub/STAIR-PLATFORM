// Package pipeline реализует Event-Driven Pipeline Orchestrator (ARCH-0012).
// Оркестрирует переход между стадиями конвейера через Event Bus:
// список Engine Finished → триггер следующего engine → PipelineCompleted/Failed.
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	domevents "stairplatform/internal/domain/events"
	"stairplatform/internal/infrastructure/events"
)

// Stage — этап pipeline с именем и функцией выполнения.
type Stage struct {
	Name      string
	Execute   func(ctx context.Context, input *Context) error
	DependsOn []string // имена стадий, от которых зависит эта
}

// Context — общий контекст pipeline, передаётся между стадиями.
type Context struct {
	CorrelationID string
	ConfigID      string
	Actor         string
	Tenant        string
	Data          map[string]any // произвольные данные для передачи между стадиями
	Error         error
	FailedStage   string
}

// Orchestrator — event-driven pipeline orchestrator.
// Управляет порядком выполнения стадий, pub/sub событиями, timeline.
type Orchestrator struct {
	bus    *events.Bus
	store  *events.Store
	stages map[string]*Stage
	order  []string // топологический порядок стадий
	logger *slog.Logger
	mu     sync.RWMutex
	active map[string]*Context // активные pipeline по ConfigID
}

// NewOrchestrator создаёт Pipeline Orchestrator.
func NewOrchestrator(bus *events.Bus, store *events.Store, logger *slog.Logger) *Orchestrator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Orchestrator{
		bus:    bus,
		store:  store,
		stages: make(map[string]*Stage),
		logger: logger,
		active: make(map[string]*Context),
	}
}

// RegisterStage регистрирует стадию pipeline.
func (o *Orchestrator) RegisterStage(stage Stage) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.stages[stage.Name] = &stage
	o.order = append(o.order, stage.Name)
}

// Start запускает pipeline для ConfigID.
// Выполняет все стадии в топологическом порядке, публикуя engine events.
func (o *Orchestrator) Start(ctx context.Context, configID, actor, tenant string) error {
	correlationID := domevents.NewCorrelationID()

	o.mu.Lock()
	if _, exists := o.active[configID]; exists {
		o.mu.Unlock()
		return fmt.Errorf("pipeline: already running for config %s", configID)
	}

	pctx := &Context{
		CorrelationID: correlationID,
		ConfigID:      configID,
		Actor:         actor,
		Tenant:        tenant,
		Data:          make(map[string]any),
	}
	o.active[configID] = pctx
	o.mu.Unlock()

	o.logger.Info("pipeline started",
		"config_id", configID,
		"correlation_id", correlationID,
		"stages", len(o.order),
	)

	start := time.Now()

	// Выполняем стадии последовательно (для event-driven можно заменить на
	// подписку на engine events, но для MVP — seq с pub engine events).
	for _, stageName := range o.order {
		if ctx.Err() != nil {
			o.failPipeline(pctx, stageName, "context cancelled", time.Since(start))
			return ctx.Err()
		}

		stage := o.stages[stageName]
		o.logger.Debug("pipeline: executing stage",
			"stage", stageName,
			"config_id", configID,
		)

		stageStart := time.Now()

		// Публикуем Engine Started event (аналогично AnalysisStarted).
		o.publishEngineEvent(ctx, stageName, configID, correlationID, true, "")

		// Проверяем зависимости.
		if !o.dependenciesSatisfied(stageName, pctx) {
			err := fmt.Errorf("pipeline: dependencies not satisfied for stage %s", stageName)
			o.failPipeline(pctx, stageName, err.Error(), time.Since(start))
			return err
		}

		// Выполняем стадию.
		if err := stage.Execute(ctx, pctx); err != nil {
			o.publishEngineEvent(ctx, stageName, configID, correlationID, false, err.Error())
			o.failPipeline(pctx, stageName, err.Error(), time.Since(start))
			return fmt.Errorf("pipeline: stage %s failed: %w", stageName, err)
		}

		duration := time.Since(stageStart).Seconds() * 1000
		o.publishEngineEvent(ctx, stageName, configID, correlationID, false, "")
		_ = duration

		o.logger.Debug("pipeline: stage completed",
			"stage", stageName,
			"config_id", configID,
			"duration_ms", duration,
		)
	}

	totalDuration := time.Since(start).Seconds() * 1000

	// Публикуем PipelineCompleted.
	completed := domevents.PipelineCompleted{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(
				domevents.EventPipelineCompleted,
				"pipeline",
				actor,
				tenant,
				correlationID,
				"",
			),
		},
		ConfigID: configID,
		Duration: totalDuration,
	}
	_ = o.bus.Publish(ctx, completed)
	o.store.Append(completed)

	o.mu.Lock()
	delete(o.active, configID)
	o.mu.Unlock()

	o.logger.Info("pipeline completed",
		"config_id", configID,
		"correlation_id", correlationID,
		"duration_ms", totalDuration,
	)

	return nil
}

func (o *Orchestrator) dependenciesSatisfied(stageName string, pctx *Context) bool {
	stage := o.stages[stageName]
	for _, dep := range stage.DependsOn {
		if _, ok := o.stages[dep]; !ok {
			return false
		}
		// Проверяем что зависимая стадия уже выполнена (есть в Data).
		if _, done := pctx.Data["stage:"+dep+":done"]; !done {
			return false
		}
	}
	return true
}

func (o *Orchestrator) publishEngineEvent(ctx context.Context, stage, configID, correlationID string, started bool, errMsg string) {
	eventType := domevents.EventType("engine." + stage + "_finished")
	if started {
		eventType = domevents.EventType("engine." + stage + "_started")
	}

	event := domevents.EngineFinished{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(
				eventType,
				"pipeline",
				"system",
				"",
				correlationID,
				"",
			),
		},
		Engine:   stage,
		ConfigID: configID,
		Success:  errMsg == "",
		ErrorMsg: errMsg,
	}
	o.bus.PublishAsync(ctx, event)
	o.store.Append(event)
}

func (o *Orchestrator) failPipeline(pctx *Context, stage, errMsg string, duration time.Duration) {
	o.mu.Lock()
	pctx.Error = fmt.Errorf("pipeline failed at stage %s: %s", stage, errMsg)
	pctx.FailedStage = stage
	delete(o.active, pctx.ConfigID)
	o.mu.Unlock()

	ctx := context.Background()
	failed := domevents.PipelineFailed{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(
				domevents.EventPipelineFailed,
				"pipeline",
				pctx.Actor,
				pctx.Tenant,
				pctx.CorrelationID,
				"",
			),
		},
		ConfigID: pctx.ConfigID,
		Stage:    stage,
		ErrorMsg: errMsg,
	}
	_ = o.bus.Publish(ctx, failed)
	o.store.Append(failed)

	o.logger.Error("pipeline failed",
		"config_id", pctx.ConfigID,
		"stage", stage,
		"error", errMsg,
		"duration_ms", duration.Milliseconds(),
	)
}

// IsActive проверяет, запущен ли pipeline для ConfigID.
func (o *Orchestrator) IsActive(configID string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	_, ok := o.active[configID]
	return ok
}

// ActiveCount возвращает количество активных pipeline.
func (o *Orchestrator) ActiveCount() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.active)
}
