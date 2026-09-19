package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutMiddleware_SetsDeadline(t *testing.T) {
	handler := TimeoutMiddleware(100 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Error("expected context to have deadline")
			return
		}
		remaining := time.Until(deadline)
		if remaining > 100*time.Millisecond {
			t.Errorf("expected deadline ~100ms, got %v", remaining)
		}
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTimeoutMiddleware_FastHandler(t *testing.T) {
	handler := TimeoutMiddleware(100 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTimeoutWithDefault(t *testing.T) {
	handler := TimeoutWithDefault()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTimeoutAPI(t *testing.T) {
	handler := TimeoutAPI()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTimeoutLong(t *testing.T) {
	handler := TimeoutLong()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTimeoutContextDuration_WithDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dur := TimeoutContextDuration(ctx)
	if dur < 4*time.Second || dur > 5*time.Second {
		t.Errorf("expected ~5s, got %v", dur)
	}
}

func TestTimeoutContextDuration_NoDeadline(t *testing.T) {
	dur := TimeoutContextDuration(context.Background())
	if dur != 0 {
		t.Errorf("expected 0 for no deadline, got %v", dur)
	}
}

func TestRouteTimeoutConfig_SetGet(t *testing.T) {
	cfg := NewRouteTimeoutConfig(30 * time.Second)
	cfg.Set("/api/v1/health", 5*time.Second)
	cfg.Set("/api/v1/stairs/calculate", 60*time.Second)

	if cfg.Get("/api/v1/health") != 5*time.Second {
		t.Errorf("expected 5s for health, got %v", cfg.Get("/api/v1/health"))
	}
	if cfg.Get("/api/v1/stairs/calculate") != 60*time.Second {
		t.Errorf("expected 60s for calculate, got %v", cfg.Get("/api/v1/stairs/calculate"))
	}
	if cfg.Get("/unknown") != 30*time.Second {
		t.Errorf("expected 30s default for unknown, got %v", cfg.Get("/unknown"))
	}
}

func TestRouteTimeoutMiddleware(t *testing.T) {
	cfg := NewRouteTimeoutConfig(30 * time.Second)
	cfg.Set("/fast", 50*time.Millisecond)

	handler := RouteTimeoutMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/fast", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDefaultAPIRouteTimeouts(t *testing.T) {
	cfg := DefaultAPIRouteTimeouts()

	tests := []struct {
		path    string
		timeout time.Duration
	}{
		{"/health", 5 * time.Second},
		{"/api/v1/auth/login", 10 * time.Second},
		{"/api/v1/stairs:calculate", 60 * time.Second},
		{"/api/v1/stairs:optimize", 60 * time.Second},
		{"/api/v1/projects/abc-123/calculate", 60 * time.Second},
		{"/api/v1/projects/abc-123/export/cad", 120 * time.Second},
	}
	for _, tt := range tests {
		if cfg.Get(tt.path) != tt.timeout {
			t.Errorf("Get(%q) = %v, want %v", tt.path, cfg.Get(tt.path), tt.timeout)
		}
	}
}
