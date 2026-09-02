package http

import (
	"net/http"

	"stairplatform/internal/infrastructure/security"
)

// SecurityConfig — конфигурация security middleware для HTTP router.
type SecurityConfig struct {
	// AllowedOrigins — список разрешенных origins для CORS.
	AllowedOrigins []string
	// EnableHSTS — включить HSTS.
	EnableHSTS bool
	// CSPPolicy — Content Security Policy policy string.
	CSPPolicy string
}

// SecurityMiddleware добавляет security headers и CORS.
func SecurityMiddleware(cfg *SecurityConfig) func(http.Handler) http.Handler {
	if cfg == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	secCfg := security.DefaultConfig()
	if len(cfg.AllowedOrigins) > 0 {
		secCfg.AllowedOrigins = cfg.AllowedOrigins
	}
	secCfg.EnableHSTS = cfg.EnableHSTS
	if cfg.CSPPolicy != "" {
		secCfg.CSPPolicy = cfg.CSPPolicy
	}

	return func(next http.Handler) http.Handler {
		// Apply security headers
		handler := security.SecurityHeaders(secCfg)(next)
		// Apply CORS
		handler = security.CORS(secCfg)(handler)
		return handler
	}
}
