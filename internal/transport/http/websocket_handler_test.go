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

	// Создаем тестовый запрос с валидным Bearer-токеном
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
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

	// Создаем тестовый запрос с origin и Bearer-токеном
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Authorization", "Bearer valid-token")
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

	// Создаем тестовый запрос с невалидным Bearer-токеном
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
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

// TestWebSocketHandlerRejectsQueryToken — S-110 (WS-TOKEN-IN-QUERY):
// ?token= больше не аутентифицирует, даже если токен валиден.
func TestWebSocketHandlerRejectsQueryToken(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{} // принимает любой токен
	handler := NewWebSocketHandler(hub, logger, validator)

	req := httptest.NewRequest(http.MethodGet, "/ws?token=valid-token", nil)
	rr := httptest.NewRecorder()

	handler.HandleWebSocket(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for query-token, got %d", rr.Code)
	}
}

// TestWebSocketHandlerRejectsQueryTokenEvenWithCookie — query-токен отклоняется
// явно даже при валидной session-cookie (чтобы практика не сохранилась).
func TestWebSocketHandlerRejectsQueryTokenEvenWithCookie(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	req := httptest.NewRequest(http.MethodGet, "/ws?token=leaked-token", nil)
	req.AddCookie(testCookie(sessionCookieName, "valid-cookie"))
	rr := httptest.NewRecorder()

	handler.HandleWebSocket(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for query-token even with cookie, got %d", rr.Code)
	}
}

// TestWebSocketHandlerWithSessionCookie — аутентификация по session-cookie
// (store; браузер шлёт cookie на same-origin WS-handshake).
func TestWebSocketHandlerWithSessionCookie(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.AddCookie(testCookie(sessionCookieName, "valid-token"))
	rr := httptest.NewRecorder()

	handler.HandleWebSocket(rr, req)

	// upgrade не работает в httptest: ок 200/400, но не 401
	if rr.Code == http.StatusUnauthorized {
		t.Errorf("expected authenticated via session cookie, got 401")
	}
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusOK {
		t.Errorf("expected status 200 or 400, got %d", rr.Code)
	}
}

// TestWebSocketHandlerWithAdminSessionCookie — session_admin cookie (admin).
func TestWebSocketHandlerWithAdminSessionCookie(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{}
	handler := NewWebSocketHandler(hub, logger, validator)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set(appOriginHeader, appOriginAdmin)
	req.AddCookie(testCookie(sessionCookieName+adminOriginCookieSuffix, "valid-token"))
	rr := httptest.NewRecorder()

	handler.HandleWebSocket(rr, req)

	if rr.Code == http.StatusUnauthorized {
		t.Errorf("expected authenticated via admin session cookie, got 401")
	}
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusOK {
		t.Errorf("expected status 200 or 400, got %d", rr.Code)
	}
}

// TestWebSocketHandlerInvalidSessionCookie — невалидная cookie → 401.
func TestWebSocketHandlerInvalidSessionCookie(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	logger := slog.Default()
	validator := &mockTokenValidator{
		validateFunc: func(ctx context.Context, token string) (string, error) {
			return "", context.Canceled
		},
	}
	handler := NewWebSocketHandler(hub, logger, validator)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.AddCookie(testCookie(sessionCookieName, "bad-token"))
	rr := httptest.NewRecorder()

	handler.HandleWebSocket(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for invalid cookie, got %d", rr.Code)
	}
}
