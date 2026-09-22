package http

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// cookieSecure — Secure-флаг cookie. По умолчанию off (локальная разработка
// на http://localhost); включается через STAIR_COOKIE_SECURE=1.
// Прокидывается в хендлеры параметром из NewRouter(cfg.CookieSecure) —
// глобал-переменная не используется (P2-11).

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
	// QuoteRateLimit — максимум публичных расчётов цены с одного IP за окно
	// (STAIR_QUOTE_RATE_LIMIT); защищает публичный quote-эндпоинт от спама.
	QuoteRateLimit int
	// QuoteRateWindow — окно rate-limit публичного расчёта.
	QuoteRateWindow time.Duration
	// ValidateRateLimit — максимум живых валидаций при вводе с одного IP за
	// окно (STAIR_VALIDATE_RATE_LIMIT); защищает публичный validate-эндпоинт
	// (самый частый запрос конструктора при вводе, S-P5).
	ValidateRateLimit int
	// ValidateRateWindow — окно rate-limit живой валидации.
	ValidateRateWindow time.Duration
	// AuthRateLimit — максимум авторизованных запросов с одного пользователя
	// за окно (STAIR_AUTH_RATE_LIMIT); по умолчанию 200 req/min (router.go).
	AuthRateLimit int
	// AuthRateWindow — окно rate-limit авторизованных запросов.
	AuthRateWindow time.Duration
	// SsoRateLimit — максимум запросов к публичным SSO-роутам с одного IP за
	// окно (STAIR_SSO_RATE_LIMIT); защищает begin/callback от спама и
	// неограниченного роста sso_states. По умолчанию 20 req/min.
	SsoRateLimit int
	// SsoRateWindow — окно rate-limit SSO-роутов.
	SsoRateWindow time.Duration
	// RedisAddr — адрес Redis для распределённого лимитера; пусто — memory.
	RedisAddr string
	// TrustedProxies — доверенные прокси (STAIR_TRUSTED_PROXIES, CSV из CIDR
	// или одиночных IP, напр. "10.0.0.0/8, 1.2.3.4"). Только если прямой пир
	// (RemoteAddr) входит в этот список, rate-limiter берёт IP клиента из
	// X-Forwarded-For (S-120). Пусто — XFF игнорируется везде (дефолт S-112:
	// защита от спуфинга при прямом доступе).
	TrustedProxies string
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
	// Assistant — сервис AI-ассистентов (EDR-0036, Phase D D1); nil —
	// маршруты assistant/{kind} не регистрируются.
	Assistant AssistantService
	// Orders — сервис розничных заказов (клиентский сайт); nil — маршруты
	// orders и admin/orders не регистрируются.
	Orders OrderService
	// Testimonials — сервис отзывов клиентов (клиентский сайт); nil —
	// маршруты testimonials и admin/testimonials не регистрируются.
	Testimonials TestimonialService
	// WebSocketHandler — handler для WebSocket подключений; nil —
	// маршрут /ws не регистрируется.
	WebSocketHandler *WebSocketHandler
	// SecurityConfig — конфигурация security middleware; nil —
	// security headers не добавляются.
	SecurityConfig *SecurityConfig
	// StripeWebhookService — сервис обработки Stripe webhook; nil —
	// маршрут /api/v1/payments/stripe/webhook не регистрируется.
	StripeWebhookService StripeWebhookService
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() Config {
	return Config{
		CookieSecure:       false,
		LoginRateLimit:     10,
		LoginRateWindow:    time.Minute,
		RegisterRateLimit:  5,
		RegisterRateWindow: time.Minute,
		QuoteRateLimit:     30,
		QuoteRateWindow:    time.Minute,
		ValidateRateLimit:  120,
		ValidateRateWindow: time.Minute,
		AuthRateLimit:      200,
		AuthRateWindow:     time.Minute,
		SsoRateLimit:       20,
		SsoRateWindow:      time.Minute,
		MaxBodyBytes:       1 << 20, // 1 MiB
	}
}

// requireAuth — обязательная аутентификация (SEC-0003). Принимает либо
// session-cookie (браузер), либо Authorization: Bearer <api-key> (EDR-0016
// §7, интеграции). Читает токен, проверяет через auth.Service и кладёт
// субъект в контекст. При ротации сессии (EDR-0014 §3.1) обновляет
// session-cookie новым токеном. 401 — нет/невалидная сессия.
// rateLimiter — per-user/api-key лимит; secure — флаг Secure для cookie
// (оба инстанс-зависимые значения из NewRouter, P2-11).
func requireAuth(svc AuthService, rateLimiter RateLimiter, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// API-ключ (Bearer): не-браузерный клиент, CSRF не требуется.
			if bearer := bearerToken(r); bearer != "" {
				key, err := svc.AuthenticateApiKey(r.Context(), bearer)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "unauthorized", "Неверный API-ключ.")
					return
				}
				// Per-API-key rate limiting
				if rateLimiter != nil {
					if !rateLimiter.Allow("apikey:" + key.ID) {
						w.Header().Set("Retry-After", "60")
						writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded",
							"Превышен лимит запросов для API ключа.")
						return
					}
				}
				next.ServeHTTP(w, r.WithContext(withApiKey(r.Context(), key)))
				return
			}
			token := sessionToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
				return
			}
			u, rotatedToken, err := svc.Authenticate(r.Context(), token)
			if err != nil {
				clearSessionCookies(w, appOrigin(r), secure)
				writeError(w, http.StatusUnauthorized, "unauthorized", "Сессия истекла или недействительна.")
				return
			}
			// Per-user rate limiting для authenticated запросов
			if rateLimiter != nil {
				if !rateLimiter.Allow("user:" + u.ID) {
					w.Header().Set("Retry-After", "60")
					writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded",
						"Превышен лимит запросов.")
					return
				}
			}
			if rotatedToken != "" {
				// Сессия ротирована: выдаём новый session-cookie (httpOnly).
				setSessionCookie(w, appOrigin(r), rotatedToken, secure)
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
// заголовок X-CSRF-Token должен совпадать с csrf-cookie приложения
// (csrfCookieFor(appOrigin(r)) — store: «csrf», admin: «csrf_admin»).
// Дополнительно (EDR-0014 §3.3) проверяется Origin/Referer запроса.
func requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(csrfCookieFor(appOrigin(r)))
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusForbidden, "csrf", "Требуется CSRF-токен.")
			return
		}
		if r.Header.Get(csrfHeader) != cookie.Value {
			writeError(w, http.StatusForbidden, "csrf", "CSRF-токен не совпадает.")
			return
		}
		if !csrfOriginAllowed(r) {
			writeError(w, http.StatusForbidden, "csrf", "Запрос с другого источника отклонён.")
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
	cancel   context.CancelFunc
}

type rateBucket struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(ctx context.Context, limit int, window time.Duration) *rateLimiter {
	ctx, cancel := context.WithCancel(ctx)
	l := &rateLimiter{
		limit:    limit,
		window:   window,
		attempts: make(map[string]*rateBucket),
		cancel:   cancel,
	}
	go l.cleanup(ctx)
	return l
}

// cleanup.periodically removes expired entries to prevent memory leak.
func (l *rateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(l.window / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for ip, b := range l.attempts {
				if now.After(b.resetAt) {
					delete(l.attempts, ip)
				}
			}
			l.mu.Unlock()
		}
	}
}

// Stop останавливает cleanup goroutine.
func (l *rateLimiter) Stop() {
	if l.cancel != nil {
		l.cancel()
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
func limitRate(l RateLimiter, trusted []*net.IPNet, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(rateLimitIP(r, trusted)) {
			// Добавляем Retry-After header (стандарт для 429)
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много запросов, попробуйте позже")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP извлекает IP клиента из RemoteAddr.
// X-Forwarded-For не используется, чтобы rate limiting нельзя было
// обойти через поддельный заголовок (SEC).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Если нет порта — это просто host.
		return r.RemoteAddr
	}
	return host
}

// parseTrustedProxies разбирает CSV из CIDR ("10.0.0.0/8") или одиночных IP
// ("1.2.3.4" → /32, IPv6 → /128) в список сетей (S-120). Невалидные записи
// пропускаются; пустая строка — nil (XFF не доверяется никому).
func parseTrustedProxies(csv string) []*net.IPNet {
	var out []*net.IPNet
	for _, s := range strings.Split(csv, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if strings.Contains(s, "/") {
			if _, n, err := net.ParseCIDR(s); err == nil {
				out = append(out, n)
			}
			continue
		}
		if ip := net.ParseIP(s); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
		}
	}
	return out
}

// rateLimitIP — ключ per-IP лимитера (S-120). Если прямой пир (RemoteAddr)
// входит в trusted-прокси и запрос несёт X-Forwarded-For — берём самый левый
// (исходный клиент; прокси дописывают себя справа). Иначе — RemoteAddr
// (поведение S-112). Непарсящийся XFF — fallback на RemoteAddr.
func rateLimitIP(r *http.Request, trusted []*net.IPNet) string {
	if len(trusted) > 0 {
		if peer := net.ParseIP(clientIP(r)); peer != nil {
			for _, n := range trusted {
				if n.Contains(peer) {
					if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
						first := strings.TrimSpace(strings.Split(xff, ",")[0])
						if net.ParseIP(first) != nil {
							return first
						}
					}
					break
				}
			}
		}
	}
	return clientIP(r)
}
