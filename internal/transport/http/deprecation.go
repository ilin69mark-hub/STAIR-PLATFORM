package http

import (
	"net/http"
	"time"
)

// DeprecationInfo — информация о deprecated endpoint.
type DeprecationInfo struct {
	// Sunset — дата, когда endpoint будет удалён.
	Sunset time.Time
	// Message — описание для клиента.
	Message string
	// Link — ссылка на документацию или миграцию.
	Link string
}

// DeprecatedMiddleware добавляет заголовки Deprecation и Sunset
// для deprecated endpoints (RFC 8594).
func DeprecatedMiddleware(info DeprecationInfo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Deprecation", "true")
			w.Header().Set("Sunset", info.Sunset.Format(http.TimeFormat))
			if info.Link != "" {
				w.Header().Set("Link", info.Link+`; rel="successor-version"`)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// DeprecatedResponse возвращает 410 Gone для удалённых endpoints.
func DeprecatedResponse(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusGone, map[string]string{
		"error":   "deprecated",
		"message": message,
	})
}
