// Package graphql реализует GraphQL resolvers для STAIR PLATFORM.
package graphql

import (
	"context"
	"fmt"
	"time"

	"stairplatform/internal/application/project"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
	domevents "stairplatform/internal/domain/events"
	"stairplatform/internal/infrastructure/events"
)

// Resolver — корневой resolver GraphQL.
type Resolver struct {
	service     *stair.Service
	bus         *events.Bus
	projectRepo project.Repository
	configs     map[string]*stair.Config
	results     map[string]*stair.Result
	documents   map[string][]*DocumentDTO
}

// NewResolver создаёт новый resolver.
func NewResolver(service *stair.Service, bus *events.Bus, projectRepo project.Repository) *Resolver {
	return &Resolver{
		service:     service,
		bus:         bus,
		projectRepo: projectRepo,
		configs:     make(map[string]*stair.Config),
		results:     make(map[string]*stair.Result),
		documents:   make(map[string][]*DocumentDTO),
	}
}

// StairConfigurationDTO — DTO для конфигурации лестницы.
type StairConfigurationDTO struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"projectId"`
	Name              string    `json:"name"`
	Width             float64   `json:"width"`
	Height            float64   `json:"height"`
	FlightType        string    `json:"flightType"`
	StepCount         int       `json:"stepCount"`
	StepHeight        float64   `json:"stepHeight"`
	TreadDepth        float64   `json:"treadDepth"`
	StringerThickness float64   `json:"stringerThickness"`
	StepThickness     float64   `json:"stepThickness"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// AnalysisResultDTO — DTO для результата анализа.
type AnalysisResultDTO struct {
	ID           string               `json:"id"`
	ConfigID     string               `json:"configId"`
	Status       string               `json:"status"`
	Flight       string               `json:"flight,omitempty"`
	StepCount    int                  `json:"stepCount,omitempty"`
	Angle        float64              `json:"angle,omitempty"`
	RiserHeight  float64              `json:"riserHeight,omitempty"`
	TreadDepth   float64              `json:"treadDepth,omitempty"`
	ComfortScore float64              `json:"comfortScore,omitempty"`
	SafetyScore  float64              `json:"safetyScore,omitempty"`
	Issues       []ValidationIssueDTO `json:"issues,omitempty"`
	CreatedAt    time.Time            `json:"createdAt"`
}

// ValidationIssueDTO — DTO для проблемы валидации.
type ValidationIssueDTO struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Element  string `json:"element"`
	Message  string `json:"message"`
}

// OptimizationVariantDTO — DTO для варианта оптимизации.
type OptimizationVariantDTO struct {
	ID           string  `json:"id"`
	ConfigID     string  `json:"configId"`
	Rank         int     `json:"rank"`
	StepHeight   float64 `json:"stepHeight"`
	TreadDepth   float64 `json:"treadDepth"`
	ComfortScore float64 `json:"comfortScore"`
	SafetyScore  float64 `json:"safetyScore"`
	MaterialCost float64 `json:"materialCost,omitempty"`
	Selected     bool    `json:"selected"`
}

// PipelineStatusDTO — DTO для статуса pipeline.
type PipelineStatusDTO struct {
	ConfigID    string             `json:"configId"`
	Status      string             `json:"status"`
	Stages      []PipelineStageDTO `json:"stages"`
	StartedAt   *time.Time         `json:"startedAt,omitempty"`
	CompletedAt *time.Time         `json:"completedAt,omitempty"`
	Error       string             `json:"error,omitempty"`
}

// PipelineStageDTO — DTO для стадии pipeline.
type PipelineStageDTO struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// DocumentDTO — DTO для документа.
type DocumentDTO struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Format    string    `json:"format"`
	Status    string    `json:"status"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
}

// CNCJobDTO — DTO для CNC-задания.
type CNCJobDTO struct {
	ID         string    `json:"id"`
	ConfigID   string    `json:"configId"`
	Format     string    `json:"format"`
	Status     string    `json:"status"`
	OutputPath string    `json:"outputPath,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Query возвращает корневой resolver для запросов.
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

// Mutation возвращает корневой resolver для мутаций.
func (r *Resolver) Mutation() MutationResolver {
	return &mutationResolver{r}
}

// Subscription возвращает корневой resolver для подписок.
func (r *Resolver) Subscription() SubscriptionResolver {
	return &subscriptionResolver{r}
}

// QueryResolver — интерфейс для запросов.
type QueryResolver interface {
	StairConfiguration(ctx context.Context, id string) (*StairConfigurationDTO, error)
	ProjectConfigurations(ctx context.Context, projectID string) ([]*StairConfigurationDTO, error)
	Analysis(ctx context.Context, id string) (*AnalysisResultDTO, error)
	OptimizationVariants(ctx context.Context, configID string) ([]*OptimizationVariantDTO, error)
	PipelineStatus(ctx context.Context, configID string) (*PipelineStatusDTO, error)
	ConfigurationDocuments(ctx context.Context, configID string) ([]*DocumentDTO, error)
	SearchStairs(ctx context.Context, params SearchParams) ([]*StairConfigurationDTO, error)
}

// MutationResolver — интерфейс для мутаций.
type MutationResolver interface {
	CreateStairConfiguration(ctx context.Context, input CreateStairInput) (*StairConfigurationDTO, error)
	UpdateStairConfiguration(ctx context.Context, id string, input UpdateStairInput) (*StairConfigurationDTO, error)
	RunAnalysis(ctx context.Context, configID string) (*AnalysisResultDTO, error)
	RunOptimization(ctx context.Context, configID string, params OptimizationParams) ([]*OptimizationVariantDTO, error)
	GenerateDocuments(ctx context.Context, configID string, types []string) ([]*DocumentDTO, error)
	ExportCNC(ctx context.Context, configID string, format string) (*CNCJobDTO, error)
	RunPipeline(ctx context.Context, configID string) (*PipelineStatusDTO, error)
}

// SubscriptionResolver — интерфейс для подписок.
type SubscriptionResolver interface {
	PipelineStatusChanged(ctx context.Context, configID string) (<-chan *PipelineStatusDTO, error)
	AnalysisProgress(ctx context.Context, configID string) (<-chan *AnalysisResultDTO, error)
	Notifications(ctx context.Context, userID string) (<-chan *NotificationDTO, error)
}

// NotificationDTO — DTO для уведомления.
type NotificationDTO struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

// SearchParams — параметры поиска.
type SearchParams struct {
	ProjectID  *string  `json:"projectId,omitempty"`
	MinWidth   *float64 `json:"minWidth,omitempty"`
	MaxWidth   *float64 `json:"maxWidth,omitempty"`
	MinHeight  *float64 `json:"minHeight,omitempty"`
	MaxHeight  *float64 `json:"maxHeight,omitempty"`
	FlightType *string  `json:"flightType,omitempty"`
	Limit      *int     `json:"limit,omitempty"`
	Offset     *int     `json:"offset,omitempty"`
}

// CreateStairInput — входные данные для создания конфигурации.
type CreateStairInput struct {
	ProjectID         string   `json:"projectId"`
	Name              string   `json:"name"`
	Width             float64  `json:"width"`
	Height            float64  `json:"height"`
	FlightType        string   `json:"flightType"`
	StepCount         *int     `json:"stepCount,omitempty"`
	StepHeight        *float64 `json:"stepHeight,omitempty"`
	TreadDepth        *float64 `json:"treadDepth,omitempty"`
	StringerThickness *float64 `json:"stringerThickness,omitempty"`
	StepThickness     *float64 `json:"stepThickness,omitempty"`
}

// UpdateStairInput — входные данные для обновления конфигурации.
type UpdateStairInput struct {
	Name              *string  `json:"name,omitempty"`
	Width             *float64 `json:"width,omitempty"`
	Height            *float64 `json:"height,omitempty"`
	FlightType        *string  `json:"flightType,omitempty"`
	StepCount         *int     `json:"stepCount,omitempty"`
	StepHeight        *float64 `json:"stepHeight,omitempty"`
	TreadDepth        *float64 `json:"treadDepth,omitempty"`
	StringerThickness *float64 `json:"stringerThickness,omitempty"`
	StepThickness     *float64 `json:"stepThickness,omitempty"`
}

// OptimizationParams — параметры оптимизации.
type OptimizationParams struct {
	MaxVariants *int    `json:"maxVariants,omitempty"`
	OptimizeFor *string `json:"optimizeFor,omitempty"`
}

// queryResolver — resolver для запросов.
type queryResolver struct {
	*Resolver
}

// StairConfiguration получает конфигурацию по ID.
func (r *queryResolver) StairConfiguration(ctx context.Context, id string) (*StairConfigurationDTO, error) {
	cfg, ok := r.configs[id]
	if !ok {
		return nil, fmt.Errorf("configuration %q not found", id)
	}
	return r.configToDTO(id, cfg), nil
}

// ProjectConfigurations получает конфигурации проекта.
func (r *queryResolver) ProjectConfigurations(ctx context.Context, projectID string) ([]*StairConfigurationDTO, error) {
	var result []*StairConfigurationDTO
	for id, cfg := range r.configs {
		if cfg != nil {
			dto := r.configToDTO(id, cfg)
			if dto.ProjectID == projectID {
				result = append(result, dto)
			}
		}
	}
	return result, nil
}

// Analysis получает результат анализа.
func (r *queryResolver) Analysis(ctx context.Context, id string) (*AnalysisResultDTO, error) {
	return &AnalysisResultDTO{
		ID:        id,
		Status:    "completed",
		CreatedAt: time.Now(),
	}, nil
}

// OptimizationVariants получает варианты оптимизации.
func (r *queryResolver) OptimizationVariants(ctx context.Context, configID string) ([]*OptimizationVariantDTO, error) {
	return []*OptimizationVariantDTO{}, nil
}

// PipelineStatus получает статус pipeline.
func (r *queryResolver) PipelineStatus(ctx context.Context, configID string) (*PipelineStatusDTO, error) {
	return &PipelineStatusDTO{
		ConfigID: configID,
		Status:   "completed",
		Stages:   []PipelineStageDTO{},
	}, nil
}

// ConfigurationDocuments получает документы конфигурации.
func (r *queryResolver) ConfigurationDocuments(ctx context.Context, configID string) ([]*DocumentDTO, error) {
	return r.documents[configID], nil
}

// SearchStairs ищет лестницы по параметрам.
func (r *queryResolver) SearchStairs(ctx context.Context, params SearchParams) ([]*StairConfigurationDTO, error) {
	var result []*StairConfigurationDTO
	for id, cfg := range r.configs {
		if cfg != nil {
			dto := r.configToDTO(id, cfg)
			result = append(result, dto)
		}
	}
	if params.Limit != nil && len(result) > *params.Limit {
		result = result[:*params.Limit]
	}
	return result, nil
}

// mutationResolver — resolver для мутаций.
type mutationResolver struct {
	*Resolver
}

// CreateStairConfiguration создаёт новую конфигурацию.
func (r *mutationResolver) CreateStairConfiguration(ctx context.Context, input CreateStairInput) (*StairConfigurationDTO, error) {
	cfg := &stair.Config{
		Width:      engineering.Length(input.Width),
		Height:     engineering.Length(input.Height),
		Flight:     engineering.FlightType(input.FlightType),
		StepHeight: engineering.Length(180),
	}

	if input.StepHeight != nil {
		cfg.StepHeight = engineering.Length(*input.StepHeight)
	}
	if input.StringerThickness != nil {
		cfg.StringerThickness = engineering.Length(*input.StringerThickness)
	} else {
		cfg.StringerThickness = engineering.Length(50)
	}
	if input.StepThickness != nil {
		cfg.StepThickness = engineering.Length(*input.StepThickness)
	} else {
		cfg.StepThickness = engineering.Length(40)
	}

	id := fmt.Sprintf("config-%d", time.Now().UnixNano())
	r.configs[id] = cfg

	return r.configToDTO(id, cfg), nil
}

// UpdateStairConfiguration обновляет конфигурацию.
func (r *mutationResolver) UpdateStairConfiguration(ctx context.Context, id string, input UpdateStairInput) (*StairConfigurationDTO, error) {
	cfg, ok := r.configs[id]
	if !ok {
		return nil, fmt.Errorf("configuration %q not found", id)
	}

	// Name хранится в DTO, не в Config — обновление игнорируется.
	if input.Width != nil {
		cfg.Width = engineering.Length(*input.Width)
	}
	if input.Height != nil {
		cfg.Height = engineering.Length(*input.Height)
	}
	if input.FlightType != nil {
		cfg.Flight = engineering.FlightType(*input.FlightType)
	}
	if input.StepHeight != nil {
		cfg.StepHeight = engineering.Length(*input.StepHeight)
	}

	return r.configToDTO(id, cfg), nil
}

// RunAnalysis запускает анализ.
func (r *mutationResolver) RunAnalysis(ctx context.Context, configID string) (*AnalysisResultDTO, error) {
	cfg, ok := r.configs[configID]
	if !ok {
		return nil, fmt.Errorf("configuration %q not found", configID)
	}

	result, err := r.service.Calculate(ctx, *cfg, stair.Options{})
	if err != nil {
		return nil, err
	}

	r.results[configID] = result

	return &AnalysisResultDTO{
		ID:        configID,
		ConfigID:  configID,
		Status:    "completed",
		Flight:    string(cfg.Flight),
		StepCount: result.Flight.StepCount,
		Angle:     float64(result.Flight.Angle),
		CreatedAt: time.Now(),
	}, nil
}

// RunOptimization запускает оптимизацию.
func (r *mutationResolver) RunOptimization(ctx context.Context, configID string, params OptimizationParams) ([]*OptimizationVariantDTO, error) {
	return []*OptimizationVariantDTO{}, nil
}

// GenerateDocuments генерирует документы.
func (r *mutationResolver) GenerateDocuments(ctx context.Context, configID string, types []string) ([]*DocumentDTO, error) {
	docs := make([]*DocumentDTO, 0, len(types))
	for _, t := range types {
		doc := &DocumentDTO{
			ID:        fmt.Sprintf("doc-%d", time.Now().UnixNano()),
			Type:      t,
			Format:    "json",
			Status:    "rendered",
			Title:     fmt.Sprintf("Document for %s", configID),
			CreatedAt: time.Now(),
		}
		docs = append(docs, doc)
		r.documents[configID] = append(r.documents[configID], doc)
	}
	return docs, nil
}

// ExportCNC экспортирует в CNC формат.
func (r *mutationResolver) ExportCNC(ctx context.Context, configID string, format string) (*CNCJobDTO, error) {
	return &CNCJobDTO{
		ID:        fmt.Sprintf("job-%d", time.Now().UnixNano()),
		ConfigID:  configID,
		Format:    format,
		Status:    "pending",
		CreatedAt: time.Now(),
	}, nil
}

// RunPipeline запускает pipeline.
func (r *mutationResolver) RunPipeline(ctx context.Context, configID string) (*PipelineStatusDTO, error) {
	cfg, ok := r.configs[configID]
	if !ok {
		return nil, fmt.Errorf("configuration %q not found", configID)
	}

	// Запускаем pipeline в goroutine
	go func() {
		adapter := stair.NewPipelineAdapter(r.service, &busAdapter{bus: r.bus})
		_, _ = adapter.RunPipelineWithResult(ctx, configID, *cfg, stair.Options{})
	}()

	return &PipelineStatusDTO{
		ConfigID: configID,
		Status:   "running",
		Stages: []PipelineStageDTO{
			{Name: "validation", Status: "pending"},
			{Name: "analysis", Status: "pending"},
			{Name: "geometry", Status: "pending"},
			{Name: "manufacturing", Status: "pending"},
			{Name: "pricing", Status: "pending"},
			{Name: "document", Status: "pending"},
		},
	}, nil
}

// subscriptionResolver — resolver для подписок.
type subscriptionResolver struct {
	*Resolver
}

// PipelineStatusChanged подписывается на изменения статуса pipeline.
func (r *subscriptionResolver) PipelineStatusChanged(ctx context.Context, configID string) (<-chan *PipelineStatusDTO, error) {
	ch := make(chan *PipelineStatusDTO, 1)

	// Подписываемся на события pipeline
	r.bus.Subscribe(domevents.EventPipelineCompleted, func(ctx context.Context, e domevents.Event) error {
		ch <- &PipelineStatusDTO{
			ConfigID: configID,
			Status:   "completed",
		}
		return nil
	})

	return ch, nil
}

// AnalysisProgress подписывается на прогресс анализа.
func (r *subscriptionResolver) AnalysisProgress(ctx context.Context, configID string) (<-chan *AnalysisResultDTO, error) {
	ch := make(chan *AnalysisResultDTO, 1)

	r.bus.Subscribe(domevents.EventAnalysisCompleted, func(ctx context.Context, e domevents.Event) error {
		ch <- &AnalysisResultDTO{
			ConfigID: configID,
			Status:   "completed",
		}
		return nil
	})

	return ch, nil
}

// Notifications подписывается на уведомления.
func (r *subscriptionResolver) Notifications(ctx context.Context, userID string) (<-chan *NotificationDTO, error) {
	ch := make(chan *NotificationDTO, 1)

	r.bus.Subscribe(domevents.EventAnalysisStarted, func(ctx context.Context, e domevents.Event) error {
		ch <- &NotificationDTO{
			ID:        string(e.EventType()),
			Type:      string(e.EventType()),
			Message:   fmt.Sprintf("Event: %s", e.EventType()),
			CreatedAt: time.Now(),
		}
		return nil
	})

	return ch, nil
}

// configToDTO конвертирует Config в DTO.
func (r *Resolver) configToDTO(id string, cfg *stair.Config) *StairConfigurationDTO {
	return &StairConfigurationDTO{
		ID:                id,
		ProjectID:         "default",
		Name:              fmt.Sprintf("Stair %s", id),
		Width:             cfg.Width.Millimeters(),
		Height:            cfg.Height.Millimeters(),
		FlightType:        string(cfg.Flight),
		StepHeight:        cfg.StepHeight.Millimeters(),
		StringerThickness: cfg.StringerThickness.Millimeters(),
		StepThickness:     cfg.StepThickness.Millimeters(),
		Status:            "draft",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

// busAdapter — адаптер для Bus.
type busAdapter struct {
	bus *events.Bus
}

func (b *busAdapter) Publish(ctx context.Context, event domevents.Event) {
	_ = b.bus.Publish(ctx, event)
}
