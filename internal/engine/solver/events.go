package solver

import (
	"context"

	domevents "stairplatform/internal/domain/events"
)

// EventPublisher — интерфейс для публикации событий.
type EventPublisher interface {
	Publish(ctx context.Context, event domevents.Event)
}

// SolverWithEvents — обёртка над Solver с публикацией событий.
type SolverWithEvents struct {
	publisher EventPublisher
}

// NewSolverWithEvents создаёт Solver с событиями.
func NewSolverWithEvents(publisher EventPublisher) *SolverWithEvents {
	return &SolverWithEvents{publisher: publisher}
}

// PublishAnalysisStarted публикует событие начала анализа.
func (s *SolverWithEvents) PublishAnalysisStarted(ctx context.Context, configID, flight string) {
	if s.publisher == nil {
		return
	}
	s.publisher.Publish(ctx, domevents.AnalysisStarted{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventAnalysisStarted, "solver", "system", "", "", ""),
		},
		ConfigID: configID,
		Flight:   flight,
	})
}

// PublishAnalysisCompleted публикует событие завершения анализа.
func (s *SolverWithEvents) PublishAnalysisCompleted(ctx context.Context, configID, flight string, stepCount int, angle float64) {
	if s.publisher == nil {
		return
	}
	s.publisher.Publish(ctx, domevents.AnalysisCompleted{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventAnalysisCompleted, "solver", "system", "", "", ""),
		},
		ConfigID:  configID,
		Flight:    flight,
		StepCount: stepCount,
		Angle:     angle,
	})
}

// PublishAnalysisFailed публикует событие ошибки анализа.
func (s *SolverWithEvents) PublishAnalysisFailed(ctx context.Context, configID, errorMsg string) {
	if s.publisher == nil {
		return
	}
	s.publisher.Publish(ctx, domevents.EngineFinished{
		BaseEvent: domevents.BaseEvent{
			Meta: domevents.NewEventMetadata(domevents.EventEngineAnalysisFinished, "solver", "system", "", "", ""),
		},
		Engine:   "solver",
		ConfigID: configID,
		Success:  false,
		ErrorMsg: errorMsg,
	})
}
