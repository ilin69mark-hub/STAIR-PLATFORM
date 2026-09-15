// Package graphql реализует GraphQL HTTP handler для STAIR PLATFORM.
package graphql

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// Request — GraphQL запрос.
type Request struct {
	Query     string                 `json:"query"`
	Operation string                 `json:"operationName"`
	Variables map[string]interface{} `json:"variables"`
}

// Response — GraphQL ответ.
type Response struct {
	Data   interface{} `json:"data,omitempty"`
	Errors []Error     `json:"errors,omitempty"`
}

// Error — GraphQL ошибка.
type Error struct {
	Message    string        `json:"message"`
	Locations  []Location    `json:"locations,omitempty"`
	Path       []interface{} `json:"path,omitempty"`
	Extensions interface{}   `json:"extensions,omitempty"`
}

// Location — позиция в запросе.
type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// HTTPHandler — GraphQL HTTP handler.
type HTTPHandler struct {
	resolver *Resolver
}

// NewHTTPHandler создаёт новый HTTP handler.
func NewHTTPHandler(resolver *Resolver) *HTTPHandler {
	return &HTTPHandler{
		resolver: resolver,
	}
}

// Resolver возвращает GraphQL resolver.
func (h *HTTPHandler) Resolver() *Resolver {
	return h.resolver
}

// ServeHTTP обрабатывает HTTP запросы.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// CORS preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Только POST и GET
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(Response{
			Errors: []Error{{Message: "method not allowed"}},
		})
		return
	}

	var req Request

	// Парсим запрос
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(Response{
				Errors: []Error{{Message: "failed to read request body"}},
			})
			return
		}
		defer func() { _ = r.Body.Close() }()

		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(Response{
				Errors: []Error{{Message: "invalid JSON"}},
			})
			return
		}
	} else {
		// GET request — парсим query из URL
		req.Query = r.URL.Query().Get("query")
		req.Operation = r.URL.Query().Get("operationName")
	}

	// Выполняем запрос
	ctx := r.Context()
	resp := h.executeQuery(ctx, req)

	// Отправляем ответ
	_ = json.NewEncoder(w).Encode(resp)
}

// executeQuery выполняет GraphQL запрос.
func (h *HTTPHandler) executeQuery(ctx context.Context, req Request) Response {
	// Простая маршрулизация по operation name
	switch {
	case containsQuery(req.Query, "stairConfiguration") || req.Operation == "StairConfiguration":
		return h.executeStairConfigurationQuery(ctx, req)
	case containsQuery(req.Query, "projectConfigurations") || req.Operation == "ProjectConfigurations":
		return h.executeProjectConfigurationsQuery(ctx, req)
	case containsQuery(req.Query, "createStairConfiguration") || req.Operation == "CreateStairConfiguration":
		return h.executeCreateStairConfigurationMutation(ctx, req)
	case containsQuery(req.Query, "runAnalysis") || req.Operation == "RunAnalysis":
		return h.executeRunAnalysisMutation(ctx, req)
	case containsQuery(req.Query, "runPipeline") || req.Operation == "RunPipeline":
		return h.executeRunPipelineMutation(ctx, req)
	default:
		return Response{
			Errors: []Error{{Message: "unknown query"}},
		}
	}
}

// executeStairConfigurationQuery выполняет запрос stairConfiguration.
func (h *HTTPHandler) executeStairConfigurationQuery(ctx context.Context, req Request) Response {
	// Извлекаем ID из переменных
	id := ""
	if v, ok := req.Variables["id"].(string); ok {
		id = v
	}

	result, err := h.resolver.Query().StairConfiguration(ctx, id)
	if err != nil {
		return Response{
			Errors: []Error{{Message: err.Error()}},
		}
	}

	return Response{Data: result}
}

// executeProjectConfigurationsQuery выполняет запрос projectConfigurations.
func (h *HTTPHandler) executeProjectConfigurationsQuery(ctx context.Context, req Request) Response {
	projectID := ""
	if v, ok := req.Variables["projectId"].(string); ok {
		projectID = v
	}

	result, err := h.resolver.Query().ProjectConfigurations(ctx, projectID)
	if err != nil {
		return Response{
			Errors: []Error{{Message: err.Error()}},
		}
	}

	return Response{Data: result}
}

// executeCreateStairConfigurationMutation выполняет мутацию createStairConfiguration.
func (h *HTTPHandler) executeCreateStairConfigurationMutation(ctx context.Context, req Request) Response {
	// Извлекаем input из переменных
	input := CreateStairInput{}
	if v, ok := req.Variables["input"].(map[string]interface{}); ok {
		if v, ok := v["projectId"].(string); ok {
			input.ProjectID = v
		}
		if v, ok := v["name"].(string); ok {
			input.Name = v
		}
		if v, ok := v["width"].(float64); ok {
			input.Width = v
		}
		if v, ok := v["height"].(float64); ok {
			input.Height = v
		}
		if v, ok := v["flightType"].(string); ok {
			input.FlightType = v
		}
	}

	result, err := h.resolver.Mutation().CreateStairConfiguration(ctx, input)
	if err != nil {
		return Response{
			Errors: []Error{{Message: err.Error()}},
		}
	}

	return Response{Data: result}
}

// executeRunAnalysisMutation выполняет мутацию runAnalysis.
func (h *HTTPHandler) executeRunAnalysisMutation(ctx context.Context, req Request) Response {
	configID := ""
	if v, ok := req.Variables["configId"].(string); ok {
		configID = v
	}

	result, err := h.resolver.Mutation().RunAnalysis(ctx, configID)
	if err != nil {
		return Response{
			Errors: []Error{{Message: err.Error()}},
		}
	}

	return Response{Data: result}
}

// executeRunPipelineMutation выполняет мутацию runPipeline.
func (h *HTTPHandler) executeRunPipelineMutation(ctx context.Context, req Request) Response {
	configID := ""
	if v, ok := req.Variables["configId"].(string); ok {
		configID = v
	}

	result, err := h.resolver.Mutation().RunPipeline(ctx, configID)
	if err != nil {
		return Response{
			Errors: []Error{{Message: err.Error()}},
		}
	}

	return Response{Data: result}
}

// containsQuery проверяет содержит ли запрос подстроку.
func containsQuery(query, substr string) bool {
	for i := 0; i <= len(query)-len(substr); i++ {
		if query[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
