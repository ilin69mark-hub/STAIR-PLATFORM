package http

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"stairplatform/internal/application/auth"
	ws "stairplatform/internal/transport/websocket"
)

// TokenValidator интерфейс для проверки session tokens. Возвращает ID и роль
// пользователя (роль нужна для admin-namespace WS, S-119).
type TokenValidator interface {
	Authenticate(ctx context.Context, token string) (userID, role string, err error)
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
	userRole := ""

	// 1. Authorization: Bearer <token> (не-браузерные клиенты, API-ключи).
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			uid, role, err := h.validator.Authenticate(r.Context(), token)
			if err != nil {
				h.logger.Warn("websocket: invalid bearer token",
					"error", err,
					"remote_addr", r.RemoteAddr,
				)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID = uid
			userRole = role
		}
	}

	// 2. Session cookie (store: «session», admin: «session_admin»);
	//    браузер шлёт её на WS-handshake автоматически (same-origin).
	//    Namespace определяется ПУТЁМ (/ws = store, /ws/admin = admin),
	//    а не заголовком X-App-Origin: браузерный WebSocket API не умеет
	//    ставить кастомные заголовки (S-116).
	if userID == "" {
		if token := wsSessionToken(r); token != "" {
			uid, role, err := h.validator.Authenticate(r.Context(), token)
			if err != nil {
				h.logger.Warn("websocket: invalid session cookie",
					"error", err,
					"remote_addr", r.RemoteAddr,
				)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID = uid
			userRole = role
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

	// 4. Admin-namespace (/ws/admin) — только роль admin (S-119). Обычный
	// пользователь с валидной session_admin cookie НЕ проходит: cookie
	// получает любой зарегистрированный юзер, роль — только админ.
	// Store-namespace (/ws) открыт всем аутентифицированным.
	if wsAppOrigin(r) == appOriginAdmin && userRole != string(auth.RoleAdmin) {
		h.logger.Warn("websocket: non-admin role on admin namespace",
			"user_id", userID,
			"role", userRole,
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
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

// wsAppOrigin определяет namespace приложения по пути WS-handshake.
// Браузерный WebSocket API не позволяет ставить кастомные заголовки
// (X-App-Origin), поэтому admin-подключения идут на /ws/admin, а store —
// на /ws (S-116, SHOULD ревью PR #50). Заголовок X-App-Origin на WS
// намеренно НЕ используется: браузер его не пошлёт, а не-браузерные
// клиенты аутентифицируются Bearer-токеном (заголовок им не нужен).
func wsAppOrigin(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/ws/admin") {
		return appOriginAdmin
	}
	return appOriginStore
}

// wsSessionToken возвращает session-cookie для namespace приложения,
// определённого по пути WS-handshake (см. wsAppOrigin).
func wsSessionToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieFor(wsAppOrigin(r))); err == nil {
		return c.Value
	}
	return ""
}
