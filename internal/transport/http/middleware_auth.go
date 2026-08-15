package http

import (
	"net/http"
	"net/url"
	"strings"
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
	// RegisterRateLimit — максимум регистраций с одного IP за окно.
	RegisterRateLimit int
	// RegisterRateWindow — окно rate-limit регистрации.
	RegisterRateWindow time.Duration
	// RedisAddr — адрес Redis для распределённого лимитера; пусто — memory.
	RedisAddr string
	// MaxBodyBytes — предельный размер тела запроса (защита от DoS).
	MaxBodyBytes int64
	// InstanceID — идентификатор реплики (STAIR_INSTANCE_ID, EDR-0018 §3.5);
	// пусто — не логируется.
	InstanceID string
	// ShutdownTimeout — таймаут graceful shutdown (STAIR_SHUTDOWN_TIMEOUT).
	// Используется не HTTP-слоем, а cmd/api при остановке сервера.
	ShutdownTimeout time.Duration
	// Readiness — пробы готовности (EDR-0018 §3.2); nil — /ready не
	// регистрируется (инстанс всегда «не проверяем»).
	Readiness ReadinessChecker
	// Region — идентификатор региона инстанса (STAIR_REGION, EDR-0019 §3.1);
	// отражается в /health. Пусто — регион не задан.
	Region string
	// Integrations — сервис интеграций (EDR-0023 §3.4); nil — маршруты
	// integrations/quote-send не регистрируются.
	Integrations IntegrationService
	// Storage — сервис объектного хранилища (EDR-0026 §3.5); nil — маршруты
	// storage/export-cad не регистрируются.
	Storage StorageService
	// Payments — сервис платежей (EDR-0027 §3.3); nil — маршруты
	// payments/checkout/webhook не регистрируются.
	Payments PaymentService
	// PaymentsWebhookSecret — секрет верификации входящего webhook PSP
	// (STAIR_PAYMENT_WEBHOOK_SECRET); пусто — webhook отклоняется (401).
	PaymentsWebhookSecret string
	// Analytics — сервис аналитики (EDR-0028 §3.4, Phase F); nil —
	// маршруты analytics не регистрируются.
	Analytics AnalyticsService
	// Jobs — сервис фоновых заданий (EDR-0035, Phase B B4); nil —
	// маршруты stairs:calculate/async и jobs/{id} не регистрируются.
	Jobs JobsService
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() Config {
	return Config{
		CookieSecure:       false,
		LoginRateLimit:     10,
		LoginRateWindow:    time.Minute,
		RegisterRateLimit:  5,
		RegisterRateWindow: time.Minute,
		MaxBodyBytes:       1 << 20, // 1 MiB
	}
}

// requireAuth — обязательная аутентификация (SEC-0003). Принимает либо
// session-cookie (браузер), либо Authorization: Bearer <api-key> (EDR-0016
// §7, интеграции). Читает токен, проверяет через auth.Service и кладёт
// субъект в контекст. При ротации сессии (EDR-0014 §3.1) обновляет
// session-cookie новым токеном. 401 — нет/невалидная сессия.
func requireAuth(svc AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// API-ключ (Bearer): не-браузерный клиент, CSRF не требуется.
			if bearer := bearerToken(r); bearer != "" {
				key, err := svc.AuthenticateApiKey(r.Context(), bearer)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "unauthorized", "invalid api key")
					return
				}
				next.ServeHTTP(w, r.WithContext(withApiKey(r.Context(), key)))
				return
			}
			token := sessionToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			u, rotatedToken, err := svc.Authenticate(r.Context(), token)
			if err != nil {
				clearSessionCookies(w)
				writeError(w, http.StatusUnauthorized, "unauthorized", "session expired or invalid")
				return
			}
			if rotatedToken != "" {
				// Сессия ротирована: выдаём новый session-cookie (httpOnly).
				setSessionCookie(w, rotatedToken)
			}
			next.ServeHTTP(w, r.WithContext(withAuthUser(r.Context(), u)))
		})
	}
}

// bearerToken извлекает opaque-токен из Authorization: Bearer <token>.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// requireCSRF — защита от CSRF для мутирующих запросов (double-submit):
// заголовок X-CSRF-Token должен совпадать с csrf-cookie. Дополнительно
// (EDR-0014 §3.3) проверяется Origin/Referer запроса.
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
		if !csrfOriginAllowed(r) {
			writeError(w, http.StatusForbidden, "csrf", "cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// csrfOriginAllowed проверяет источник запроса (EDR-0014 §3.3): Origin,
// если заголовок есть, иначе Referer. Host источника должен совпадать с
// host запроса; при отсутствии обоих заголовков (не-браузерный клиент)
// запрос пропускается.
func csrfOriginAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// RateLimiter — стратегия лимитирования по ключу (IP). Интерфейс позволяет
// подменять реализацию: memory (fallback) и redis (EDR-0014 §3.2).
type RateLimiter interface {
	// Allow возвращает true, если запрос с ключом разрешён.
	Allow(ip string) bool
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

// Allow — реализация RateLimiter (делегирует allow).
func (l *rateLimiter) Allow(ip string) bool { return l.allow(ip) }

// limitRate ограничивает число запросов с одного IP (анти-брутфорс
// login/register, SEC-0003; EDR-0014 §3.2).
func limitRate(l RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(clientIP(r)) {
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
