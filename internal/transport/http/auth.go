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
type AuthService interface {
	Register(ctx context.Context, email, name, password string) (*auth.User, string, error)
	Login(ctx context.Context, email, password string) (*auth.User, string, error)
	Authenticate(ctx context.Context, token string) (*auth.User, error)
	Logout(ctx context.Context, token string) error
}

// Имена cookie (SEC-0003). session — httpOnly, его читает только сервер;
// csrf — доступен JS (double-submit).
const (
	sessionCookieName = "session"
	csrfCookieName    = "csrf"
	csrfHeader        = "X-CSRF-Token"
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

// tenantID возвращает tenant аутентифицированного пользователя.
func tenantID(ctx context.Context) string {
	if u := authUser(ctx); u != nil {
		return u.TenantID
	}
	return ""
}

// userID возвращает ID аутентифицированного пользователя (инициатора
// запроса); пустая строка — не аутентифицирован.
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

func toUserDTO(u *auth.User) userDTO {
	return userDTO{ID: u.ID, Email: u.Email, Name: u.Name, Role: string(u.Role), TenantID: u.TenantID}
}

// ---- handlers ----

// handleRegister — POST /api/v1/auth/register (public).
// 201 — пользователь создан (сессия выставлена, авто-вход);
// 409 — email занят; 422 — невалидный вход.
func handleRegister(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		u, token, err := svc.Register(r.Context(), req.Email, req.Name, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrEmailExists):
				writeError(w, http.StatusConflict, "email_exists", "email already registered")
			case errors.Is(err, auth.ErrInvalidEmail), errors.Is(err, auth.ErrWeakPassword):
				writeError(w, http.StatusUnprocessableEntity, "invalid_input", err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
			return
		}
		setSessionCookies(w, token)
		writeJSON(w, http.StatusCreated, toUserDTO(u))
	}
}

// handleLogin — POST /api/v1/auth/login (public). Успех устанавливает
// session (httpOnly) и csrf cookie; клиент читает csrf и шлёт его в
// X-CSRF-Token на мутирующие запросы (double-submit).
// 200 — вход; 401 — неверные учётные данные; 422 — невалидный вход.
func handleLogin(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		u, token, err := svc.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrInvalidCreds):
				writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			case errors.Is(err, auth.ErrUserDisabled):
				writeError(w, http.StatusForbidden, "user_disabled", "account disabled")
			default:
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
			return
		}
		setSessionCookies(w, token)
		writeJSON(w, http.StatusOK, toUserDTO(u))
	}
}

// handleLogout — POST /api/v1/auth/logout (auth+CSRF). 204 — сессия удалена.
func handleLogout(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if err := svc.Logout(r.Context(), token); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		clearSessionCookies(w)
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

// setSessionCookies выставляет session (httpOnly) и csrf (доступный JS).
// csrf — отдельный случайный nonce, не session-токен: иначе клиентский JS
// смог бы прочитать session-токен (нарушение httpOnly).
func setSessionCookies(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    newCSRF(),
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure,
	})
}

func clearSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{sessionCookieName, csrfCookieName} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1})
	}
}

func sessionToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		return c.Value
	}
	return ""
}

// newCSRF генерирует случайный nonce для double-submit CSRF.
func newCSRF() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}
