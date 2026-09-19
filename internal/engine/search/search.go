// Package search реализует поисковый движок для STAIR PLATFORM.
package search

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SearchParams — параметры поиска.
type SearchParams struct {
	ProjectID  *string  `json:"projectId,omitempty"`
	MinWidth   *float64 `json:"minWidth,omitempty"`
	MaxWidth   *float64 `json:"maxWidth,omitempty"`
	MinHeight  *float64 `json:"minHeight,omitempty"`
	MaxHeight  *float64 `json:"maxHeight,omitempty"`
	FlightType *string  `json:"flightType,omitempty"`
	Query      *string  `json:"query,omitempty"`
	SortBy     *string  `json:"sortBy,omitempty"`
	SortOrder  *string  `json:"sortOrder,omitempty"`
	Limit      *int     `json:"limit,omitempty"`
	Offset     *int     `json:"offset,omitempty"`
}

// SearchResult — результат поиска.
type SearchResult struct {
	Items     []*SearchItem `json:"items"`
	Total     int           `json:"total"`
	Limit     int           `json:"limit"`
	Offset    int           `json:"offset"`
	HasMore   bool          `json:"hasMore"`
	QueryTime time.Duration `json:"queryTime"`
}

// SearchItem — элемент поиска.
type SearchItem struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // "project", "configuration", "document"
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	ProjectID   string                 `json:"projectId,omitempty"`
	TenantID    string                 `json:"tenantId"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
	Score       float64                `json:"score,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Repository — интерфейс для доступа к данным поиска.
type Repository interface {
	SearchConfigs(ctx context.Context, tenantID string, params SearchParams) ([]*SearchItem, int, error)
	SearchProjects(ctx context.Context, tenantID string, params SearchParams) ([]*SearchItem, int, error)
}

// Engine — поисковый движок.
type Engine struct {
	repo Repository
}

// NewEngine создаёт новый поисковый движок.
func NewEngine(repo Repository) *Engine {
	return &Engine{
		repo: repo,
	}
}

// Search выполняет поиск по всем сущностям.
func (e *Engine) Search(ctx context.Context, tenantID string, params SearchParams) (*SearchResult, error) {
	start := time.Now()

	// Валидируем параметры
	if err := validateParams(params); err != nil {
		return nil, err
	}

	// Устанавливаем значения по умолчанию
	params = applyDefaults(params)

	var allItems []*SearchItem
	total := 0

	// Ищем конфигурации
	configItems, configTotal, err := e.repo.SearchConfigs(ctx, tenantID, params)
	if err != nil {
		return nil, fmt.Errorf("search configs: %w", err)
	}
	allItems = append(allItems, configItems...)
	total += configTotal

	// Ищем проекты
	projectItems, projectTotal, err := e.repo.SearchProjects(ctx, tenantID, params)
	if err != nil {
		return nil, fmt.Errorf("search projects: %w", err)
	}
	allItems = append(allItems, projectItems...)
	total += projectTotal

	// Сортируем по score (если нужно)
	if params.SortBy != nil && *params.SortBy == "relevance" {
		sortByScore(allItems)
	}

	// Применяем лимит и offset
	items := applyPagination(allItems, params)

	return &SearchResult{
		Items:     items,
		Total:     total,
		Limit:     *params.Limit,
		Offset:    *params.Offset,
		HasMore:   *params.Offset+*params.Limit < total,
		QueryTime: time.Since(start),
	}, nil
}

// SearchByQuery выполняет полнотекстовый поиск.
func (e *Engine) SearchByQuery(ctx context.Context, tenantID, query string) (*SearchResult, error) {
	params := SearchParams{
		Query: &query,
	}
	return e.Search(ctx, tenantID, params)
}

// SearchByFilters выполняет поиск по фильтрам.
func (e *Engine) SearchByFilters(ctx context.Context, tenantID string, filters map[string]interface{}) (*SearchResult, error) {
	params := SearchParams{}

	if v, ok := filters["projectId"].(string); ok {
		params.ProjectID = &v
	}
	if v, ok := filters["minWidth"].(float64); ok {
		params.MinWidth = &v
	}
	if v, ok := filters["maxWidth"].(float64); ok {
		params.MaxWidth = &v
	}
	if v, ok := filters["minHeight"].(float64); ok {
		params.MinHeight = &v
	}
	if v, ok := filters["maxHeight"].(float64); ok {
		params.MaxHeight = &v
	}
	if v, ok := filters["flightType"].(string); ok {
		params.FlightType = &v
	}

	return e.Search(ctx, tenantID, params)
}

// validateParams валидирует параметры поиска.
func validateParams(params SearchParams) error {
	if params.Limit != nil && (*params.Limit < 0 || *params.Limit > 1000) {
		return fmt.Errorf("limit must be between 0 and 1000")
	}
	if params.Offset != nil && *params.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}
	if params.MinWidth != nil && params.MaxWidth != nil && *params.MinWidth > *params.MaxWidth {
		return fmt.Errorf("minWidth must be less than maxWidth")
	}
	if params.MinHeight != nil && params.MaxHeight != nil && *params.MinHeight > *params.MaxHeight {
		return fmt.Errorf("minHeight must be less than maxHeight")
	}
	return nil
}

// applyDefaults применяет значения по умолчанию.
func applyDefaults(params SearchParams) SearchParams {
	if params.Limit == nil {
		limit := 20
		params.Limit = &limit
	}
	if params.Offset == nil {
		offset := 0
		params.Offset = &offset
	}
	if params.SortOrder == nil {
		order := "desc"
		params.SortOrder = &order
	}
	return params
}

// applyPagination применяет пагинацию к результатам.
func applyPagination(items []*SearchItem, params SearchParams) []*SearchItem {
	start := *params.Offset
	if start >= len(items) {
		return []*SearchItem{}
	}

	end := start + *params.Limit
	if end > len(items) {
		end = len(items)
	}

	return items[start:end]
}

// sortByScore сортирует элементы по score.
func sortByScore(items []*SearchItem) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Score > items[i].Score {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// HighlightQuery выделяет совпадения в тексте.
func HighlightQuery(text, query string) string {
	if query == "" {
		return text
	}

	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)

	idx := strings.Index(lowerText, lowerQuery)
	if idx == -1 {
		return text
	}

	return text[:idx] + "**" + text[idx:idx+len(query)] + "**" + text[idx+len(query):]
}
