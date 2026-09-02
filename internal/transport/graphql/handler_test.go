package graphql

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/events"
)

func newTestResolver() *Resolver {
	service := stair.NewService()
	bus := events.NewBus()
	repo := &MockProjectRepository{}
	return NewResolver(service, bus, repo)
}

func TestHTTPHandlerCreation(t *testing.T) {
	resolver := newTestResolver()
	handler := NewHTTPHandler(resolver)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestHTTPHandlerServeHTTPOptions(t *testing.T) {
	resolver := newTestResolver()
	handler := NewHTTPHandler(resolver)

	req := httptest.NewRequest(http.MethodOptions, "/graphql", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHTTPHandlerServeHTTPMethodNotAllowed(t *testing.T) {
	resolver := newTestResolver()
	handler := NewHTTPHandler(resolver)

	req := httptest.NewRequest(http.MethodDelete, "/graphql", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rr.Code)
	}
}

func TestHTTPHandlerServeHTTPPost(t *testing.T) {
	resolver := newTestResolver()
	handler := NewHTTPHandler(resolver)

	body := Request{
		Query: `{ stAIRConfiguration(id: "test") { id } }`,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp Response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.Errors) == 0 {
		t.Error("expected errors for unknown query")
	}
}

func TestHTTPHandlerServeHTTPGet(t *testing.T) {
	resolver := newTestResolver()
	handler := NewHTTPHandler(resolver)

	req := httptest.NewRequest(http.MethodGet, "/graphql?query={stairConfiguration(id:\"test\"){id}}", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHTTPHandlerInvalidJSON(t *testing.T) {
	resolver := newTestResolver()
	handler := NewHTTPHandler(resolver)

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestContainsQuery(t *testing.T) {
	tests := []struct {
		query    string
		substr   string
		expected bool
	}{
		{"query { stairConfiguration }", "stairConfiguration", true},
		{"query { projectConfigurations }", "projectConfigurations", true},
		{"query { createStairConfiguration }", "createStairConfiguration", true},
		{"query { runAnalysis }", "runAnalysis", true},
		{"query { runPipeline }", "runPipeline", true},
		{"query { other }", "stairConfiguration", false},
	}

	for _, tt := range tests {
		result := containsQuery(tt.query, tt.substr)
		if result != tt.expected {
			t.Errorf("containsQuery(%q, %q) = %v, want %v", tt.query, tt.substr, result, tt.expected)
		}
	}
}

func TestResponseSerialization(t *testing.T) {
	resp := Response{
		Data: map[string]string{"id": "test"},
		Errors: []Error{
			{Message: "test error"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(decoded.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(decoded.Errors))
	}
}
