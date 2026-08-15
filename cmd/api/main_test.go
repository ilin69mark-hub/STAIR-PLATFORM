package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewRootHandlerPProfDisabled: pprof выключен по умолчанию —
// /debug/pprof/ отдаёт 404 через корневой catch-all (B2, EDR-0033 §3.3).
func TestNewRootHandlerPProfDisabled(t *testing.T) {
	t.Setenv("STAIR_PPROF_ENABLED", "")
	root := newRootHandler(http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("pprof must be disabled by default, got %d", rec.Code)
	}
}

// TestNewRootHandlerPProfEnabled: при STAIR_PPROF_ENABLED=true индекс
// pprof доступен.
func TestNewRootHandlerPProfEnabled(t *testing.T) {
	t.Setenv("STAIR_PPROF_ENABLED", "true")
	root := newRootHandler(http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pprof index expected 200, got %d", rec.Code)
	}
}
