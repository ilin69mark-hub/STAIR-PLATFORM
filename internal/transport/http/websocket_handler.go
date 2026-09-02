package http

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	ws "stairplatform/internal/transport/websocket"
)

// TokenValidator интерфейс для проверки session tokens.
type TokenValidator interface {
	Authenticate(ctx context.Context, token string) (userID string, err error)
}

// WebSocketHandler обрабатывает WebSocket подключения.
type WebSocketHandler struct {
	hub       *ws.Hub
	logger    *slog.Logger
	validator TokenValidator
}

// NewWebSocketHandler создаёт новый handler.
func NewWebSocketHandler(hub *ws.Hub, logger *slog.Logger, validator TokenValidator) *WebSocketHandler {
	return &WebSocketHandler{
		hub:       hub,
		logger:    logger,
		validator: validator,
	}
}

// HandleWebSocket обрабатывает WebSocket upgrade.
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	userID := ""

	// 1. Пробуем Authorization: Bearer <token>
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			uid, err := h.validator.Authenticate(r.Context(), token)
			if err != nil {
				h.logger.Warn("websocket: invalid bearer token",
					"error", err,
					"remote_addr", r.RemoteAddr,
				)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID = uid
		}
	}

	// 2. Пробуем token query parameter
	if userID == "" {
		if token := r.URL.Query().Get("token"); token != "" {
			uid, err := h.validator.Authenticate(r.Context(), token)
			if err != nil {
				h.logger.Warn("websocket: invalid query token",
					"error", err,
					"remote_addr", r.RemoteAddr,
				)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID = uid
		}
	}

	// 3. Если нет валидного токена — reject
	if userID == "" {
		h.logger.Warn("websocket: no valid authentication",
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Проверяем origin
	origin := r.Header.Get("Origin")
	if origin == "" {
		h.logger.Warn("websocket: missing origin header",
			"user_id", userID,
			"remote_addr", r.RemoteAddr,
		)
	}

	h.logger.Info("websocket: authenticated connection",
		"user_id", userID,
		"origin", origin,
		"remote_addr", r.RemoteAddr,
	)

	ws.HandleWebSocket(h.hub, w, r)
}

// GetHub возвращает WebSocket hub.
func (h *WebSocketHandler) GetHub() *ws.Hub {
	return h.hub
}
