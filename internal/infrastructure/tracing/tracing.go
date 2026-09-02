// Package tracing реализует настройку OpenTelemetry трассировки.
// Поддерживает OTLP (Jaeger, Grafana Tempo) и stdout (для разработки).
package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Config — конфигурация трассировки.
type Config struct {
	// Enabled — включить трассировку.
	Enabled bool
	// ServiceName — имя сервиса для Jaeger/Tempo.
	ServiceName string
	// Endpoint — OTLP endpoint (напр. "localhost:4318").
	// Пусто → stdout exporter (для разработки).
	Endpoint string
	// SampleRate — доля семплирования (0.0-1.0). 1.0 = все запросы.
	SampleRate float64
	// Environment — окружение (dev, staging, prod).
	Environment string
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		ServiceName: "stair-platform",
		SampleRate:  0.1, // 10% в проде
		Environment: "development",
	}
}

// InitTracer 初始化 OpenTelemetry трассировку. Возвращает функцию shutdown.
func InitTracer(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled {
		return func(ctx context.Context) error { return nil }, nil
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"", // use default schema URL
			semconv.ServiceNameKey.String(cfg.ServiceName),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	if cfg.Endpoint != "" {
		exporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("tracing: create OTLP exporter: %w", err)
		}
		slog.Info("tracing: OTLP exporter initialized", "endpoint", cfg.Endpoint)
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("tracing: create stdout exporter: %w", err)
		}
		slog.Info("tracing: stdout exporter initialized")
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(cfg.SampleRate),
		)),
	)

	otel.SetTracerProvider(tp)

	slog.Info("tracing initialized",
		"service", cfg.ServiceName,
		"sample_rate", cfg.SampleRate,
		"environment", cfg.Environment,
	)

	return tp.Shutdown, nil
}

// Tracer возвращает трейсер с заданным именем.
func Tracer(name string) *sdktrace.TracerProvider {
	return nil // используется глобальный otel.Tracer
}
