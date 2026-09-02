package http

import (
	"net/http"
	"sync"
	"time"
)

// DebugCBStatus статус circuit breaker для debug endpoint.
type DebugCBStatus struct {
	Name           string `json:"name"`
	State          string `json:"state"`
	Failures       int    `json:"failures"`
	Successes      int    `json:"successes"`
	LastFailure    string `json:"last_failure,omitempty"`
}

// circuitBreakerRegistry глобальный реестр CB для debug endpoint.
var circuitBreakerRegistry = struct {
	mu      sync.RWMutex
	breakers map[string]*DebugCBStatus
}{
	breakers: make(map[string]*DebugCBStatus),
}

// RegisterCB регистрирует circuit breaker для debug endpoint.
func RegisterCB(name string, status *DebugCBStatus) {
	circuitBreakerRegistry.mu.Lock()
	defer circuitBreakerRegistry.mu.Unlock()
	circuitBreakerRegistry.breakers[name] = status
}

// handleDebugCircuitBreakers — GET /debug/circuit-breakers
func handleDebugCircuitBreakers(w http.ResponseWriter, _ *http.Request) {
	circuitBreakerRegistry.mu.RLock()
	defer circuitBreakerRegistry.mu.RUnlock()

	breakers := make([]DebugCBStatus, 0, len(circuitBreakerRegistry.breakers))
	for _, b := range circuitBreakerRegistry.breakers {
		breakers = append(breakers, *b)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"circuit_breakers": breakers,
		"count":            len(breakers),
	})
}

// HandleDebugCache — GET /debug/cache
func HandleDebugCache(cache *responseCache) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"entries": cache.Size(),
			"max":     cache.config.MaxEntries,
			"ttl":     cache.config.DefaultTTL.String(),
		})
	}
}

// handleDebugRoutes — GET /debug/routes
func handleDebugRoutes(w http.ResponseWriter, _ *http.Request) {
	routes := []string{
		"GET /api/v1/health",
		"GET /api/v1/ready",
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"GET /api/v1/projects",
		"POST /api/v1/projects",
		"GET /api/v1/stairs",
		"POST /api/v1/stairs",
		"POST /api/v1/stairs/calculate",
		"GET /metrics",
		"GET /debug/pprof/",
		"GET /debug/circuit-breakers",
		"GET /debug/cache",
		"GET /debug/routes",
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"routes": routes,
		"count":  len(routes),
		"time":   time.Now().Format(time.RFC3339),
	})
}
