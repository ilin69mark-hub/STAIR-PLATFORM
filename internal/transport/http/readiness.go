package http

import (
	"context"
	"net/http"
	"time"
)

// ReadinessChecker — пробы готовности инстанса (EDR-0018 §3.2). Возвращает
// состояние каждой проверки и признак готовности. Реализация предоставляется
// cmd/api (pgxpool SELECT 1, go-redis PING); transport не зависит от
// конкретных клиентов.
type ReadinessChecker interface {
	// Ready выполняет пробы зависимостей и возвращает готовность инстанса.
	// checks — имя проверки → "ok" или описание ошибки.
	Ready(ctx context.Context) (ready bool, checks map[string]string)
}

// handleReady — GET /ready (readiness, EDR-0018 §3.1). В отличие от /health,
// проверяет зависимости (БД, Redis). 503 — инстанс не готов (не выводить в
// ротацию); 200 — готов.
func handleReady(checker ReadinessChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		ready, checks := checker.Ready(ctx)
		payload := map[string]any{"checks": checks}
		if !ready {
			payload["status"] = "unavailable"
			writeJSON(w, http.StatusServiceUnavailable, payload)
			return
		}
		payload["status"] = "ok"
		writeJSON(w, http.StatusOK, payload)
	}
}
