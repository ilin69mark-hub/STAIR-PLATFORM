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
	// allowedOrigins — разрешённые Origin для WS-handshake (STAIR_WS_ORIGINS /
	// STAIR_CORS_ORIGINS); nil/пусто — безопасный дефолт same-origin (S-112).
	allowedOrigins []string
}

// NewWebSocketHandler создаёт новый handler.
func NewWebSocketHandler(hub *ws.Hub, logger *slog.Logger, validator TokenValidator, allowedOrigins []string) *WebSocketHandler {
	return &WebSocketHandler{
		hub:            hub,
		logger:         logger,
		validator:      validator,
		allowedOrigins: allowedOrigins,
	}
}

// HandleWebSocket обрабатывает WebSocket upgrade.
// Аутентификация — только через session-cookie (браузер) или
// Authorization: Bearer <token>; query-параметр token намеренно отклоняется
// (токен в URL светится в логах/истории/referrer — S-110, WS-TOKEN-IN-QUERY).
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Query-токен отклоняем явно, даже если рядом есть валидная cookie —
	// чтобы практика передавать токен в URL не могла незаметно сохраниться.
	if r.URL.Query().Get("token") != "" {
		h.logger.Warn("websocket: token in query string is not supported",
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "unauthorized: use session cookie or Authorization header", http.StatusUnauthorized)
		return
	}

	userID := ""

	// 1. Authorization: Bearer <token> (не-браузерные клиенты, API-ключи).
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

	// 2. Session cookie (store: «session», admin: «session_admin»);
	//    браузер шлёт её на WS-handshake автоматически (same-origin).
	if userID == "" {
		if token := sessionToken(r); token != "" {
			uid, err := h.validator.Authenticate(r.Context(), token)
			if err != nil {
				h.logger.Warn("websocket: invalid session cookie",
					"error", err,
					"remote_addr", r.RemoteAddr,
				)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID = uid
		}
	}

	// 3. Если нет валидной аутентификации — reject
	if userID == "" {
		h.logger.Warn("websocket: no valid authentication",
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Origin проверяется в ws.OriginChecker (upgrader) — см. S-112.
	origin := r.Header.Get("Origin")

	h.logger.Info("websocket: authenticated connection",
		"user_id", userID,
		"origin", origin,
		"remote_addr", r.RemoteAddr,
	)

	ws.HandleWebSocket(h.hub, w, r, userID, h.allowedOrigins)
}

// GetHub возвращает WebSocket hub.
func (h *WebSocketHandler) GetHub() *ws.Hub {
	return h.hub
}
