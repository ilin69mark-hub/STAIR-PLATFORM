package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeprecatedMiddleware(t *testing.T) {
	mw := DeprecatedMiddleware(DeprecationInfo{Sunset: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), Message: "use v2", Link: "https://example.com"})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	req := httptest.NewRequest("GET", "/old", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Deprecation") != "true" {
		t.Fatal("missing Deprecation")
	}
	if rec.Header().Get("Sunset") == "" {
		t.Fatal("missing Sunset")
	}
	if rec.Header().Get("Link") == "" {
		t.Fatal("missing Link")
	}
	// no link case
	mw2 := DeprecatedMiddleware(DeprecationInfo{Sunset: time.Now()})
	h2 := mw2(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, httptest.NewRequest("GET", "/old", nil))
	if rec2.Header().Get("Link") != "" {
		t.Fatal("link should be empty")
	}
}

func TestDeprecatedResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	DeprecatedResponse(rec, "gone")
	if rec.Code != 410 {
		t.Fatalf("want 410 got %d", rec.Code)
	}
	if rec.Body.String() == "" {
		t.Fatal("empty body")
	}
}
