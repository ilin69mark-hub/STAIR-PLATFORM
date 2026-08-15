package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stairplatform/internal/application/stair"
)

// fakeReadiness — контролируемая проба для тестов /ready.
type fakeReadiness struct {
	ready  bool
	checks map[string]string
}

func (f fakeReadiness) Ready(ctx context.Context) (bool, map[string]string) {
	return f.ready, f.checks
}

func TestReadyOK(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Readiness = fakeReadiness{
		ready:  true,
		checks: map[string]string{"database": "ok", "redis": "ok"},
	}
	r := NewRouter(stair.NewService(), nil, testAuth{}, cfg)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf(`expected status "ok", got %v`, body["status"])
	}
}

func TestReadyUnavailable(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Readiness = fakeReadiness{
		ready:  false,
		checks: map[string]string{"database": "error: connection refused"},
	}
	r := NewRouter(stair.NewService(), nil, testAuth{}, cfg)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if body["status"] != "unavailable" {
		t.Fatalf(`expected status "unavailable", got %v`, body["status"])
	}
}

func TestReadyNotRegisteredWhenNil(t *testing.T) {
	r := NewRouter(stair.NewService(), nil, testAuth{}, DefaultConfig())

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Без Readiness проба не регистрируется → 404.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHealthAlwaysOKEvenIfReadyFails(t *testing.T) {
	// /health не зависит от Readiness (liveness vs readiness, EDR-0018 §3.1).
	cfg := DefaultConfig()
	cfg.Readiness = fakeReadiness{ready: false, checks: map[string]string{"database": "error"}}
	r := NewRouter(stair.NewService(), nil, testAuth{}, cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// TestHealthIncludesRegion: region из конфига отражается в /health
// (EDR-0019 §6).
func TestHealthIncludesRegion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Region = "eu-central-1"
	r := NewRouter(stair.NewService(), nil, testAuth{}, cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if body["region"] != "eu-central-1" {
		t.Fatalf(`expected region "eu-central-1", got %v`, body["region"])
	}
}
