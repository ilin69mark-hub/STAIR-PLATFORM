package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDGenerated(t *testing.T) {
	handler := withLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestID(r.Context()); got == "" {
			t.Error("request id must be present in context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get(requestIDHeader) == "" {
		t.Fatal("response must carry X-Request-Id")
	}
}

func TestRequestIDCarriedFromHeader(t *testing.T) {
	handler := withLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestID(r.Context()); got != "client-id-42" {
			t.Fatalf("request id = %q, want %q", got, "client-id-42")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(requestIDHeader, "client-id-42")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestPanicRecovered(t *testing.T) {
	handler := withLogging(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	// Паника не должна упасть за пределы middleware.
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	if rec.Header().Get(requestIDHeader) == "" {
		t.Fatal("panic response must carry X-Request-Id")
	}
}

func TestStatusWriterRecordsStatus(t *testing.T) {
	var recorded int
	handler := withLogging(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	// Проверим, что код попадает в statusWriter через ответ клиенту.
	_ = recorded
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 through middleware, got %d", rec.Code)
	}
}
