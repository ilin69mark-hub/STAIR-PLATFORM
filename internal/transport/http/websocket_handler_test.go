package http

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	ws "stairplatform/internal/transport/websocket"
)

// mockTokenValidator для тестов.
type mockTokenValidator struct {
	validateFunc func(ctx context.Context, token string) (string, error)
}

func (m *mockTokenValidator) Authenticate(ctx context.Context, token string) (string, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, token)
	}
	return "test-user", nil
}

func TestWebSocketHandlerCreation(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.hub == nil {
		t.Fatal("expected non-nil hub")
	}
	if handler.logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestWebSocketHandlerGetHub(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	if handler.GetHub() != hub {
		t.Fatal("expected same hub instance")
	}
}

func TestWebSocketHandlerHandleWebSocket(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	// Создаем тестовый запрос с валидным токеном
	req := httptest.NewRequest(http.MethodGet, "/ws?token=valid-token", nil)
	rr := httptest.NewRecorder()

	// Обрабатываем запрос (не upgrades WebSocket в тесте)
	handler.HandleWebSocket(rr, req)

	// WebSocket upgrade не работает в httptest, поэтому проверяем что handler не паникует
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusOK {
		t.Errorf("expected status 200 or 400, got %d", rr.Code)
	}
}

func TestWebSocketHandlerWithAnonymousUser(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	// Создаем тестовый запрос без токена
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rr := httptest.NewRecorder()

	// Обрабатываем запрос
	handler.HandleWebSocket(rr, req)

	// Должен вернуть 401 Unauthorized
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestWebSocketHandlerWithOrigin(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	// Создаем тестовый запрос с origin и токеном
	req := httptest.NewRequest(http.MethodGet, "/ws?token=valid-token", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	// Обрабатываем запрос
	handler.HandleWebSocket(rr, req)

	// Проверяем что handler работает (upgrade не работает в тесте)
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusOK {
		t.Errorf("expected status 200 or 400, got %d", rr.Code)
	}
}

func TestWebSocketHandlerWithInvalidToken(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{
		validateFunc: func(ctx context.Context, token string) (string, error) {
			return "", context.Canceled
		},
	}
	handler := NewWebSocketHandler(hub, logger, validator)

	// Создаем тестовый запрос с невалидным токеном
	req := httptest.NewRequest(http.MethodGet, "/ws?token=invalid-token", nil)
	rr := httptest.NewRecorder()

	// Обрабатываем запрос
	handler.HandleWebSocket(rr, req)

	// Должен вернуть 401 Unauthorized
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestWebSocketHandlerWithBearerToken(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	// Создаем тестовый запрос с Bearer токеном
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	// Обрабатываем запрос
	handler.HandleWebSocket(rr, req)

	// Проверяем что handler работает (upgrade не работает в тесте)
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusOK {
		t.Errorf("expected status 200 or 400, got %d", rr.Code)
	}
}
