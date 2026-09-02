// Package security реализует security middleware для HTTP сервера.
package security

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// Config — конфигурация security middleware.
type Config struct {
	// AllowedOrigins — список разрешенных origins для CORS.
	AllowedOrigins []string

	// AllowCredentials — разрешить ли credentials (cookies, authorization headers).
	AllowCredentials bool

	// AllowedMethods — разрешенные HTTP методы.
	AllowedMethods []string

	// AllowedHeaders — разрешенные заголовки.
	AllowedHeaders []string

	// ExposedHeaders — заголовки, доступные клиенту.
	ExposedHeaders []string

	// MaxAge — максимальный age для preflight кэша (в секундах).
	MaxAge int

	// CSPPolicy — Content Security Policy policy string.
	CSPPolicy string

	// EnableHSTS — включить HSTS.
	EnableHSTS bool

	// HSTSMaxAge — максимальный age для HSTS (в секундах).
	HSTSMaxAge int

	// Logger — логгер для security событий.
	Logger *slog.Logger
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() Config {
	return Config{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-CSRF-Token", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		MaxAge:           86400,
		// CSP hardening: removed unsafe-inline and unsafe-eval
		// Use nonces or hashes for inline scripts if needed
		CSPPolicy:  "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' ws: wss:; frame-ancestors 'none'",
		EnableHSTS: true,
		HSTSMaxAge: 31536000,
		Logger:     slog.Default(),
	}
}

// SecurityHeaders middleware добавляет security headers.
func SecurityHeaders(config Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content Security Policy
			if config.CSPPolicy != "" {
				w.Header().Set("Content-Security-Policy", config.CSPPolicy)
			}

			// HSTS
			if config.EnableHSTS {
				w.Header().Set("Strict-Transport-Security", "max-age="+itoa(config.HSTSMaxAge)+"; includeSubDomains; preload")
			}

			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// Referrer Policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions Policy
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

			next.ServeHTTP(w, r)
		})
	}
}

// CORS middleware добавляет CORS заголовки.
func CORS(config Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Проверяем разрешенные origins
			if origin != "" && isOriginAllowed(origin, config.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")

				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if len(config.AllowedHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
				}

				if len(config.ExposedHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
				}
			}

			// Обработка preflight запросов
			if r.Method == http.MethodOptions && origin != "" {
				if len(config.AllowedMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
				}

				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", itoa(config.MaxAge))
				}

				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isOriginAllowed проверяет, разрешен ли origin.
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		// Поддержка wildcard (*.example.com)
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:] // .example.com
			if strings.HasSuffix(origin, suffix) {
				return true
			}
		}
	}
	return false
}

// itoa конвертирует int в string.
func itoa(n int) string {
	return strconv.Itoa(n)
}
