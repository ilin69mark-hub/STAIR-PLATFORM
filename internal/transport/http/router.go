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
func NewRouter(svc StairService, projects ProjectService, authSvc AuthService, cfg Config) http.Handler {
	applyConfig(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /api/v1/auth/register", handleRegister(authSvc))
	mux.Handle("POST /api/v1/auth/login", limitRate(loginLimiter, handleLogin(authSvc)))

	authProtected := func(next http.Handler) http.Handler {
		return requireAuth(authSvc)(next)
	}
	authMutating := func(next http.Handler) http.Handler {
		return requireAuth(authSvc)(requireCSRF(next))
	}

	mux.Handle("GET /api/v1/auth/me", authProtected(handleMe()))
	mux.Handle("POST /api/v1/auth/logout", authMutating(handleLogout(authSvc)))

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
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = DefaultConfig().MaxBodyBytes
	}
	maxBodyBytes = cfg.MaxBodyBytes
	loginLimiter = newRateLimiter(cfg.LoginRateLimit, cfg.LoginRateWindow)
}

var loginLimiter *rateLimiter

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "stair-platform-api",
	})
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
