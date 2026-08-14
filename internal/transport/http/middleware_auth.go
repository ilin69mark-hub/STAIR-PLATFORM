package http

import (
	"net/http"
	"sync"
	"time"
)

// cookieSecure — Secure-флаг cookie. По умолчанию off (локальная разработка
// на http://localhost); включается через STAIR_COOKIE_SECURE=1.
var cookieSecure bool

// Config — конфигурация HTTP-слоя (BE-0029 Configuration).
type Config struct {
	// CookieSecure — флаг Secure для cookie (HTTPS в проде).
	CookieSecure bool
	// LoginRateLimit — максимум попыток входа с одного IP за окно.
	LoginRateLimit int
	// LoginRateWindow — окно rate-limit входа.
	LoginRateWindow time.Duration
	// MaxBodyBytes — предельный размер тела запроса (защита от DoS).
	MaxBodyBytes int64
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() Config {
	return Config{
		CookieSecure:    false,
		LoginRateLimit:  10,
		LoginRateWindow: time.Minute,
		MaxBodyBytes:    1 << 20, // 1 MiB
	}
}

// requireAuth — обязательная аутентификация (SEC-0003). Читает session-cookie,
// проверяет токен через auth.Service и кладёт пользователя в контекст.
// 401 — нет/невалидная сессия.
func requireAuth(svc AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := sessionToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			u, err := svc.Authenticate(r.Context(), token)
			if err != nil {
				clearSessionCookies(w)
				writeError(w, http.StatusUnauthorized, "unauthorized", "session expired or invalid")
				return
			}
			next.ServeHTTP(w, r.WithContext(withAuthUser(r.Context(), u)))
		})
	}
}

// requireCSRF — защита от CSRF для мутирующих запросов (double-submit):
// заголовок X-CSRF-Token должен совпадать с csrf-cookie.
func requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusForbidden, "csrf", "csrf token required")
			return
		}
		if r.Header.Get(csrfHeader) != cookie.Value {
			writeError(w, http.StatusForbidden, "csrf", "csrf token mismatch")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimiter — простейший sliding-window лимитер по IP (в памяти).
type rateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	attempts map[string]*rateBucket
}

type rateBucket struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:    limit,
		window:   window,
		attempts: make(map[string]*rateBucket),
	}
}

// allow возвращает true, если запрос с ip разрешён.
func (l *rateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.attempts[ip]
	if !ok || now.After(b.resetAt) {
		l.attempts[ip] = &rateBucket{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	b.count++
	return b.count <= l.limit
}

// limitRate ограничивает число запросов с одного IP (анти-брутфорс
// login/register, SEC-0003).
func limitRate(l *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP извлекает IP клиента (X-Forwarded-For для прокси; RemoteAddr — base).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	host, _, err := splitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func splitHostPort(addr string) (string, string, error) {
	// Локальная реализация для избежания лишних зависимостей.
	host := addr
	port := ""
	for i := 0; i < len(addr); i++ {
		if addr[i] == ':' {
			host, port = addr[:i], addr[i+1:]
			break
		}
	}
	return host, port, nil
}
