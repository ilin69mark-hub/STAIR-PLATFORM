package stair

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	engmfg "stairplatform/internal/engine/manufacturing"
	"stairplatform/internal/engine/pipeline"
	"stairplatform/internal/infrastructure/events"
)

func pipelineCfg() Config {
	return Config{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2700),
		Flight:            engineering.FlightStraight,
		StepHeight:        engineering.Length(180),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		Clearance:         engineering.Length(80),
		RailingHeight:     engineering.Length(900),
	}
}

// TestPipelineAdapterRegisterStages — стадии регистрируются и первый
// stage (validation) действительно исполняется через замыкание.
func TestPipelineAdapterRegisterStages(t *testing.T) {
	orch := pipeline.NewOrchestrator(events.NewBus(), events.NewStore(100), nil)
	adapter := NewPipelineAdapter(NewService(), nil)
	adapter.RegisterStages(orch)

	err := orch.Start(context.Background(), "cfg-x", "", "")
	if err == nil {
		t.Fatal("expected error: no config in fresh pipeline context")
	}
}

// TestPipelineAdapterExecuteStages — успешное исполнение всех стадий.
func TestPipelineAdapterExecuteStages(t *testing.T) {
	bus := events.NewBus()
	adapter := NewPipelineAdapter(NewService(), &busAdapter{bus: bus})

	ctx := context.Background()
	cfg := pipelineCfg()
	stages := []struct {
		name string
		exec func(context.Context, *pipeline.Context) error
	}{
		{"validation", adapter.executeValidation},
		{"analysis", adapter.executeAnalysis},
		{"geometry", adapter.executeGeometry},
		{"manufacturing", adapter.executeManufacturing},
		{"pricing", adapter.executePricing},
		{"document", adapter.executeDocument},
	}
	for _, st := range stages {
		input := &pipeline.Context{ConfigID: "cfg-1", Data: map[string]any{"config_id": "cfg-1", "config": cfg}}
		if err := st.exec(ctx, input); err != nil {
			t.Fatalf("%s: unexpected error: %v", st.name, err)
		}
	}
}

// TestPipelineAdapterNoBus — все стадии работают и без EventPublisher.
func TestPipelineAdapterNoBus(t *testing.T) {
	adapter := NewPipelineAdapter(NewService(), nil)
	ctx := context.Background()

	adapter.executeAnalysis(ctx, &pipeline.Context{Data: map[string]any{"config_id": "c"}})
	adapter.executeGeometry(ctx, &pipeline.Context{Data: map[string]any{"config_id": "c"}})
	adapter.executeManufacturing(ctx, &pipeline.Context{Data: map[string]any{"config_id": "c"}})
	adapter.executePricing(ctx, &pipeline.Context{Data: map[string]any{"config_id": "c"}})
	adapter.executeDocument(ctx, &pipeline.Context{Data: map[string]any{"config_id": "c"}})

	// Валидация без конфигурации в Data — ошибка раньше вызова Calculate.
	if err := adapter.executeValidation(ctx, &pipeline.Context{Data: map[string]any{"config_id": "c"}}); err == nil {
		t.Fatal("expected error for missing config")
	}
}

// TestPipelineAdapterValidationFailed — ошибка Calculate → ValidationFailed.
func TestPipelineAdapterValidationFailed(t *testing.T) {
	bus := events.NewBus()
	adapter := NewPipelineAdapter(NewService(), &busAdapter{bus: bus})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input := &pipeline.Context{
		ConfigID: "cfg-1",
		Data: map[string]any{
			"config_id": "cfg-1",
			"config":    pipelineCfg(),
		},
	}
	if err := adapter.executeValidation(ctx, input); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// TestManufacturingBlocked — blocking-результат при превышении габарита листа.
func TestManufacturingBlocked(t *testing.T) {
	feas := &engmfg.FeasibilityError{PartLen: 8000, PartWid: 2000, Kerf: 3}

	// Материал задан явно — крупнейший лист находится в каталоге.
	if vr := manufacturingBlocked(feas, 50, dommfg.MaterialCode("STEEL-S235")); !vr.Blocking || len(vr.Issues) == 0 {
		t.Fatal("want blocking issues for explicit material")
	}
	// Пустой материал — подбор по толщине (50 мм → STEEL-S235).
	if vr := manufacturingBlocked(feas, 50, ""); !vr.Blocking {
		t.Fatal("want blocking result for thickness-based material")
	}
	// Толщина без поддержки в каталоге — фолбэк STEEL-S235.
	if vr := manufacturingBlocked(feas, 0, ""); !vr.Blocking {
		t.Fatal("want blocking result for unsupported thickness")
	}
	// Материал вне каталога листов — дефолтные габариты 6000×3000.
	if vr := manufacturingBlocked(feas, 50, dommfg.MaterialCode("BOGUS-CUSTOM")); !vr.Blocking {
		t.Fatal("want blocking result for unknown material")
	}
}

// TestValidateConfig — валидация конфигурации до постановки в очередь.
func TestValidateConfig(t *testing.T) {
	cfg := pipelineCfg()
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig(valid): %v", err)
	}
	if err := (&Service{}).ValidateConfig(cfg); err != nil {
		t.Fatalf("Service.ValidateConfig(valid): %v", err)
	}

	bad := Config{Flight: engineering.FlightType("bogus")}
	if err := ValidateConfig(bad); err == nil {
		t.Fatal("ValidateConfig(bogus): want error")
	}
}
