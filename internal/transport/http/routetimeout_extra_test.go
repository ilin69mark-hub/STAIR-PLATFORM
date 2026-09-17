package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatchRoutePatternExtra(t *testing.T) {
	if !matchRoutePattern("/api/v1/projects/{id}/calculate", "/api/v1/projects/5/calculate") {
		t.Fatal("should match")
	}
	if matchRoutePattern("/api/v1/projects/{id}/calculate", "/api/v1/projects/5/other") {
		t.Fatal("should not match")
	}
	if matchRoutePattern("", "/a") {
		t.Fatal("empty pattern should be false")
	}
	if matchRoutePattern("/a/b", "/a") {
		t.Fatal("len mismatch")
	}
}

func TestRouteTimeoutMiddlewareZero(t *testing.T) {
	cfg := NewRouteTimeoutConfig(0)
	cfg.Set("/fast", 0)
	h := RouteTimeoutMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, hasDeadline := r.Context().Deadline(); hasDeadline {
			t.Error("zero timeout should not set deadline")
		}
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/fast", nil))
	if rec.Code != 200 {
		t.Fatalf("want 200 got %d", rec.Code)
	}
}
