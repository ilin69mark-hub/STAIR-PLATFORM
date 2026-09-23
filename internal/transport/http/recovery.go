package http

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"stairplatform/internal/infrastructure/sentry"
)

// PanicRecoveryMiddleware перехватывает паники в handler'ах и возвращает 500.
// Логирует stack trace для отладки. S-140: паника дополнительно уходит в
// Sentry (только если SDK инициализирован — STAIR_SENTRY_DSN задан).
func PanicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				slog.Error("panic recovered",
					"panic", rec,
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
					"stack", string(stack),
				)

				// S-140: в Sentry с контекстом запроса; PII скраббится
				// BeforeSend-хуком (см. internal/infrastructure/sentry/scrub.go).
				sentry.CapturePanic(rec, r, stack)

				// Отправляем структурированную ошибку
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"error": map[string]any{
						"code":       "internal",
						"message":    "Internal server error",
						"request_id": RequestID(r.Context()),
					},
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
