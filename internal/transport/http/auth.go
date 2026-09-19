package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"stairplatform/internal/application/auth"
)

// AuthService — прикладной интерфейс auth, ожидаемый транспортным слоем
// (инверсия зависимостей, DOM-0008). Реализуется application/auth.Service.
// Включает управление пользователями (EDR-0015 §3.4) и Enterprise Controls
// (EDR-0016: статус, политики, API-ключи).
type AuthService interface {
	Register(ctx context.Context, email, name, password string) (*auth.User, string, error)
	Login(ctx context.Context, email, password string) (*auth.User, string, error)
	// Authenticate возвращает пользователя и, при ротации сессии
	// (EDR-0014), новый session-токен для обновления cookie.
	Authenticate(ctx context.Context, token string) (*auth.User, string, error)
	Logout(ctx context.Context, token string) error
	ListUsers(ctx context.Context, tenantID string) ([]*auth.User, error)
	UpdateUserRole(ctx context.Context, tenantID, actorID, userID string, role auth.Role) error
	// UpdateUser меняет роль и/или статус пользователя (EDR-0016 §3.1).
	UpdateUser(ctx context.Context, tenantID, actorID, userID string, role *auth.Role, status *auth.Status) error
	// GetPolicy возвращает политику безопасности tenant (EDR-0016 §3.2).
	GetPolicy(ctx context.Context, tenantID string) (auth.Policy, error)
	// UpdatePolicy сохраняет политику безопасности tenant.
	UpdatePolicy(ctx context.Context, tenantID, actorID string, p auth.Policy) error
	// AuthenticateApiKey проверяет service-токен (Bearer, EDR-0016 §7).
	AuthenticateApiKey(ctx context.Context, token string) (*auth.ApiKey, error)
	// CreateApiKey создаёт API-ключ; открытый токен отдаётся один раз.
	CreateApiKey(ctx context.Context, tenantID, actorID, name string, scopes []auth.Permission) (*auth.ApiKey, string, error)
	// ListApiKeys возвращает ключи tenant.
	ListApiKeys(ctx context.Context, tenantID string) ([]*auth.ApiKey, error)
	// RevokeApiKey отзывает ключ (мягко).
	RevokeApiKey(ctx context.Context, tenantID, actorID, keyID string) error
	// SsoAuthorizeURL начинает SSO-вход (EDR-0017 §6): возвращает URL IdP.
	SsoAuthorizeURL(ctx context.Context, redirect string) (string, error)
	// SsoCallback завершает SSO-вход; возвращает пользователя и session-токен.
	SsoCallback(ctx context.Context, code, state string) (*auth.User, string, error)
	// SsoEnabled возвращает публичную конфигурацию SSO.
	SsoEnabled() auth.SsoConfig
	// DefaultTenant возвращает дефолтный tenant (slug "default"); используется
	// публичными маршрутами (регистрация, консультации) без аутентификации.
	DefaultTenant(ctx context.Context) (*auth.Tenant, error)
}

// Имена cookie (SEC-0003). session — httpOnly, его читает только сервер;
// csrf — доступен JS (double-submit). Имена виртуально origin-специфичны:
// admin-приложение использует суффикс «_admin» (cookie на localhost
// host-only, порт в scope не входит — RFC 6265 §5.1.3), чтобы store:3000
// и admin:5174 не делили одну сессию. store — дефолт (без суффикса,
// обратная совместимость с curl/e2e).
const (
	sessionCookieName = "session"
	csrfCookieName    = "csrf"
	// adminOriginCookieSuffix — суффикс имени cookie для admin-приложения.
	adminOriginCookieSuffix = "_admin"
	// appOriginHeader — заголовок, которым фронт сообщает своё приложение.
	appOriginHeader = "X-App-Origin"
	appOriginStore  = "store"
	appOriginAdmin  = "admin"
	csrfHeader      = "X-CSRF-Token"
)

// ---- контекст авторизации ----

type authCtxKey struct{}

// withAuthUser кладёт пользователя в контекст запроса.
func withAuthUser(ctx context.Context, u *auth.User) context.Context {
	return context.WithValue(ctx, authCtxKey{}, u)
}

// authUser возвращает пользователя из контекста (nil — не аутентифицирован).
func authUser(ctx context.Context) *auth.User {
	u, _ := ctx.Value(authCtxKey{}).(*auth.User)
	return u
}

// ---- контекст API-ключ ----

type apiKeyCtxKey struct{}

// withApiKey кладёт API-ключ в контекст запроса (Bearer, EDR-0016 §7).
func withApiKey(ctx context.Context, k *auth.ApiKey) context.Context {
	return context.WithValue(ctx, apiKeyCtxKey{}, k)
}

// apiKey возвращает API-ключ из контекста (nil — session-аутентификация).
func apiKey(ctx context.Context) *auth.ApiKey {
	k, _ := ctx.Value(apiKeyCtxKey{}).(*auth.ApiKey)
	return k
}

// hasPermission проверяет право субъекта запроса (EDR-0015/0016):
// пользователя по его роли или API-ключа по его scopes.
func hasPermission(r *http.Request, p auth.Permission) bool {
	if k := apiKey(r.Context()); k != nil {
		return k.HasScope(p)
	}
	if u := authUser(r.Context()); u != nil {
		return u.Role.HasPermission(p)
	}
	return false
}

// tenantID возвращает tenant аутентифицированного субъекта (пользователя
// или API-ключа).
func tenantID(ctx context.Context) string {
	if k := apiKey(ctx); k != nil {
		return k.TenantID
	}
	if u := authUser(ctx); u != nil {
		return u.TenantID
	}
	return ""
}

// userID возвращает ID аутентифицированного пользователя (инициатора
// запроса); пустая строка — не аутентифицирован или API-ключ.
func userID(ctx context.Context) string {
	if u := authUser(ctx); u != nil {
		return u.ID
	}
	return ""
}

// ---- DTO ----

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userDTO struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

// authResponse — ответ при register/login (содержит токен в body для e2e клиентов).
type authResponse struct {
	User  userDTO `json:"user"`
	Token string  `json:"token"`
}

func toUserDTO(u *auth.User) userDTO {
	return userDTO{ID: u.ID, Email: u.Email, Name: u.Name, Role: string(u.Role), TenantID: u.TenantID}
}

// ---- handlers ----

// handleRegister — POST /api/v1/auth/register (public).
// 201 — пользователь создан (сессия выставлена, авто-вход);
// 409 — email занят; 422 — невалидный вход.
func handleRegister(svc AuthService, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		u, token, err := svc.Register(r.Context(), req.Email, req.Name, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrEmailExists):
				writeError(w, http.StatusConflict, "email_exists", "Пользователь с таким email уже зарегистрирован")
			case errors.Is(err, auth.ErrInvalidEmail), errors.Is(err, auth.ErrWeakPassword):
				writeInputError(w, "invalid_input", err)
			default:
				writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			}
			return
		}
		setSessionCookies(w, appOrigin(r), token, secure)
		writeJSON(w, http.StatusCreated, authResponse{User: toUserDTO(u), Token: token})
	}
}

// handleLogin — POST /api/v1/auth/login (public). Успех устанавливает
// session (httpOnly) и csrf cookie; клиент читает csrf и шлёт его в
// X-CSRF-Token на мутирующие запросы (double-submit).
// 200 — вход; 401 — неверные учётные данные; 422 — невалидный вход.
func handleLogin(svc AuthService, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Некорректный JSON в теле запроса")
			return
		}
		u, token, err := svc.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrInvalidCreds):
				writeError(w, http.StatusUnauthorized, "invalid_credentials", "Неверный email или пароль.")
			case errors.Is(err, auth.ErrUserDisabled):
				writeError(w, http.StatusForbidden, "user_disabled", "Аккаунт отключён.")
			default:
				writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			}
			return
		}
		setSessionCookies(w, appOrigin(r), token, secure)
		writeJSON(w, http.StatusOK, authResponse{User: toUserDTO(u), Token: token})
	}
}

// handleLogout — POST /api/v1/auth/logout (auth+CSRF). 204 — сессия удалена.
func handleLogout(svc AuthService, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if err := svc.Logout(r.Context(), token); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		clearSessionCookies(w, appOrigin(r), secure)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleMe — GET /api/v1/auth/me (auth). 200 — текущий пользователь.
func handleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, toUserDTO(authUser(r.Context())))
	}
}

// ---- cookie helpers ----

// --- cookie helpers ---

// appOrigin возвращает идентификатор приложения по заголовку X-App-Origin
// (браузерный клиент проставляет store|admin). Пустой/неизвестный —
// «store» (обратная совместимость: curl, e2e-тесты, api-ключи не шлют
// заголовок и остаются в дефолтном namespace).
func appOrigin(r *http.Request) string {
	switch r.Header.Get(appOriginHeader) {
	case appOriginAdmin:
		return appOriginAdmin
	default:
		return appOriginStore
	}
}

// sessionCookieFor возвращает имя session-cookie для приложения (origin).
// store — «session» (без суффикса, обратная совместимость); admin —
// «session_admin». Разные имена критично: cookie host-only на localhost
// не различает порты (RFC 6265 §5.1.3), поэтому store:3000 и admin:5174
// иначе делили бы одну сессию (выход в одном приложении инвалидировал бы
// сессию во втором).
func sessionCookieFor(origin string) string {
	if origin == appOriginAdmin {
		return sessionCookieName + adminOriginCookieSuffix
	}
	return sessionCookieName
}

// csrfCookieFor возвращает имя csrf-cookie для приложения (origin).
func csrfCookieFor(origin string) string {
	if origin == appOriginAdmin {
		return csrfCookieName + adminOriginCookieSuffix
	}
	return csrfCookieName
}

// originCookieNames возвращает оба имени (session, csrf) приложения.
func originCookieNames(origin string) (session, csrf string) {
	return sessionCookieFor(origin), csrfCookieFor(origin)
}

// setSessionCookies выставляет session (httpOnly) и csrf (доступный JS).
func setSessionCookies(w http.ResponseWriter, origin string, token string, secure bool) {
	setSessionCookie(w, origin, token, secure)
	// CSRF-cookie намеренно доступен JS (double-submit) и не является session-токеном.
	http.SetCookie(w, &http.Cookie{ // #nosec G124 // CSRF-cookie: HttpOnly=false намеренно (double-submit); Secure/sameSite заданы
		Name:     csrfCookieFor(origin),
		Value:    newCSRF(),
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

// setSessionCookie выставляет только session-cookie (httpOnly). Используется
// при ротации сессии (EDR-0014 §3.1), когда csrf-cookie менять не нужно.
func setSessionCookie(w http.ResponseWriter, origin string, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{ // #nosec G124 // Secure управляется STAIR_COOKIE_SECURE
		Name:     sessionCookieFor(origin),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		Expires:  time.Now().Add(24 * time.Hour),
	})
}

func clearSessionCookies(w http.ResponseWriter, origin string, secure bool) {
	session, csrf := originCookieNames(origin)
	for _, name := range []string{session, csrf} {
		http.SetCookie(w, &http.Cookie{ // #nosec G124 // Secure управляется STAIR_COOKIE_SECURE
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
		})
	}
}

func sessionToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieFor(appOrigin(r))); err == nil {
		return c.Value
	}
	return ""
}

// newCSRF генерирует случайный nonce для double-submit CSRF.
// crypto/rand failure означает критический сбой системы — паника.
func newCSRF() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("csrf: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
