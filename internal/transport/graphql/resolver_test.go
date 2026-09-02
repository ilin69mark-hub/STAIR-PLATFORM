package graphql

import (
	"context"
	"testing"
	"time"
)

func TestResolverCreation(t *testing.T) {
	resolver := newTestResolver()
	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestCreateStairConfiguration(t *testing.T) {
	resolver := newTestResolver()

	input := CreateStairInput{
		ProjectID:  "proj-1",
		Name:       "Test Stair",
		Width:      900,
		Height:     2700,
		FlightType: "straight",
	}

	dto, err := resolver.Mutation().CreateStairConfiguration(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto == nil {
		t.Fatal("expected non-nil dto")
	}
	if dto.Width != 900 {
		t.Fatalf("expected width 900, got %v", dto.Width)
	}
}

func TestUpdateStairConfiguration(t *testing.T) {
	resolver := newTestResolver()

	// Создаём конфигурацию
	input := CreateStairInput{
		ProjectID:  "proj-1",
		Name:       "Test Stair",
		Width:      900,
		Height:     2700,
		FlightType: "straight",
	}

	dto, _ := resolver.Mutation().CreateStairConfiguration(context.Background(), input)

	// Обновляем
	newWidth := 1000.0
	updateInput := UpdateStairInput{
		Width: &newWidth,
	}

	updated, err := resolver.Mutation().UpdateStairConfiguration(context.Background(), dto.ID, updateInput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Width != 1000 {
		t.Fatalf("expected width 1000, got %v", updated.Width)
	}
}

func TestGetStairConfiguration(t *testing.T) {
	resolver := newTestResolver()

	// Создаём конфигурацию
	input := CreateStairInput{
		ProjectID:  "proj-1",
		Name:       "Test Stair",
		Width:      900,
		Height:     2700,
		FlightType: "straight",
	}

	created, _ := resolver.Mutation().CreateStairConfiguration(context.Background(), input)

	// Получаем
	dto, err := resolver.Query().StairConfiguration(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.ID != created.ID {
		t.Fatalf("expected ID %q, got %q", created.ID, dto.ID)
	}
}

func TestGetStairConfigurationNotFound(t *testing.T) {
	resolver := newTestResolver()

	_, err := resolver.Query().StairConfiguration(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

func TestRunAnalysis(t *testing.T) {
	resolver := newTestResolver()

	// Создаём конфигурацию
	input := CreateStairInput{
		ProjectID:  "proj-1",
		Name:       "Test Stair",
		Width:      900,
		Height:     2700,
		FlightType: "straight",
	}

	created, _ := resolver.Mutation().CreateStairConfiguration(context.Background(), input)

	// Запускаем анализ
	result, err := resolver.Mutation().RunAnalysis(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected status 'completed', got %q", result.Status)
	}
}

func TestGenerateDocuments(t *testing.T) {
	resolver := newTestResolver()

	// Создаём конфигурацию
	input := CreateStairInput{
		ProjectID:  "proj-1",
		Name:       "Test Stair",
		Width:      900,
		Height:     2700,
		FlightType: "straight",
	}

	created, _ := resolver.Mutation().CreateStairConfiguration(context.Background(), input)

	// Генерируем документы
	docs, err := resolver.Mutation().GenerateDocuments(context.Background(), created.ID, []string{"technical_spec", "drawing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
}

func TestExportCNC(t *testing.T) {
	resolver := newTestResolver()

	// Создаём конфигурацию
	input := CreateStairInput{
		ProjectID:  "proj-1",
		Name:       "Test Stair",
		Width:      900,
		Height:     2700,
		FlightType: "straight",
	}

	created, _ := resolver.Mutation().CreateStairConfiguration(context.Background(), input)

	// Экспортируем
	job, err := resolver.Mutation().ExportCNC(context.Background(), created.ID, "step")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Format != "step" {
		t.Fatalf("expected format 'step', got %q", job.Format)
	}
}

func TestSearchStairs(t *testing.T) {
	resolver := newTestResolver()

	// Создаём конфигурации с задержкой для уникальных ID
	for i := 0; i < 3; i++ {
		input := CreateStairInput{
			ProjectID:  "proj-1",
			Name:       "Test Stair",
			Width:      900,
			Height:     2700,
			FlightType: "straight",
		}
		resolver.Mutation().CreateStairConfiguration(context.Background(), input)
		time.Sleep(10 * time.Millisecond)
	}

	// Ищем
	results, err := resolver.Query().SearchStairs(context.Background(), SearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
}

func TestPipelineStatus(t *testing.T) {
	resolver := newTestResolver()

	status, err := resolver.Query().PipelineStatus(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.ConfigID != "config-1" {
		t.Fatalf("expected configID 'config-1', got %q", status.ConfigID)
	}
}

func TestConfigurationDocuments(t *testing.T) {
	resolver := newTestResolver()

	// Создаём конфигурацию
	input := CreateStairInput{
		ProjectID:  "proj-1",
		Name:       "Test Stair",
		Width:      900,
		Height:     2700,
		FlightType: "straight",
	}

	created, _ := resolver.Mutation().CreateStairConfiguration(context.Background(), input)

	// Генерируем документы
	resolver.Mutation().GenerateDocuments(context.Background(), created.ID, []string{"technical_spec"})

	// Получаем документы
	docs, err := resolver.Query().ConfigurationDocuments(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
}

func TestOptimizationVariants(t *testing.T) {
	resolver := newTestResolver()

	variants, err := resolver.Query().OptimizationVariants(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 0 {
		t.Fatalf("expected 0 variants, got %d", len(variants))
	}
}

func TestRunOptimization(t *testing.T) {
	resolver := newTestResolver()

	// Создаём конфигурацию
	input := CreateStairInput{
		ProjectID:  "proj-1",
		Name:       "Test Stair",
		Width:      900,
		Height:     2700,
		FlightType: "straight",
	}

	created, _ := resolver.Mutation().CreateStairConfiguration(context.Background(), input)

	// Запускаем оптимизацию
	variants, err := resolver.Mutation().RunOptimization(context.Background(), created.ID, OptimizationParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 0 {
		t.Fatalf("expected 0 variants, got %d", len(variants))
	}
}

func TestRunPipeline(t *testing.T) {
	resolver := newTestResolver()

	// Создаём конфигурацию
	input := CreateStairInput{
		ProjectID:  "proj-1",
		Name:       "Test Stair",
		Width:      900,
		Height:     2700,
		FlightType: "straight",
	}

	created, _ := resolver.Mutation().CreateStairConfiguration(context.Background(), input)

	// Запускаем pipeline
	status, err := resolver.Mutation().RunPipeline(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "running" {
		t.Fatalf("expected status 'running', got %q", status.Status)
	}

	// Ждём завершения
	time.Sleep(100 * time.Millisecond)
}
