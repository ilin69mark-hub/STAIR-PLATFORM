package search

import (
	"context"
	"testing"
	"time"
)

// MockRepository — мок для тестирования.
type MockRepository struct {
	configs []*SearchItem
	projects []*SearchItem
}

func (m *MockRepository) SearchConfigs(ctx context.Context, tenantID string, params SearchParams) ([]*SearchItem, int, error) {
	var result []*SearchItem
	for _, item := range m.configs {
		if item.TenantID == tenantID {
			if matchesFilters(item, params) {
				result = append(result, item)
			}
		}
	}
	return result, len(result), nil
}

func (m *MockRepository) SearchProjects(ctx context.Context, tenantID string, params SearchParams) ([]*SearchItem, int, error) {
	var result []*SearchItem
	for _, item := range m.projects {
		if item.TenantID == tenantID {
			if matchesFilters(item, params) {
				result = append(result, item)
			}
		}
	}
	return result, len(result), nil
}

func matchesFilters(item *SearchItem, params SearchParams) bool {
	if params.Query != nil {
		query := *params.Query
		if query != "" && !containsText(item.Title, query) && !containsText(item.Description, query) {
			return false
		}
	}
	return true
}

func containsText(text, query string) bool {
	return len(text) > 0 && len(query) > 0 &&
		(len(text) >= len(query) && contains(text, query))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestEngineCreation(t *testing.T) {
	repo := &MockRepository{}
	engine := NewEngine(repo)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestSearch(t *testing.T) {
	repo := &MockRepository{
		configs: []*SearchItem{
			{ID: "config-1", Type: "configuration", Title: "Stair 1", TenantID: "tenant-1"},
			{ID: "config-2", Type: "configuration", Title: "Stair 2", TenantID: "tenant-1"},
		},
		projects: []*SearchItem{
			{ID: "project-1", Type: "project", Title: "Project 1", TenantID: "tenant-1"},
		},
	}

	engine := NewEngine(repo)

	result, err := engine.Search(context.Background(), "tenant-1", SearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 3 {
		t.Fatalf("expected 3 total results, got %d", result.Total)
	}
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
}

func TestSearchWithQuery(t *testing.T) {
	repo := &MockRepository{
		configs: []*SearchItem{
			{ID: "config-1", Type: "configuration", Title: "Spiral Stair", TenantID: "tenant-1"},
			{ID: "config-2", Type: "configuration", Title: "Straight Stair", TenantID: "tenant-1"},
		},
	}

	engine := NewEngine(repo)

	query := "Spiral"
	result, err := engine.Search(context.Background(), "tenant-1", SearchParams{Query: &query})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 total result, got %d", result.Total)
	}
	if result.Items[0].ID != "config-1" {
		t.Fatalf("expected config-1, got %s", result.Items[0].ID)
	}
}

func TestSearchWithPagination(t *testing.T) {
	repo := &MockRepository{
		configs: []*SearchItem{
			{ID: "config-1", Type: "configuration", Title: "Stair 1", TenantID: "tenant-1"},
			{ID: "config-2", Type: "configuration", Title: "Stair 2", TenantID: "tenant-1"},
			{ID: "config-3", Type: "configuration", Title: "Stair 3", TenantID: "tenant-1"},
		},
	}

	engine := NewEngine(repo)

	limit := 1
	offset := 1
	result, err := engine.Search(context.Background(), "tenant-1", SearchParams{Limit: &limit, Offset: &offset})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 3 {
		t.Fatalf("expected 3 total results, got %d", result.Total)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if !result.HasMore {
		t.Error("expected HasMore to be true")
	}
}

func TestSearchWithTenantFilter(t *testing.T) {
	repo := &MockRepository{
		configs: []*SearchItem{
			{ID: "config-1", Type: "configuration", Title: "Stair 1", TenantID: "tenant-1"},
			{ID: "config-2", Type: "configuration", Title: "Stair 2", TenantID: "tenant-2"},
		},
	}

	engine := NewEngine(repo)

	result, err := engine.Search(context.Background(), "tenant-1", SearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 total result, got %d", result.Total)
	}
}

func TestSearchByQuery(t *testing.T) {
	repo := &MockRepository{
		configs: []*SearchItem{
			{ID: "config-1", Type: "configuration", Title: "Spiral Stair", TenantID: "tenant-1"},
		},
	}

	engine := NewEngine(repo)

	result, err := engine.SearchByQuery(context.Background(), "tenant-1", "Spiral")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 total result, got %d", result.Total)
	}
}

func TestSearchByFilters(t *testing.T) {
	repo := &MockRepository{
		configs: []*SearchItem{
			{ID: "config-1", Type: "configuration", Title: "Stair 1", TenantID: "tenant-1"},
		},
	}

	engine := NewEngine(repo)

	filters := map[string]interface{}{
		"projectId": "project-1",
	}

	result, err := engine.SearchByFilters(context.Background(), "tenant-1", filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 total result, got %d", result.Total)
	}
}

func TestValidateParams(t *testing.T) {
	// Тест валидного лимита
	validLimit := 10
	err := validateParams(SearchParams{Limit: &validLimit})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Тест невалидного лимита
	invalidLimit := 2000
	err = validateParams(SearchParams{Limit: &invalidLimit})
	if err == nil {
		t.Error("expected error for invalid limit")
	}

	// Тест невалидного offset
	invalidOffset := -1
	err = validateParams(SearchParams{Offset: &invalidOffset})
	if err == nil {
		t.Error("expected error for invalid offset")
	}

	// Тест minWidth > maxWidth
	minWidth := 1000.0
	maxWidth := 500.0
	err = validateParams(SearchParams{MinWidth: &minWidth, MaxWidth: &maxWidth})
	if err == nil {
		t.Error("expected error for minWidth > maxWidth")
	}
}

func TestApplyDefaults(t *testing.T) {
	params := SearchParams{}
	result := applyDefaults(params)

	if *result.Limit != 20 {
		t.Errorf("expected limit 20, got %d", *result.Limit)
	}
	if *result.Offset != 0 {
		t.Errorf("expected offset 0, got %d", *result.Offset)
	}
	if *result.SortOrder != "desc" {
		t.Errorf("expected sort order 'desc', got %s", *result.SortOrder)
	}
}

func TestApplyPagination(t *testing.T) {
	items := []*SearchItem{
		{ID: "1"},
		{ID: "2"},
		{ID: "3"},
		{ID: "4"},
		{ID: "5"},
	}

	limit := 2
	offset := 2
	params := SearchParams{Limit: &limit, Offset: &offset}

	result := applyPagination(items, params)

	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].ID != "3" {
		t.Errorf("expected item 3, got %s", result[0].ID)
	}
	if result[1].ID != "4" {
		t.Errorf("expected item 4, got %s", result[1].ID)
	}
}

func TestApplyPaginationOutOfBounds(t *testing.T) {
	items := []*SearchItem{
		{ID: "1"},
		{ID: "2"},
	}

	limit := 10
	offset := 10
	params := SearchParams{Limit: &limit, Offset: &offset}

	result := applyPagination(items, params)

	if len(result) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result))
	}
}

func TestSortByScore(t *testing.T) {
	items := []*SearchItem{
		{ID: "1", Score: 0.5},
		{ID: "2", Score: 0.9},
		{ID: "3", Score: 0.7},
	}

	sortByScore(items)

	if items[0].ID != "2" {
		t.Errorf("expected item 2 first, got %s", items[0].ID)
	}
	if items[1].ID != "3" {
		t.Errorf("expected item 3 second, got %s", items[1].ID)
	}
	if items[2].ID != "1" {
		t.Errorf("expected item 1 third, got %s", items[2].ID)
	}
}

func TestHighlightQuery(t *testing.T) {
	tests := []struct {
		text     string
		query    string
		expected string
	}{
		{"Spiral Stair", "Spiral", "**Spiral** Stair"},
		{"Straight Stair", "Spiral", "Straight Stair"},
		{"Spiral Stair", "", "Spiral Stair"},
	}

	for _, tt := range tests {
		result := HighlightQuery(tt.text, tt.query)
		if result != tt.expected {
			t.Errorf("HighlightQuery(%q, %q) = %q, want %q", tt.text, tt.query, result, tt.expected)
		}
	}
}

func TestSearchResult(t *testing.T) {
	result := &SearchResult{
		Items: []*SearchItem{
			{ID: "1", Title: "Test"},
		},
		Total:     1,
		Limit:     10,
		Offset:    0,
		HasMore:   false,
		QueryTime: 100 * time.Millisecond,
	}

	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
	if result.HasMore {
		t.Error("expected HasMore to be false")
	}
}
