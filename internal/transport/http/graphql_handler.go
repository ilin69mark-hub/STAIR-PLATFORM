package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	gqlhttp "stairplatform/internal/transport/graphql"
)

// GraphQLHTTPHandler — wrapper для GraphQL handler совместимый с HTTP router.
type GraphQLHTTPHandler struct {
	*gqlhttp.HTTPHandler
}

// NewGraphQLHTTPHandler создаёт новый GraphQL HTTP handler.
func NewGraphQLHTTPHandler(resolver *gqlhttp.Resolver) *GraphQLHTTPHandler {
	return &GraphQLHTTPHandler{
		HTTPHandler: gqlhttp.NewHTTPHandler(resolver),
	}
}

// ServeHTTP обрабатывает HTTP запросы.
func (h *GraphQLHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// CORS preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Только POST и GET
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(gqlhttp.Response{
			Errors: []gqlhttp.Error{{Message: "method not allowed"}},
		})
		return
	}

	var req gqlhttp.Request

	// Парсим запрос
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(gqlhttp.Response{
				Errors: []gqlhttp.Error{{Message: "failed to read request body"}},
			})
			return
		}
		defer r.Body.Close()

		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(gqlhttp.Response{
				Errors: []gqlhttp.Error{{Message: "invalid JSON"}},
			})
			return
		}
	} else {
		// GET request — парсим query из URL
		req.Query = r.URL.Query().Get("query")
		req.Operation = r.URL.Query().Get("operationName")
	}

	// Выполняем запрос через встроенный handler с контекстом запроса (содержит auth данные).
	h.ServeHTTPWithContext(r.Context(), w, r, req)
}

// ServeHTTPWithContext выполняет GraphQL запрос с контекстом.
func (h *GraphQLHTTPHandler) ServeHTTPWithContext(ctx context.Context, w http.ResponseWriter, r *http.Request, req gqlhttp.Request) {
	// Простая маршрулизация по operation name
	var resp gqlhttp.Response

	switch {
	case containsQuery(req.Query, "stairConfiguration") || req.Operation == "StairConfiguration":
		id := ""
		if v, ok := req.Variables["id"].(string); ok {
			id = v
		}
		result, err := h.Resolver().Query().StairConfiguration(ctx, id)
		if err != nil {
			resp = gqlhttp.Response{Errors: []gqlhttp.Error{{Message: err.Error()}}}
		} else {
			resp = gqlhttp.Response{Data: result}
		}

	case containsQuery(req.Query, "projectConfigurations") || req.Operation == "ProjectConfigurations":
		projectID := ""
		if v, ok := req.Variables["projectId"].(string); ok {
			projectID = v
		}
		result, err := h.Resolver().Query().ProjectConfigurations(ctx, projectID)
		if err != nil {
			resp = gqlhttp.Response{Errors: []gqlhttp.Error{{Message: err.Error()}}}
		} else {
			resp = gqlhttp.Response{Data: result}
		}

	case containsQuery(req.Query, "createStairConfiguration") || req.Operation == "CreateStairConfiguration":
		input := gqlhttp.CreateStairInput{}
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
		result, err := h.Resolver().Mutation().CreateStairConfiguration(ctx, input)
		if err != nil {
			resp = gqlhttp.Response{Errors: []gqlhttp.Error{{Message: err.Error()}}}
		} else {
			resp = gqlhttp.Response{Data: result}
		}

	case containsQuery(req.Query, "runAnalysis") || req.Operation == "RunAnalysis":
		configID := ""
		if v, ok := req.Variables["configId"].(string); ok {
			configID = v
		}
		result, err := h.Resolver().Mutation().RunAnalysis(ctx, configID)
		if err != nil {
			resp = gqlhttp.Response{Errors: []gqlhttp.Error{{Message: err.Error()}}}
		} else {
			resp = gqlhttp.Response{Data: result}
		}

	case containsQuery(req.Query, "runPipeline") || req.Operation == "RunPipeline":
		configID := ""
		if v, ok := req.Variables["configId"].(string); ok {
			configID = v
		}
		result, err := h.Resolver().Mutation().RunPipeline(ctx, configID)
		if err != nil {
			resp = gqlhttp.Response{Errors: []gqlhttp.Error{{Message: err.Error()}}}
		} else {
			resp = gqlhttp.Response{Data: result}
		}

	default:
		resp = gqlhttp.Response{Errors: []gqlhttp.Error{{Message: "unknown query"}}}
	}

	// Отправляем ответ
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// Resolver возвращает GraphQL resolver.
func (h *GraphQLHTTPHandler) Resolver() *gqlhttp.Resolver {
	return h.HTTPHandler.Resolver()
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
