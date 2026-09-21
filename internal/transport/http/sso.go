package http

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"stairplatform/internal/application/auth"
)

// redirectOK собирает целевой URL после колбэка/ошибки с параметрами.
func ssoTarget(path string, errMsg string) string {
	q := url.Values{}
	if errMsg != "" {
		q.Set("error", "sso_error")
		q.Set("message", errMsg)
	}
	u := &url.URL{Path: path, RawQuery: q.Encode()}
	return u.String()
}

// handleSsoBegin — GET /api/v1/auth/sso (public, EDR-0017 §6). Генерирует
// одноразовый OIDC-state и редиректит на authorization endpoint IdP.
// 302 — начало входа; 503 — SSO не сконфигурирован.
func handleSsoBegin(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		redirect := r.URL.Query().Get("redirect")
		if redirect == "" {
			redirect = "/"
		}
		authURL, err := svc.SsoAuthorizeURL(r.Context(), redirect)
		if err != nil {
			if errors.Is(err, auth.ErrSsoNotConfigured) {
				writeError(w, http.StatusServiceUnavailable, "sso_disabled", "Единый вход не настроен.")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		w.Header().Set("Location", authURL)
		w.WriteHeader(http.StatusFound)
	}
}

// handleSsoCallback — GET /api/v1/auth/sso/callback (public, EDR-0017 §3.3).
// Проверяет state, обменивает code, верифицирует id_token, создаёт сессию и
// редиректит на фронтенд. Ошибки — редирект с ?error=... (без raw-деталей).
func handleSsoCallback(svc AuthService, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" || state == "" {
			w.Header().Set("Location", ssoTarget("/", "missing authorization code or state"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		u, token, err := svc.SsoCallback(r.Context(), code, state)
		if err != nil {
			if errors.Is(err, auth.ErrSsoDenied) {
				w.Header().Set("Location", ssoTarget("/", "login denied or id_token rejected"))
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Location", ssoTarget("/", "internal error"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		setSessionCookies(w, appOrigin(r), token, secure)
		target := "/"
		if red := r.URL.Query().Get("redirect"); red != "" && isSafeRedirect(red) {
			target = red
		}
		w.Header().Set("Location", target)
		_ = u
		w.WriteHeader(http.StatusFound)
	}
}

// isSafeRedirect проверяет что redirect URL безопасен — только relative paths
// без scheme:// (защита от Open Redirect).
func isSafeRedirect(raw string) bool {
	// Разрешаем только относительные пути, начинающиеся с /.
	if len(raw) == 0 || raw[0] != '/' {
		return false
	}
	// Backslash-байпас (SSO-OPEN-REDIRECT-BACKSLASH): браузеры трактуют "\"
	// как "/", поэтому "/\evil.com" превращается в "//evil.com" — open redirect.
	// Отклоняем backslash как в raw, так и в percent-encoded (%5C) виде.
	if strings.ContainsRune(raw, '\\') || strings.Contains(strings.ToLower(raw), "%5c") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	// Отклоняем абсолютные URL (http://evil.com, //evil.com) и backslash,
	// проявившийся после percent-декодирования (%5C → \).
	if parsed.IsAbs() || (len(raw) > 1 && raw[1] == '/') || strings.ContainsRune(parsed.Path, '\\') {
		return false
	}
	return true
}

// handleSsoConfig — GET /api/v1/auth/sso/config (public). Информирует
// фронтенд о доступности SSO и имени провайдера.
func handleSsoConfig(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, svc.SsoEnabled())
	}
}
