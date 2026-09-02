package http

import (
	"net/http"
	"strings"
	"sync"
)

// DeduplicateMiddleware предотвращает одновременную обработку
// одинаковых запросов (по ключу). Если запрос уже в обработке —
// возвращает 429 Too Many Requests.
// Подходит для тяжёлых операций (расчёт, экспорт, генерация).
func DeduplicateMiddleware(keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	inflight := make(map[string]*sync.Mutex)
	mu := sync.Mutex{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			mu.Lock()
			if m, ok := inflight[key]; ok {
				mu.Unlock()
				// Already in-flight — wait or reject
				if !m.TryLock() {
					w.Header().Set("Retry-After", "5")
					http.Error(w, "duplicate request in progress", http.StatusTooManyRequests)
					return
				}
				defer m.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			// New request — register
			m := &sync.Mutex{}
			m.Lock()
			inflight[key] = m
			mu.Unlock()

			defer func() {
				mu.Lock()
				delete(inflight, key)
				mu.Unlock()
				m.Unlock()
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// DeduplicateByKey возвращает ключ для дедупликации по user_id + path.
func DeduplicateByKey(r *http.Request) string {
	uid := ""
	if id := r.Context().Value("user_id"); id != nil {
		uid, _ = id.(string)
	}
	if uid == "" {
		uid = dedupIP(r)
	}
	return uid + ":" + r.Method + ":" + r.URL.Path
}

// dedupIP извлекает IP из RemoteAddr.
func dedupIP(r *http.Request) string {
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}
