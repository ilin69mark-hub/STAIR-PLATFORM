package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorLoggingMiddleware(t *testing.T) {
	// 200 -> no log, just pass
	h := ErrorLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("want 200 got %d", rec.Code)
	}
	// 400 log warn
	h2 := ErrorLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400) }))
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, httptest.NewRequest("GET", "/bad", nil))
	if rec2.Code != 400 {
		t.Fatalf("want 400 got %d", rec2.Code)
	}
	// 500 log error
	h3 := ErrorLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	rec3 := httptest.NewRecorder()
	h3.ServeHTTP(rec3, httptest.NewRequest("GET", "/err", nil))
	if rec3.Code != 500 {
		t.Fatalf("want 500 got %d", rec3.Code)
	}
}

func TestErrorStatusWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &errorStatusWriter{ResponseWriter: rec, statusCode: 200}
	w.WriteHeader(201)
	if w.statusCode != 201 {
		t.Fatalf("want 201 got %d", w.statusCode)
	}
	if w.Unwrap() != rec {
		t.Fatal("unwrap mismatch")
	}
}
