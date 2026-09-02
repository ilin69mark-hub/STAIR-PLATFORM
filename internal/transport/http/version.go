package http

import (
	"context"
	"net/http"
	"strings"
)

// ctxKey для version context.
type versionCtxKey string

const apiVersionKey versionCtxKey = "api_version"

// DefaultVersion — текущая версия API по умолчанию.
const DefaultVersion = "v1"

// supportedMajorVersions — список поддерживаемых major versions.
var supportedMajorVersions = map[int]bool{1: true}

// VersionMiddleware определяет версию API из path/header/query.
// Приоритет: path /api/v{N}/... > Header Accept > Query ?api_version=
// Неподдерживаемая версия → 400.
func VersionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := extractVersion(r)
		if version == "" {
			version = DefaultVersion
		}

		major := parseMajorVersion(version)
		if !supportedMajorVersions[major] {
			writeError(w, http.StatusBadRequest, "unsupported_version",
				"API version '"+version+"' is not supported. Use v1.")
			return
		}

		ctx := context.WithValue(r.Context(), apiVersionKey, version)
		w.Header().Set("X-API-Version", version)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// APIVersionFromContext возвращает версию API из контекста.
func APIVersionFromContext(ctx context.Context) string {
	if v := ctx.Value(apiVersionKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return DefaultVersion
}

// extractVersion извлекает версию из запроса (path → header → query).
func extractVersion(r *http.Request) string {
	// 1. Path: /api/v1/...
	if idx := strings.Index(r.URL.Path, "/api/v"); idx >= 0 {
		rest := r.URL.Path[idx+6:]
		end := strings.IndexAny(rest, "/?")
		if end == -1 {
			end = len(rest)
		}
		if end > 0 {
			return "v" + rest[:end]
		}
	}

	// 2. Header: Accept: application/vnd.stair.v1+json
	if accept := r.Header.Get("Accept"); accept != "" {
		if idx := strings.Index(accept, "vnd.stair."); idx >= 0 {
			rest := accept[idx+10:]
			if plusIdx := strings.Index(rest, "+"); plusIdx > 0 {
				return rest[:plusIdx]
			}
		}
	}

	// 3. Query: ?api_version=v1
	if v := r.URL.Query().Get("api_version"); v != "" {
		return v
	}

	return ""
}

// parseMajorVersion из "v1" → 1, "v2" → 2.
func parseMajorVersion(v string) int {
	if len(v) < 2 || v[0] != 'v' {
		return 0
	}
	n := 0
	for _, c := range v[1:] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
