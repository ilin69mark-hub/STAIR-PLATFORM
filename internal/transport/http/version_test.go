package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionMiddleware_PathExtraction(t *testing.T) {
	handler := VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := APIVersionFromContext(r.Context())
		_, _ = w.Write([]byte(v))
	}))

	tests := []struct {
		path    string
		version string
	}{
		{"/api/v1/projects", "v1"},
		{"/api/v1/auth/login", "v1"},
		{"/api/v1/stairs/123", "v1"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Body.String() != tt.version {
			t.Errorf("path %q: expected version %q, got %q", tt.path, tt.version, w.Body.String())
		}
	}
}

func TestVersionMiddleware_HeaderExtraction(t *testing.T) {
	handler := VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := APIVersionFromContext(r.Context())
		_, _ = w.Write([]byte(v))
	}))

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req.Header.Set("Accept", "application/vnd.stair.v1+json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Body.String() != "v1" {
		t.Errorf("expected v1 from header, got %q", w.Body.String())
	}
}

func TestVersionMiddleware_QueryExtraction(t *testing.T) {
	handler := VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := APIVersionFromContext(r.Context())
		_, _ = w.Write([]byte(v))
	}))

	req := httptest.NewRequest(http.MethodGet, "/projects?api_version=v1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Body.String() != "v1" {
		t.Errorf("expected v1 from query, got %q", w.Body.String())
	}
}

func TestVersionMiddleware_DefaultVersion(t *testing.T) {
	handler := VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := APIVersionFromContext(r.Context())
		_, _ = w.Write([]byte(v))
	}))

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Body.String() != "v1" {
		t.Errorf("expected default v1, got %q", w.Body.String())
	}
}

func TestVersionMiddleware_UnsupportedVersion(t *testing.T) {
	handler := VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for unsupported version")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v99/projects", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unsupported version, got %d", w.Code)
	}
}

func TestVersionMiddleware_ResponseHeader(t *testing.T) {
	handler := VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-API-Version") != "v1" {
		t.Errorf("expected X-API-Version v1 header, got %q", w.Header().Get("X-API-Version"))
	}
}

func TestVersionMiddleware_PathPriorityOverHeader(t *testing.T) {
	handler := VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := APIVersionFromContext(r.Context())
		_, _ = w.Write([]byte(v))
	}))

	// Path says v1, header says v2 — path wins
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Accept", "application/vnd.stair.v2+json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Body.String() != "v1" {
		t.Errorf("expected path priority v1, got %q", w.Body.String())
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name string
		path string
		accept string
		query string
		want string
	}{
		{"path only", "/api/v1/projects", "", "", "v1"},
		{"header only", "/projects", "application/vnd.stair.v1+json", "", "v1"},
		{"query only", "/projects", "", "api_version=v1", "v1"},
		{"no version", "/projects", "", "", ""},
		{"v2 path", "/api/v2/stairs", "", "", "v2"},
		{"path priority", "/api/v1/projects", "application/vnd.stair.v2+json", "api_version=v3", "v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			if tt.query != "" {
				req.URL.RawQuery = tt.query
			}
			got := extractVersion(req)
			if got != tt.want {
				t.Errorf("extractVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMajorVersion(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"v1", 1},
		{"v2", 2},
		{"v10", 10},
		{"", 0},
		{"1", 0},
		{"vx", 0},
		{"v1a", 0},
	}
	for _, tt := range tests {
		if got := parseMajorVersion(tt.input); got != tt.want {
			t.Errorf("parseMajorVersion(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestAPIVersionFromContext_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	v := APIVersionFromContext(req.Context())
	if v != "v1" {
		t.Errorf("expected default v1 from empty context, got %q", v)
	}
}
