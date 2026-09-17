package http

import (
	"context"
	"net/http"
	"strings"
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

// Get возвращает таймаут для маршрута. Поддерживает паттерны ServeMux:
// "{"..."}" сегменты трактуются как подстановочные (например
// "/api/v1/projects/{id}/calculate" матчит "/api/v1/projects/5/calculate").
func (rtc *RouteTimeoutConfig) Get(path string) time.Duration {
	rtc.mu.RLock()
	defer rtc.mu.RUnlock()
	if t, ok := rtc.routes[path]; ok {
		return t
	}
	for pattern, t := range rtc.routes {
		if matchRoutePattern(pattern, path) {
			return t
		}
	}
	return rtc.defaultTimeout
}

// matchRoutePattern сопоставляет конкретный путь с паттерном вида
// "/api/v1/projects/{id}/calculate" — сегменты {...} матчат любые значения.
func matchRoutePattern(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	pSegs := strings.Split(strings.Trim(pattern, "/"), "/")
	rSegs := strings.Split(strings.Trim(path, "/"), "/")
	if len(pSegs) != len(rSegs) {
		return false
	}
	for i := range pSegs {
		p := pSegs[i]
		if p == "" {
			return false
		}
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			continue
		}
		if p != rSegs[i] {
			return false
		}
	}
	return true
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
// Пути должны совпадать с фактически зарегистрированными маршрутами
// (router.go): с паттернами {id}/{configID} и двумя двоеточиями в
// stairs:calculate / stairs:optimize.
func DefaultAPIRouteTimeouts() *RouteTimeoutConfig {
	cfg := NewRouteTimeoutConfig(30 * time.Second)

	// Быстрые эндпоинты
	cfg.Set("/health", 5*time.Second)
	cfg.Set("/ready", 5*time.Second)

	// Средние эндпоинты
	cfg.Set("/api/v1/auth/login", 10*time.Second)
	cfg.Set("/api/v1/auth/register", 10*time.Second)
	cfg.Set("/api/v1/auth/me", 10*time.Second)

	// Долгие операции (расчёт/оптимизация/экспорт) — паттерны с двоеточием
	cfg.Set("/api/v1/stairs:calculate", 60*time.Second)
	cfg.Set("/api/v1/stairs:optimize", 60*time.Second)
	cfg.Set("/api/v1/stairs:calculate/async", 60*time.Second)
	cfg.Set("/api/v1/projects/{id}/calculate", 60*time.Second)
	cfg.Set("/api/v1/projects/{id}/preview", 60*time.Second)
	cfg.Set("/api/v1/projects/{id}/optimize", 60*time.Second)
	cfg.Set("/api/v1/projects/{id}/export", 120*time.Second)
	cfg.Set("/api/v1/projects/{id}/export/cad", 120*time.Second)

	return cfg
}
