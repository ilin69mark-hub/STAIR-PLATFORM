package http

import (
	"context"
	"net/http"
	"time"
)

// TimeoutMiddleware добавляет context deadline к запросам.
// Если handler не завершается за timeout — клиент получает 504 Gateway Timeout.
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TimeoutWithDefault возвращает TimeoutMiddleware с дефолтным timeout (30s).
func TimeoutWithDefault() func(http.Handler) http.Handler {
	return TimeoutMiddleware(30 * time.Second)
}

// TimeoutAPI возвращает TimeoutMiddleware для API запросов (10s).
func TimeoutAPI() func(http.Handler) http.Handler {
	return TimeoutMiddleware(10 * time.Second)
}

// TimeoutLong возвращает TimeoutMiddleware для долгих операций (60s).
func TimeoutLong() func(http.Handler) http.Handler {
	return TimeoutMiddleware(60 * time.Second)
}

// TimeoutContextDuration возвращает оставшееся время из контекста.
func TimeoutContextDuration(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		return time.Until(deadline)
	}
	return 0
}
