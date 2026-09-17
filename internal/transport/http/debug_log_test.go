package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDebugLoggingMiddleware_CapturesBody(t *testing.T) {
	handler := DebugLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Читаем тело — оно должно быть доступно
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		_, _ = w.Write([]byte("response:" + string(body[:n])))
	}))

	body := `{"key":"value"}`
	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Body.String()
	if !strings.HasPrefix(resp, "response:") {
		t.Errorf("expected response with body, got %q", resp)
	}
}

func TestDebugLoggingMiddleware_SmallBody(t *testing.T) {
	handler := DebugLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDebugLoggingMiddleware_LargeBody(t *testing.T) {
	handler := DebugLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	// Body > 64KB — should be truncated
	body := strings.Repeat("x", 70*1024)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDebugLoggingMiddleware_NoBody(t *testing.T) {
	handler := DebugLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDebugLoggingMiddleware_CapturesResponseStatus(t *testing.T) {
	handler := DebugLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestBodyCaptureWriter(t *testing.T) {
	w := httptest.NewRecorder()
	bcw := &bodyCaptureWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
	}

	bcw.WriteHeader(http.StatusCreated)
	_, _ = bcw.Write([]byte("hello"))

	if bcw.status != http.StatusCreated {
		t.Errorf("expected status 201, got %d", bcw.status)
	}
	if w.Code != http.StatusCreated {
		t.Errorf("expected recorder status 201, got %d", w.Code)
	}
}
