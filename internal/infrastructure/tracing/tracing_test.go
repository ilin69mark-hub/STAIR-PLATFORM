package tracing

import (
	"context"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ServiceName != "stair-platform" {
		t.Fatalf("expected service name 'stair-platform', got %q", cfg.ServiceName)
	}
	if cfg.SampleRate != 0.1 {
		t.Fatalf("expected sample rate 0.1, got %f", cfg.SampleRate)
	}
	if cfg.Enabled {
		t.Fatal("expected disabled by default")
	}
}

func TestInitTracerDisabled(t *testing.T) {
	cfg := Config{Enabled: false}

	shutdown, err := InitTracer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Shutdown should be no-op
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected error on shutdown: %v", err)
	}
}

func TestInitTracerStdout(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ServiceName: "test-service",
		SampleRate:  1.0,
		Environment: "test",
	}

	shutdown, err := InitTracer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()
}
