package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// NewRouter собирает маршруты API v1. svc — прикладной сервис расчёта,
// projects — прикладной сервис управления проектами, authSvc — сервис
// аутентификации (инверсия зависимостей, DOM-0008); nil недопустимы.
// Все /api/v1-маршруты, кроме регистрации и входа, требуют аутентификации
// (SEC-0003); мутирующие — CSRF (double-submit). Роутер оборачивает все
// маршруты middleware логгирования/request-id (наблюдаемость).
func NewRouter(svc StairService, projects ProjectService, authSvc AuthService, cfg Config, auditSvc ...AuditService) http.Handler {
	// P2-11: все ранее глобальные переключатели (rate-limit, cookie, секреты,
	// region) живут как локальные значения инстанса роутера — никаких
	// applyConfig-мутаций пакетного состояния.
	if cfg.LoginRateLimit <= 0 {
		cfg.LoginRateLimit = DefaultConfig().LoginRateLimit
	}
	if cfg.LoginRateWindow <= 0 {
		cfg.LoginRateWindow = DefaultConfig().LoginRateWindow
	}
	if cfg.RegisterRateLimit <= 0 {
		cfg.RegisterRateLimit = DefaultConfig().RegisterRateLimit
	}
	if cfg.RegisterRateWindow <= 0 {
		cfg.RegisterRateWindow = DefaultConfig().RegisterRateWindow
	}
	if cfg.QuoteRateLimit <= 0 {
		cfg.QuoteRateLimit = DefaultConfig().QuoteRateLimit
	}
	if cfg.QuoteRateWindow <= 0 {
		cfg.QuoteRateWindow = DefaultConfig().QuoteRateWindow
	}
	if cfg.ValidateRateLimit <= 0 {
		cfg.ValidateRateLimit = DefaultConfig().ValidateRateLimit
	}
	if cfg.ValidateRateWindow <= 0 {
		cfg.ValidateRateWindow = DefaultConfig().ValidateRateWindow
	}
	if cfg.AuthRateLimit <= 0 {
		cfg.AuthRateLimit = DefaultConfig().AuthRateLimit
	}
	if cfg.AuthRateWindow <= 0 {
		cfg.AuthRateWindow = DefaultConfig().AuthRateWindow
	}
	if cfg.SsoRateLimit <= 0 {
		cfg.SsoRateLimit = DefaultConfig().SsoRateLimit
	}
	if cfg.SsoRateWindow <= 0 {
		cfg.SsoRateWindow = DefaultConfig().SsoRateWindow
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = DefaultConfig().MaxBodyBytes
	}
	SetMetricsConfig(cfg.Region, cfg.InstanceID)

	loginLimiter := newRateLimiterStrategy(context.Background(), cfg.RedisAddr, cfg.LoginRateLimit, cfg.LoginRateWindow)
	registerLimiter := newRateLimiterStrategy(context.Background(), cfg.RedisAddr, cfg.RegisterRateLimit, cfg.RegisterRateWindow)
	quoteLimiter := newRateLimiterStrategy(context.Background(), cfg.RedisAddr, cfg.QuoteRateLimit, cfg.QuoteRateWindow)
	validateLimiter := newRateLimiterStrategy(context.Background(), cfg.RedisAddr, cfg.ValidateRateLimit, cfg.ValidateRateWindow)
	// Authenticated rate limiter: 200 req/min per user/API key
	authRateLimiter := newRateLimiterStrategy(context.Background(), cfg.RedisAddr, cfg.AuthRateLimit, cfg.AuthRateWindow)
	// SSO rate limiter: публичные begin/callback/config (S-109).
	ssoLimiter := newRateLimiterStrategy(context.Background(), cfg.RedisAddr, cfg.SsoRateLimit, cfg.SsoRateWindow)
	secure := cfg.CookieSecure

	var auditsvc AuditService
	if len(auditSvc) > 0 {
		auditsvc = auditSvc[0]
	}
	integrations := cfg.Integrations
	readiness := cfg.Readiness
	storage := cfg.Storage
	payments := cfg.Payments
	analytics := cfg.Analytics
	jobsSvc := cfg.Jobs
	assistantSvc := cfg.Assistant
	ordersSvc := cfg.Orders

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth(cfg.Region))
	mux.Handle("GET /metrics", InternalOnlyMiddleware(http.HandlerFunc(handleMetrics)))
	if readiness != nil {
		mux.HandleFunc("GET /ready", handleReady(readiness))
	}
	// Swagger UI (internal only)
	mux.Handle("GET /swagger", InternalOnlyMiddleware(http.HandlerFunc(handleSwaggerUI)))
	mux.Handle("GET /docs/openapi/swagger.yaml", InternalOnlyMiddleware(http.HandlerFunc(handleSwaggerSpec)))
	mux.Handle("POST /api/v1/auth/register", limitRate(registerLimiter, handleRegister(authSvc, secure)))
	mux.Handle("POST /api/v1/auth/login", limitRate(loginLimiter, handleLogin(authSvc, secure)))

	// Публичный расчёт предварительной цены для клиентского сайта (store).
	// Без аутентификации; rate-limiter защищает от злоупотреблений.
	mux.Handle("POST /api/v1/public/stairs:quote", limitRate(quoteLimiter, handlePublicQuote(svc)))
	// Живая валидация при вводе для клиентского сайта (S-P5): тот же блок
	// validation, что и в расчёте, но без геометрии/производства/цены.
	mux.Handle("POST /api/v1/public/stairs:validate", limitRate(validateLimiter, handleValidate(svc)))
	// Публичная консультация (store): анонимный запрос обратной связи.
	// Создаёт заказ-лид kind=consultation без пользователя и цены.
	mux.Handle("POST /api/v1/public/orders", limitRate(quoteLimiter, handleCreateConsultation(ordersSvc, authSvc)))

	authProtected := func(next http.Handler) http.Handler {
		return requireAuth(authSvc, authRateLimiter, secure)(next)
	}
	authMutating := func(next http.Handler) http.Handler {
		return requireAuth(authSvc, authRateLimiter, secure)(requireCSRF(next))
	}

	mux.Handle("GET /api/v1/auth/me", authProtected(handleMe()))
	mux.Handle("POST /api/v1/auth/logout", authMutating(handleLogout(authSvc, secure)))

	// SSO (EDR-0017 §6): публичные маршруты (начала и колбэк). Rate-limited
	// по IP (S-109), чтобы исключить спам и неограниченный рост sso_states.
	mux.Handle("GET /api/v1/auth/sso", limitRate(ssoLimiter, handleSsoBegin(authSvc)))
	mux.Handle("GET /api/v1/auth/sso/callback", limitRate(ssoLimiter, handleSsoCallback(authSvc, secure)))
	mux.Handle("GET /api/v1/auth/sso/config", limitRate(ssoLimiter, handleSsoConfig(authSvc)))

	mux.Handle("GET /api/v1/admin/users", authProtected(handleListUsers(authSvc)))
	mux.Handle("PATCH /api/v1/admin/users/{id}", authMutating(handleUpdateUser(authSvc)))
	mux.Handle("GET /api/v1/admin/overview", authProtected(handleAdminOverview(authSvc, projects)))
	mux.Handle("GET /api/v1/admin/settings", authProtected(handleGetSettings(authSvc)))
	mux.Handle("PUT /api/v1/admin/settings", authMutating(handleUpdateSettings(authSvc, auditsvc)))
	mux.Handle("GET /api/v1/admin/export", authProtected(handleExport(authSvc, projects, auditsvc)))
	mux.Handle("GET /api/v1/admin/api-keys", authProtected(handleListApiKeys(authSvc)))
	mux.Handle("POST /api/v1/admin/api-keys", authMutating(handleCreateApiKey(authSvc)))
	mux.Handle("DELETE /api/v1/admin/api-keys/{id}", authMutating(handleRevokeApiKey(authSvc)))

	if auditsvc != nil {
		mux.Handle("GET /api/v1/audit", authProtected(handleListTenantAudit(auditsvc)))
		mux.Handle("GET /api/v1/projects/{id}/audit", authProtected(handleListProjectAudit(projects, auditsvc)))
		// Клиентские события (клики, применение вариантов) — mutating (CSRF).
		mux.Handle("POST /api/v1/audit", authMutating(handleRecordAudit(auditsvc)))
	}

	mux.Handle("POST /api/v1/stairs:calculate", authProtected(handleCalculate(svc, auditsvc)))
	mux.Handle("POST /api/v1/stairs:validate", authProtected(handleValidate(svc)))
	mux.Handle("POST /api/v1/stairs:optimize", authProtected(handleOptimize(svc)))
	if assistantSvc != nil {
		// AI-ассистенты (Phase D, D1–D4): design/engineering/manufacturing/pricing.
		mux.Handle("POST /api/v1/assistant/{kind}", authProtected(handleAssistantAsk(assistantSvc)))
	}
	if ordersSvc != nil {
		// Розничные заказы (клиентский сайт, Store). Клиентский кабинет и
		// админ-раздел для менеджера.
		mux.Handle("POST /api/v1/orders", authMutating(handleCreateOrder(ordersSvc)))
		mux.Handle("GET /api/v1/orders", authProtected(handleListMyOrders(ordersSvc)))
		mux.Handle("GET /api/v1/admin/orders", authProtected(handleAdminListOrders(ordersSvc)))
		mux.Handle("PATCH /api/v1/admin/orders/{id}/status", authMutating(handleAdminUpdateOrderStatus(ordersSvc, auditsvc)))
	}
	if testimonials := cfg.Testimonials; testimonials != nil {
		// Отзывы клиентов (Store): публичные — для лендинга, admin — для менеджера.
		mux.Handle("GET /api/v1/public/testimonials", handlePublicListTestimonials(testimonials, authSvc))
		mux.Handle("GET /api/v1/admin/testimonials", authProtected(handleAdminListTestimonials(testimonials)))
		mux.Handle("POST /api/v1/admin/testimonials", authMutating(handleAdminCreateTestimonial(testimonials, auditsvc)))
		mux.Handle("PATCH /api/v1/admin/testimonials/{id}", authMutating(handleAdminUpdateTestimonial(testimonials, auditsvc)))
		mux.Handle("DELETE /api/v1/admin/testimonials/{id}", authMutating(handleAdminDeleteTestimonial(testimonials, auditsvc)))
	}
	if jobsSvc != nil {
		mux.Handle("POST /api/v1/stairs:calculate/async", authMutating(handleCalculateAsync(jobsSvc)))
		mux.Handle("GET /api/v1/jobs/{id}", authProtected(handleGetJob(jobsSvc)))
	}
	mux.Handle("POST /api/v1/projects", authMutating(handleCreateProject(projects)))
	mux.Handle("GET /api/v1/projects", authProtected(handleListProjects(projects)))
	mux.Handle("GET /api/v1/projects/{id}", authProtected(handleGetProject(projects)))
	mux.Handle("GET /api/v1/projects/{id}/members", authProtected(handleListMembers(projects)))
	mux.Handle("POST /api/v1/projects/{id}/members", authMutating(handleAddMember(projects)))
	mux.Handle("PATCH /api/v1/projects/{id}/members/{userID}", authMutating(handleUpdateMemberRole(projects)))
	mux.Handle("DELETE /api/v1/projects/{id}/members/{userID}", authMutating(handleRemoveMember(projects)))
	mux.Handle("GET /api/v1/projects/{id}/comments", authProtected(handleListComments(projects)))
	mux.Handle("POST /api/v1/projects/{id}/comments", authMutating(handleAddComment(projects)))
	mux.Handle("DELETE /api/v1/projects/{id}/comments/{commentID}", authMutating(handleDeleteComment(projects)))
	mux.Handle("GET /api/v1/projects/{id}/reviews", authProtected(handleListReviews(projects)))
	mux.Handle("POST /api/v1/projects/{id}/review", authMutating(handleRequestReview(projects)))
	mux.Handle("POST /api/v1/projects/{id}/reviews/{reviewID}/sign-off", authMutating(handleSignOffReview(projects)))
	mux.Handle("POST /api/v1/projects/{id}/reviews/{reviewID}/changes", authMutating(handleRequestChanges(projects)))
	mux.Handle("GET /api/v1/projects/{id}/approvals", authProtected(handleListApprovals(projects)))
	mux.Handle("POST /api/v1/projects/{id}/configurations/{configID}/approve", authMutating(handleApproveConfiguration(projects)))
	mux.Handle("GET /api/v1/projects/{id}/configurations/{configID}/approval", authProtected(handleGetConfigurationApproval(projects)))
	mux.Handle("GET /api/v1/projects/{id}/configurations", authProtected(handleListConfigurations(projects)))
	mux.Handle("GET /api/v1/projects/{id}/configurations/{configID}", authProtected(handleGetConfiguration(projects)))
	mux.Handle("POST /api/v1/projects/{id}/configurations/{configID}/restore", authMutating(handleRestoreConfiguration(projects)))
	mux.Handle("POST /api/v1/projects/{id}/calculate", authMutating(handleCalculateProject(projects)))
	mux.Handle("POST /api/v1/projects/{id}/preview", authMutating(handlePreviewProject(projects)))
	mux.Handle("POST /api/v1/projects/{id}/optimize", authMutating(handleOptimizeProject(projects)))
	mux.Handle("GET /api/v1/projects/{id}/export", authProtected(handleExportProject(projects)))
	mux.Handle("GET /api/v1/projects/{id}/export/cad", authProtected(handleExportCAD(projects)))

	if storage != nil {
		mux.Handle("POST /api/v1/projects/{id}/export/cad/store", authMutating(handleStoreExportCAD(projects, storage)))
		mux.Handle("GET /api/v1/storage/{key...}", authProtected(handleGetObject(storage)))
		mux.Handle("DELETE /api/v1/storage/{key...}", authMutating(handleDeleteObject(storage)))
	}

	if integrations != nil {
		mux.Handle("GET /api/v1/integrations/endpoints", authProtected(handleListEndpoints(integrations)))
		mux.Handle("POST /api/v1/integrations/endpoints", authMutating(handleCreateEndpoint(integrations)))
		mux.Handle("DELETE /api/v1/integrations/endpoints/{id}", authMutating(handleDeleteEndpoint(integrations)))
		mux.Handle("POST /api/v1/projects/{id}/quote-send", authMutating(handleQuoteSend(projects, integrations)))
		mux.Handle("POST /api/v1/projects/{id}/crm-sync", authMutating(handleProjectSync(projects, integrations)))
		mux.Handle("POST /api/v1/projects/{id}/order-send", authMutating(handleOrderSend(projects, integrations)))
	}

	if payments != nil {
		// Входящий webhook PSP публичный (вызывает внешняя система); публичность
		// безопасна — подпись верифицируется (EDR-0027 §3.4).
		mux.HandleFunc("POST /api/v1/payments/webhook", handlePaymentWebhook(payments, cfg.PaymentsWebhookSecret))
		mux.Handle("POST /api/v1/projects/{id}/checkout", authMutating(handleCheckout(projects, payments)))
		mux.Handle("GET /api/v1/projects/{id}/payments", authProtected(handleListPayments(projects, payments)))
		mux.Handle("GET /api/v1/payments/{id}", authProtected(handleGetPayment(payments)))
	}

	// Stripe webhook endpoint (публичный; Stripe-Signature верификация).
	if cfg.StripeWebhookService != nil {
		mux.HandleFunc("POST /api/v1/payments/stripe/webhook", handleStripeWebhook(cfg.StripeWebhookService))
	}

	if analytics != nil {
		mux.Handle("GET /api/v1/admin/analytics/usage", authProtected(handleUsageAnalytics(analytics)))
		mux.Handle("GET /api/v1/admin/analytics/projects", authProtected(handleProjectsAnalytics(analytics)))
		mux.Handle("GET /api/v1/admin/analytics/manufacturing", authProtected(handleManufacturingAnalytics(analytics)))
		mux.Handle("GET /api/v1/admin/analytics/cost", authProtected(handleCostAnalytics(analytics)))
	}

	// WebSocket endpoint (EDR-0038). /ws — store-namespace, /ws/admin —
	// admin-namespace (S-116): браузерный WebSocket API не умеет ставить
	// заголовок X-App-Origin, поэтому namespace определяется путём.
	if cfg.WebSocketHandler != nil {
		mux.HandleFunc("GET /ws", cfg.WebSocketHandler.HandleWebSocket)
		mux.HandleFunc("GET /ws/admin", cfg.WebSocketHandler.HandleWebSocket)
	}

	// GraphQL НЕ регистрируется в production: schema обещает 15 операций,
	// а реализованы только 5 (substring-диспатч), Subscription-рут не реализован.
	// Пакет internal/transport/graphql остаётся как протестированная библиотека
	// для будущей полноценной реализации (см. docs/09_API). Не выставлять в проде,
	// пока не реализованы все операции и валидация schema.graphql.

	mux.HandleFunc("GET /", handleNotFound)

	// Cache policies по путям.
	// S-107: /api/v1/projects/ намеренно НЕ указан — проекты всегда
	// аутентифицированы и не должны попадать в браузерный кеш (наследует
	// "/api/v1/" → no-store). Дополнительно CacheMiddleware принудительно
	// снимает кеш с Private-политик при наличии identity.
	cachePolicies := map[string]CachePolicy{
		"/api/v1/":             CacheNoCache,   // API — no cache
		"/api/v1/configs/":     CacheShort,     // конфигурации — 5 min
		"/api/v1/assortments/": CacheMedium,    // ассортимент — 1 hour
		"/api/v1/materials/":   CacheMedium,    // материалы — 1 hour
		"/api/v1/profiles/":    CacheMedium,    // профили — 1 hour
		"/api/v1/stairs/":      CacheShort,     // лестницы — 5 min
		"/api/v1/auth/":        CacheNoCache,   // авторизация — no cache
		"/api/v1/health":       CacheNoCache,   // health check — no cache
		"/static/":             CacheLong,      // статика — 1 day
		"/assets/":             CacheImmutable, // webpack assets — immutable
	}

	// Response cache для read-heavy GET-запросов (5min TTL, 512 entries)
	respCache := ResponseCacheMiddleware(ResponseCacheConfig{
		MaxEntries: 512,
		DefaultTTL: 5 * time.Minute,
		Methods:    []string{http.MethodGet},
	})

	// Deduplication для тяжёлых операций
	dedup := DeduplicateMiddleware(DeduplicateByKey)

	// Security middleware (CORS, HSTS, CSP)
	secMiddleware := SecurityMiddleware(cfg.SecurityConfig)

	// Middleware-конвейер: строится изнутри наружу (inner → outer).
	h := withLogging(mux)
	h = secMiddleware(h)
	h = dedup(h)
	h = respCache(h)
	h = CacheMiddleware(cachePolicies)(h)
	h = BodySizeLimit(cfg.MaxBodyBytes)(h)
	h = CompressionMiddleware(h)
	h = RouteTimeoutMiddleware(DefaultAPIRouteTimeouts())(h)
	h = PanicRecoveryMiddleware(h)
	h = VersionMiddleware(h)
	return TraceMiddleware(h)
}

func handleHealth(region string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"status":  "ok",
			"service": "stair-platform-api",
			"version": "1.0.0",
		}
		if region != "" {
			resp["region"] = region
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": "not_found",
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// Логируем ошибку кодирования —响应 будет обрезан/повреждён.
		slog.Error("writeJSON: encode failed", "error", err)
	}
}
