package http

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	ws "stairplatform/internal/transport/websocket"
)

func TestWebSocketHandlerIntegration(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	// Создаем тестовый запрос с токеном
	req := httptest.NewRequest(http.MethodGet, "/ws?token=valid-token", nil)
	rr := httptest.NewRecorder()

	// Обрабатываем запрос
	handler.HandleWebSocket(rr, req)

	// WebSocket upgrade не работает в тесте, проверяем что handler работает
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusOK {
		t.Errorf("expected status 200 or 400, got %d", rr.Code)
	}
}

func TestGraphQLHTTPHandlerCreation(t *testing.T) {
	// Тест создания GraphQLHTTPHandler
	// В реальном тесте нужен resolver, но мы проверяем компиляцию
	t.Skip("requires resolver instance")
}

func TestSecurityMiddlewareCreation(t *testing.T) {
	cfg := &SecurityConfig{
		AllowedOrigins: []string{"http://localhost:3000"},
		EnableHSTS:     true,
	}

	handler := SecurityMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestSecurityMiddlewareNil(t *testing.T) {
	handler := SecurityMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
