package http

import (
	"encoding/json"
	"net/http"
)

// NewRouter собирает маршруты API v1. svc — прикладной сервис расчёта,
// projects — прикладной сервис управления проектами, authSvc — сервис
// аутентификации (инверсия зависимостей, DOM-0008); nil недопустимы.
// Все /api/v1-маршруты, кроме регистрации и входа, требуют аутентификации
// (SEC-0003); мутирующие — CSRF (double-submit). Роутер оборачивает все
// маршруты middleware логгирования/request-id (наблюдаемость).
func NewRouter(svc StairService, projects ProjectService, authSvc AuthService, cfg Config, auditSvc ...AuditService) http.Handler {
	applyConfig(cfg)

	var auditsvc AuditService
	if len(auditSvc) > 0 {
		auditsvc = auditSvc[0]
	}
	integrations := cfg.Integrations
	readiness := cfg.Readiness
	storage := cfg.Storage
	payments := cfg.Payments
	analytics := cfg.Analytics

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /metrics", handleMetrics)
	if readiness != nil {
		mux.HandleFunc("GET /ready", handleReady(readiness))
	}
	mux.Handle("POST /api/v1/auth/register", limitRate(registerLimiter, handleRegister(authSvc)))
	mux.Handle("POST /api/v1/auth/login", limitRate(loginLimiter, handleLogin(authSvc)))

	authProtected := func(next http.Handler) http.Handler {
		return requireAuth(authSvc)(next)
	}
	authMutating := func(next http.Handler) http.Handler {
		return requireAuth(authSvc)(requireCSRF(next))
	}

	mux.Handle("GET /api/v1/auth/me", authProtected(handleMe()))
	mux.Handle("POST /api/v1/auth/logout", authMutating(handleLogout(authSvc)))

	// SSO (EDR-0017 §6): публичные маршруты (начала и колбэк).
	mux.HandleFunc("GET /api/v1/auth/sso", handleSsoBegin(authSvc))
	mux.HandleFunc("GET /api/v1/auth/sso/callback", handleSsoCallback(authSvc))
	mux.HandleFunc("GET /api/v1/auth/sso/config", handleSsoConfig(authSvc))

	mux.Handle("GET /api/v1/admin/users", authProtected(handleListUsers(authSvc)))
	mux.Handle("PATCH /api/v1/admin/users/{id}", authMutating(handleUpdateUser(authSvc)))
	mux.Handle("GET /api/v1/admin/overview", authProtected(handleAdminOverview(authSvc, projects)))
	mux.Handle("GET /api/v1/admin/settings", authProtected(handleGetSettings(authSvc)))
	mux.Handle("PUT /api/v1/admin/settings", authMutating(handleUpdateSettings(authSvc)))
	mux.Handle("GET /api/v1/admin/export", authProtected(handleExport(authSvc, projects, auditsvc)))
	mux.Handle("GET /api/v1/admin/api-keys", authProtected(handleListApiKeys(authSvc)))
	mux.Handle("POST /api/v1/admin/api-keys", authMutating(handleCreateApiKey(authSvc)))
	mux.Handle("DELETE /api/v1/admin/api-keys/{id}", authMutating(handleRevokeApiKey(authSvc)))

	if auditsvc != nil {
		mux.Handle("GET /api/v1/audit", authProtected(handleListTenantAudit(auditsvc)))
		mux.Handle("GET /api/v1/projects/{id}/audit", authProtected(handleListProjectAudit(projects, auditsvc)))
	}

	mux.Handle("POST /api/v1/stairs:calculate", authProtected(handleCalculate(svc)))
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
		mux.Handle("DELETE /api/v1/integrations/endpoints/{id}", authProtected(handleDeleteEndpoint(integrations)))
		mux.Handle("POST /api/v1/projects/{id}/quote-send", authProtected(handleQuoteSend(projects, integrations)))
		mux.Handle("POST /api/v1/projects/{id}/crm-sync", authProtected(handleProjectSync(projects, integrations)))
		mux.Handle("POST /api/v1/projects/{id}/order-send", authProtected(handleOrderSend(projects, integrations)))
	}

	if payments != nil {
		// Входящий webhook PSP публичный (вызывает внешняя система); публичность
		// безопасна — подпись верифицируется (EDR-0027 §3.4).
		mux.HandleFunc("POST /api/v1/payments/webhook", handlePaymentWebhook(payments))
		mux.Handle("POST /api/v1/projects/{id}/checkout", authMutating(handleCheckout(projects, payments)))
		mux.Handle("GET /api/v1/projects/{id}/payments", authProtected(handleListPayments(projects, payments)))
		mux.Handle("GET /api/v1/payments/{id}", authProtected(handleGetPayment(payments)))
	}

	if analytics != nil {
		mux.Handle("GET /api/v1/admin/analytics/usage", authProtected(handleUsageAnalytics(analytics)))
	}

	mux.HandleFunc("GET /", handleNotFound)
	return withLogging(mux)
}

// applyConfig применяет конфигурацию HTTP-слоя (глобальные переключатели
// cookie/rate-limit; вызывается один раз при сборке роутера).
func applyConfig(cfg Config) {
	cookieSecure = cfg.CookieSecure
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
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = DefaultConfig().MaxBodyBytes
	}
	maxBodyBytes = cfg.MaxBodyBytes
	region = cfg.Region
	regionLabel = cfg.Region
	instanceLabel = cfg.InstanceID
	loginLimiter = newRateLimiterStrategy(cfg.RedisAddr, cfg.LoginRateLimit, cfg.LoginRateWindow)
	registerLimiter = newRateLimiterStrategy(cfg.RedisAddr, cfg.RegisterRateLimit, cfg.RegisterRateWindow)
	paymentsWebhookSecret = cfg.PaymentsWebhookSecret
}

var (
	loginLimiter    RateLimiter
	registerLimiter RateLimiter
	region          string
	// paymentsWebhookSecret — секрет верификации входящего webhook PSP
	// (EDR-0027 §3.4); глобал из-за единственного публичного маршрута,
	// применяется один раз при сборке роутера.
	paymentsWebhookSecret string
)

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]string{
		"status":  "ok",
		"service": "stair-platform-api",
	}
	if region != "" {
		resp["region"] = region
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": "not_found",
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
