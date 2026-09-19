package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodySizeLimitMiddleware_WithinLimit(t *testing.T) {
	handler := BodySizeLimitMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	body := strings.Repeat("x", 512)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestBodySizeLimitMiddleware_OverLimit_ContentLength(t *testing.T) {
	handler := BodySizeLimitMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	body := strings.Repeat("x", 2048)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", w.Code)
	}
}

func TestBodySizeLimitMiddleware_ExactLimit(t *testing.T) {
	handler := BodySizeLimitMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	body := strings.Repeat("x", 1024)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for exact limit, got %d", w.Code)
	}
}

func TestBodySizeLimitMiddleware_Default(t *testing.T) {
	handler := BodySizeLimitDefault()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	// 1MB — should be fine
	body := strings.Repeat("x", 1<<20)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for 1MB, got %d", w.Code)
	}
}

func TestBodySizeLimitMiddleware_Upload(t *testing.T) {
	handler := BodySizeLimitUpload()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	// 10MB — should be fine for upload
	body := strings.Repeat("x", 10<<20)
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for 10MB upload, got %d", w.Code)
	}
}

func TestBodySizeLimitMiddleware_ZeroContentLength(t *testing.T) {
	handler := BodySizeLimitMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for GET with no body, got %d", w.Code)
	}
}

func TestItoa64(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{100, "100"},
		{-1, "-1"},
		{1024, "1024"},
		{1 << 20, "1048576"},
	}
	for _, tt := range tests {
		if got := itoa64(tt.n); got != tt.want {
			t.Errorf("itoa64(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
