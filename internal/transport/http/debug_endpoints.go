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
		// Health / readiness
		"GET /health",
		"GET /ready",
		"GET /metrics",
		// Auth
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"GET /api/v1/auth/me",
		"POST /api/v1/auth/logout",
		"GET /api/v1/auth/sso",
		"GET /api/v1/auth/sso/callback",
		"GET /api/v1/auth/sso/config",
		// Public (store)
		"POST /api/v1/public/stairs:quote",
		"POST /api/v1/public/orders",
		"GET /api/v1/public/testimonials",
		// Admin — users, settings, overview
		"GET /api/v1/admin/users",
		"PATCH /api/v1/admin/users/{id}",
		"GET /api/v1/admin/overview",
		"GET /api/v1/admin/settings",
		"PUT /api/v1/admin/settings",
		"GET /api/v1/admin/export",
		"GET /api/v1/admin/api-keys",
		"POST /api/v1/admin/api-keys",
		"DELETE /api/v1/admin/api-keys/{id}",
		// Audit
		"GET /api/v1/audit",
		"POST /api/v1/audit",
		"GET /api/v1/projects/{id}/audit",
		// Stairs calculation
		"POST /api/v1/stairs:calculate",
		"POST /api/v1/stairs:optimize",
		"POST /api/v1/stairs:calculate/async",
		// Assistant
		"POST /api/v1/assistant/{kind}",
		// Orders
		"POST /api/v1/orders",
		"GET /api/v1/orders",
		"GET /api/v1/admin/orders",
		"PATCH /api/v1/admin/orders/{id}/status",
		// Testimonials (admin)
		"GET /api/v1/admin/testimonials",
		"POST /api/v1/admin/testimonials",
		"PATCH /api/v1/admin/testimonials/{id}",
		"DELETE /api/v1/admin/testimonials/{id}",
		// Jobs
		"GET /api/v1/jobs/{id}",
		// Projects
		"POST /api/v1/projects",
		"GET /api/v1/projects",
		"GET /api/v1/projects/{id}",
		"GET /api/v1/projects/{id}/members",
		"POST /api/v1/projects/{id}/members",
		"PATCH /api/v1/projects/{id}/members/{userID}",
		"DELETE /api/v1/projects/{id}/members/{userID}",
		"GET /api/v1/projects/{id}/comments",
		"POST /api/v1/projects/{id}/comments",
		"DELETE /api/v1/projects/{id}/comments/{commentID}",
		"GET /api/v1/projects/{id}/reviews",
		"POST /api/v1/projects/{id}/review",
		"POST /api/v1/projects/{id}/reviews/{reviewID}/sign-off",
		"POST /api/v1/projects/{id}/reviews/{reviewID}/changes",
		"GET /api/v1/projects/{id}/approvals",
		"POST /api/v1/projects/{id}/configurations/{configID}/approve",
		"GET /api/v1/projects/{id}/configurations/{configID}/approval",
		"GET /api/v1/projects/{id}/configurations",
		"GET /api/v1/projects/{id}/configurations/{configID}",
		"POST /api/v1/projects/{id}/configurations/{configID}/restore",
		"POST /api/v1/projects/{id}/calculate",
		"POST /api/v1/projects/{id}/preview",
		"POST /api/v1/projects/{id}/optimize",
		"GET /api/v1/projects/{id}/export",
		"GET /api/v1/projects/{id}/export/cad",
		"POST /api/v1/projects/{id}/export/cad/store",
		"POST /api/v1/projects/{id}/checkout",
		"GET /api/v1/projects/{id}/payments",
		// Storage
		"GET /api/v1/storage/{key...}",
		"DELETE /api/v1/storage/{key...}",
		// Integrations
		"GET /api/v1/integrations/endpoints",
		"POST /api/v1/integrations/endpoints",
		"DELETE /api/v1/integrations/endpoints/{id}",
		"POST /api/v1/projects/{id}/quote-send",
		"POST /api/v1/projects/{id}/crm-sync",
		"POST /api/v1/projects/{id}/order-send",
		// Payments
		"POST /api/v1/payments/webhook",
		"POST /api/v1/payments/stripe/webhook",
		"GET /api/v1/payments/{id}",
		// Analytics
		"GET /api/v1/admin/analytics/usage",
		"GET /api/v1/admin/analytics/projects",
		"GET /api/v1/admin/analytics/manufacturing",
		"GET /api/v1/admin/analytics/cost",
		// WebSocket / GraphQL
		"GET /ws",
		"POST /graphql",
		"GET /graphql",
		// Internal (protected by InternalOnlyMiddleware)
		"GET /swagger",
		"GET /docs/openapi/swagger.yaml",
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"routes": routes,
		"count":  len(routes),
		"time":   time.Now().Format(time.RFC3339),
	})
}
