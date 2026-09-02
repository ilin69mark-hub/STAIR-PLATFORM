package pipeline

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	domevents "stairplatform/internal/domain/events"
	"stairplatform/internal/infrastructure/events"
)

func TestOrchestratorBasicPipeline(t *testing.T) {
	bus := events.NewBus()
	store := events.NewStore(0)
	orch := NewOrchestrator(bus, store, nil)

	var stageOrder []string
	var mu sync.Mutex

	orch.RegisterStage(Stage{
		Name: "geometry",
		Execute: func(ctx context.Context, input *Context) error {
			mu.Lock()
			stageOrder = append(stageOrder, "geometry")
			input.Data["stage:geometry:done"] = true
			mu.Unlock()
			return nil
		},
	})
	orch.RegisterStage(Stage{
		Name:     "validation",
		DependsOn: []string{"geometry"},
		Execute: func(ctx context.Context, input *Context) error {
			mu.Lock()
			stageOrder = append(stageOrder, "validation")
			input.Data["stage:validation:done"] = true
			mu.Unlock()
			return nil
		},
	})
	orch.RegisterStage(Stage{
		Name:     "manufacturing",
		DependsOn: []string{"validation"},
		Execute: func(ctx context.Context, input *Context) error {
			mu.Lock()
			stageOrder = append(stageOrder, "manufacturing")
			input.Data["stage:manufacturing:done"] = true
			mu.Unlock()
			return nil
		},
	})

	err := orch.Start(context.Background(), "config-1", "user-1", "t1")
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	expected := []string{"geometry", "validation", "manufacturing"}
	mu.Lock()
	defer mu.Unlock()
	if len(stageOrder) != len(expected) {
		t.Fatalf("expected stages %v, got %v", expected, stageOrder)
	}
	for i, s := range expected {
		if stageOrder[i] != s {
			t.Fatalf("expected stage %d to be %s, got %s", i, s, stageOrder[i])
		}
	}
}

func TestOrchestratorStageFailure(t *testing.T) {
	bus := events.NewBus()
	store := events.NewStore(0)
	orch := NewOrchestrator(bus, store, nil)

	orch.RegisterStage(Stage{
		Name: "geometry",
		Execute: func(ctx context.Context, input *Context) error {
			return fmt.Errorf("geometry failed")
		},
	})
	orch.RegisterStage(Stage{
		Name:     "validation",
		DependsOn: []string{"geometry"},
		Execute: func(ctx context.Context, input *Context) error {
			t.Fatal("validation should not run after geometry failure")
			return nil
		},
	})

	err := orch.Start(context.Background(), "config-1", "user-1", "t1")
	if err == nil {
		t.Fatal("expected error from pipeline")
	}

	// Проверяем что pipeline не активен после ошибки.
	if orch.IsActive("config-1") {
		t.Fatal("pipeline should not be active after failure")
	}
}

func TestOrchestratorDuplicateRun(t *testing.T) {
	bus := events.NewBus()
	store := events.NewStore(0)
	orch := NewOrchestrator(bus, store, nil)

	orch.RegisterStage(Stage{
		Name: "slow",
		Execute: func(ctx context.Context, input *Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	})

	// Запускаем в goroutine чтобы поймать параллельный запуск.
	go func() {
		_ = orch.Start(context.Background(), "config-1", "user-1", "t1")
	}()

	time.Sleep(10 * time.Millisecond) // даём времени первому запуску начаться

	err := orch.Start(context.Background(), "config-1", "user-1", "t1")
	if err == nil {
		t.Fatal("expected error for duplicate pipeline run")
	}
}

func TestOrchestratorContextCancellation(t *testing.T) {
	bus := events.NewBus()
	store := events.NewStore(0)
	orch := NewOrchestrator(bus, store, nil)

	orch.RegisterStage(Stage{
		Name: "stage1",
		Execute: func(ctx context.Context, input *Context) error {
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем сразу

	err := orch.Start(ctx, "config-1", "user-1", "t1")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestOrchestratorEventsPublished(t *testing.T) {
	bus := events.NewBus()
	store := events.NewStore(0)
	orch := NewOrchestrator(bus, store, nil)

	var engineEvents []domevents.EventType
	var mu sync.Mutex

	bus.Subscribe("*", func(ctx context.Context, e domevents.Event) error {
		mu.Lock()
		engineEvents = append(engineEvents, e.EventType())
		mu.Unlock()
		return nil
	})

	orch.RegisterStage(Stage{
		Name: "test",
		Execute: func(ctx context.Context, input *Context) error {
			return nil
		},
	})

	_ = orch.Start(context.Background(), "config-1", "user-1", "t1")

	mu.Lock()
	defer mu.Unlock()

	// Ожидаем: engine.test_started, engine.test_finished, engine.pipeline_completed
	found := make(map[domevents.EventType]bool)
	for _, et := range engineEvents {
		found[et] = true
	}

	if !found[domevents.EventPipelineCompleted] {
		t.Fatalf("expected PipelineCompleted event, got %v", engineEvents)
	}
}

func TestOrchestratorActiveCount(t *testing.T) {
	bus := events.NewBus()
	store := events.NewStore(0)
	orch := NewOrchestrator(bus, store, nil)

	if orch.ActiveCount() != 0 {
		t.Fatal("expected 0 active pipelines")
	}

	orch.RegisterStage(Stage{
		Name: "test",
		Execute: func(ctx context.Context, input *Context) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	})

	done := make(chan error)
	go func() {
		done <- orch.Start(context.Background(), "config-1", "user-1", "t1")
	}()

	time.Sleep(10 * time.Millisecond)
	if orch.ActiveCount() != 1 {
		t.Fatal("expected 1 active pipeline")
	}

	<-done

	if orch.ActiveCount() != 0 {
		t.Fatal("expected 0 active pipelines after completion")
	}
}

func TestOrchestratorDependenciesNotSatisfied(t *testing.T) {
	bus := events.NewBus()
	store := events.NewStore(0)
	orch := NewOrchestrator(bus, store, nil)

	orch.RegisterStage(Stage{
		Name:     "manufacturing",
		DependsOn: []string{"validation"}, // validation не зарегистрирована
		Execute: func(ctx context.Context, input *Context) error {
			return nil
		},
	})

	err := orch.Start(context.Background(), "config-1", "user-1", "t1")
	if err == nil {
		t.Fatal("expected error for unsatisfied dependencies")
	}
}
