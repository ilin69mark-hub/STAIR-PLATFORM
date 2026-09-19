package http

import (
	"net/http"
	"sync"
)

// DebugCBStatus статус circuit breaker для debug endpoint.
type DebugCBStatus struct {
	Name        string `json:"name"`
	State       string `json:"state"`
	Failures    int    `json:"failures"`
	Successes   int    `json:"successes"`
	LastFailure string `json:"last_failure,omitempty"`
}

// circuitBreakerRegistry глобальный реестр CB для debug endpoint.
var circuitBreakerRegistry = struct {
	mu       sync.RWMutex
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
