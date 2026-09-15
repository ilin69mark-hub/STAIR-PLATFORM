package security

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"stairplatform/internal/infrastructure/metrics"
)

type userIDCtxKey string

const ctxUserID userIDCtxKey = "user_id"

var (
	// RateLimitRegistry — глобальный реестр метрик rate limiter.
	RateLimitRegistry = metrics.NewRegistry()

	// RateLimitHits — счётчик срабатываний rate limiter по endpoint.
	RateLimitHits = RateLimitRegistry.Counter(
		"rate_limit_hits_total",
		"Total rate limit hits",
		"key",
	)

	// RateLimitRequests — счётчик всех запросов через rate limiter.
	RateLimitRequests = RateLimitRegistry.Counter(
		"rate_limit_requests_total",
		"Total requests through rate limiter",
		"key",
	)

	// ActiveKeys — количество уникальных ключей в rate limiter.
	ActiveKeys = RateLimitRegistry.Gauge("rate_limit_active_keys", "Unique keys in rate limiter")
)

// RateLimiter — in-memory rate limiter с sliding window и метриками.
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	total    atomic.Int64
	blocked  atomic.Int64
}

// NewRateLimiter создаёт новый rate limiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// RateLimit middleware ограничивает количество запросов.
func (rl *RateLimiter) RateLimit(keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			remaining, resetAt, allowed := rl.AllowWithInfo(key)

			// Устанавливаем заголовки X-RateLimit-*
			w.Header().Set("X-RateLimit-Limit", intToString(rl.limit))
			w.Header().Set("X-RateLimit-Remaining", intToString(remaining))
			w.Header().Set("X-RateLimit-Reset", intToString(int(resetAt.Unix())))

			if !allowed {
				// Динамический Retry-After: секунды до сброса окна
				retryAfter := int(time.Until(resetAt).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", intToString(retryAfter))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Allow проверяет, разрешен ли запрос.
func (rl *RateLimiter) Allow(key string) bool {
	_, _, allowed := rl.AllowWithInfo(key)
	return allowed
}

// AllowWithInfo возвращает количество оставшихся запросов, время сброса и разрешение.
func (rl *RateLimiter) AllowWithInfo(key string) (remaining int, resetAt time.Time, allowed bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Очищаем старые запросы
	requests := rl.requests[key]
	validRequests := make([]time.Time, 0, len(requests))
	for _, t := range requests {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}

	rl.total.Add(1)
	RateLimitRequests.With(key).Inc()

	remaining = rl.limit - len(validRequests)
	if remaining <= 0 {
		rl.blocked.Add(1)
		RateLimitHits.With(key).Inc()
		return 0, now.Add(rl.window), false
	}

	// Добавляем текущий запрос
	validRequests = append(validRequests, now)
	rl.requests[key] = validRequests

	return remaining - 1, now.Add(rl.window), true
}

// Stats возвращает статистику rate limiter.
func (rl *RateLimiter) Stats() (total, blocked int64, activeKeys int) {
	return rl.total.Load(), rl.blocked.Load(), len(rl.requests)
}

// ByIP возвращает ключ по IP адресу.
func ByIP(r *http.Request) string {
	// Проверяем X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Берем первый IP
		for i, c := range xff {
			if c == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Проверяем X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Используем RemoteAddr
	addr := r.RemoteAddr
	if idx := len(addr) - 1; idx > 0 {
		for i := idx; i >= 0; i-- {
			if addr[i] == ':' {
				return addr[:i]
			}
		}
	}
	return addr
}

// ByEndpoint возвращает ключ по endpoint + IP.
func ByEndpoint(r *http.Request) string {
	return r.URL.Path + ":" + ByIP(r)
}

// ByUser возвращает ключ по user ID (если аутентифицирован) или IP.
func ByUser(r *http.Request) string {
	if uid := r.Context().Value(ctxUserID); uid != nil {
		if id, ok := uid.(string); ok && id != "" {
			return "user:" + id
		}
	}
	return ByIP(r)
}

// ByUserEndpoint возвращает ключ по user ID + endpoint.
func ByUserEndpoint(r *http.Request) string {
	if uid := r.Context().Value(ctxUserID); uid != nil {
		if id, ok := uid.(string); ok && id != "" {
			return "user:" + id + ":" + r.URL.Path
		}
	}
	return ByIP(r) + ":" + r.URL.Path
}

// MultiRateLimiter позволяет задать разные лимиты для разных endpoints.
type MultiRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*RateLimiter
	defaultLimiter *RateLimiter
}

// NewMultiRateLimiter создаёт multi-rate limiter с дефолтным лимитом.
func NewMultiRateLimiter(defaultLimit int, defaultWindow time.Duration) *MultiRateLimiter {
	return &MultiRateLimiter{
		limiters:       make(map[string]*RateLimiter),
		defaultLimiter: NewRateLimiter(defaultLimit, defaultWindow),
	}
}

// AddEndpoint добавляет специфичный лимитер для endpoint.
func (mrl *MultiRateLimiter) AddEndpoint(path string, limit int, window time.Duration) {
	mrl.mu.Lock()
	defer mrl.mu.Unlock()
	mrl.limiters[path] = NewRateLimiter(limit, window)
}

// RateLimit middleware с поддержкой разных лимитов для разных endpoints.
func (mrl *MultiRateLimiter) RateLimit(keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rl := mrl.getLimiter(r.URL.Path)
			key := keyFunc(r)
			remaining, resetAt, allowed := rl.AllowWithInfo(key)

			w.Header().Set("X-RateLimit-Limit", intToString(rl.limit))
			w.Header().Set("X-RateLimit-Remaining", intToString(remaining))
			w.Header().Set("X-RateLimit-Reset", intToString(int(resetAt.Unix())))

			if !allowed {
				// Динамический Retry-After: секунды до сброса окна
				retryAfter := int(time.Until(resetAt).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", intToString(retryAfter))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (mrl *MultiRateLimiter) getLimiter(path string) *RateLimiter {
	mrl.mu.RLock()
	defer mrl.mu.RUnlock()
	if rl, ok := mrl.limiters[path]; ok {
		return rl
	}
	return mrl.defaultLimiter
}

// intToString быстрое преобразование int → string без fmt.Sprintf.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
