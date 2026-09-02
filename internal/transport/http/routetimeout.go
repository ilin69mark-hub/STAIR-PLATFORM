package http

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// RouteTimeoutConfig конфигурация таймаутов для разных маршрутов.
type RouteTimeoutConfig struct {
	defaultTimeout time.Duration
	routes         map[string]time.Duration
	mu             sync.RWMutex
}

// NewRouteTimeoutConfig создаёт конфиг с дефолтным таймаутом.
func NewRouteTimeoutConfig(defaultTimeout time.Duration) *RouteTimeoutConfig {
	return &RouteTimeoutConfig{
		defaultTimeout: defaultTimeout,
		routes:         make(map[string]time.Duration),
	}
}

// Set устанавливает таймаут для конкретного маршрута.
func (rtc *RouteTimeoutConfig) Set(path string, timeout time.Duration) {
	rtc.mu.Lock()
	defer rtc.mu.Unlock()
	rtc.routes[path] = timeout
}

// Get возвращает таймаут для маршрута.
func (rtc *RouteTimeoutConfig) Get(path string) time.Duration {
	rtc.mu.RLock()
	defer rtc.mu.RUnlock()
	if t, ok := rtc.routes[path]; ok {
		return t
	}
	return rtc.defaultTimeout
}

// RouteTimeoutMiddleware добавляет context deadline на основе маршрута.
func RouteTimeoutMiddleware(config *RouteTimeoutConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timeout := config.Get(r.URL.Path)
			if timeout <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DefaultAPIRouteTimeouts создаёт конфиг с типичными таймаутами API.
func DefaultAPIRouteTimeouts() *RouteTimeoutConfig {
	cfg := NewRouteTimeoutConfig(30 * time.Second)

	// Быстрые эндпоинты
	cfg.Set("/api/v1/health", 5*time.Second)
	cfg.Set("/api/v1/ready", 5*time.Second)
	cfg.Set("/api/v1/auth/login", 10*time.Second)
	cfg.Set("/api/v1/auth/register", 10*time.Second)

	// Средние эндпоинты
	cfg.Set("/api/v1/projects", 15*time.Second)
	cfg.Set("/api/v1/stairs", 15*time.Second)

	// Долгие операции
	cfg.Set("/api/v1/stairs/calculate", 60*time.Second)
	cfg.Set("/api/v1/documents/generate", 120*time.Second)
	cfg.Set("/api/v1/pipeline", 120*time.Second)

	return cfg
}
