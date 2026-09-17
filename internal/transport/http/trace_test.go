package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTraceMiddleware(t *testing.T) {
	h := TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("want 200 got %d", rec.Code)
	}

	// 500 path
	h2 := TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, httptest.NewRequest("GET", "/err", nil))
	if rec2.Code != 500 {
		t.Fatalf("want 500 got %d", rec2.Code)
	}

	// slow request via Write without WriteHeader (status 0 -> 200)
	h3 := TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hi"))
	}))
	rec3 := httptest.NewRecorder()
	h3.ServeHTTP(rec3, httptest.NewRequest("GET", "/slow", nil))
	if rec3.Code != 200 {
		t.Fatalf("want 200 got %d", rec3.Code)
	}
}

func TestTraceContextMiddleware(t *testing.T) {
	h := TraceContextMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("want 200 got %d", rec.Code)
	}
}

func TestTraceStatusWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &traceStatusWriter{ResponseWriter: rec}
	w.WriteHeader(201)
	if w.status != 201 {
		t.Fatalf("want 201 got %d", w.status)
	}
	if w.Unwrap() != rec {
		t.Fatal("unwrap")
	}
	rec2 := httptest.NewRecorder()
	w2 := &traceStatusWriter{ResponseWriter: rec2}
	_, _ = w2.Write([]byte("hi"))
	if w2.status != 200 {
		t.Fatalf("Write without WriteHeader should set 200, got %d", w2.status)
	}
}
