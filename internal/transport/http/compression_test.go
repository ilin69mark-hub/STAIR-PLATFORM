package http

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressionMiddleware_GzipEnabled(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(strings.Repeat("Hello, World! ", 100))) // ~1.4KB
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("expected gzip Content-Encoding, got %q", resp.Header.Get("Content-Encoding"))
	}
	body, _ := io.ReadAll(resp.Body)
	gz, _ := gzip.NewReader(strings.NewReader(string(body)))
	decoded, _ := io.ReadAll(gz)
	if !strings.Contains(string(decoded), "Hello, World!") {
		t.Error("decoded body does not contain expected content")
	}
}

func TestCompressionMiddleware_NoGzip(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(strings.Repeat("Hello, World! ", 100)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Accept-Encoding header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("expected no gzip when Accept-Encoding not set")
	}
}

func TestCompressionMiddleware_SmallResponse(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("small")) // < 1KB
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("expected no gzip for small response")
	}
}

func TestCompressionMiddleware_BinaryContentType(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte(strings.Repeat("binary", 200)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("expected no gzip for binary content type")
	}
}

func TestCompressionMiddleware_HEAD(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(strings.Repeat("Hello!", 200)))
	}))

	req := httptest.NewRequest(http.MethodHead, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("expected no gzip for HEAD request")
	}
}

func TestCompressionMiddleware_JSON(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": "` + strings.Repeat("test", 300) + `"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("expected gzip for JSON, got %q", resp.Header.Get("Content-Encoding"))
	}
}

func TestCompressionMiddleware_AlreadyCompressed(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write([]byte(strings.Repeat("zip", 400)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Encoding") == "gzip" {
		t.Error("expected no gzip for already compressed type")
	}
}

func TestShouldNotCompress(t *testing.T) {
	tests := []struct {
		ct   string
		skip bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"audio/mpeg", true},
		{"video/mp4", true},
		{"application/zip", true},
		{"application/gzip", true},
		{"application/pdf", true},
		{"application/octet-stream", true},
		{"text/plain", false},
		{"application/json", false},
		{"text/html", false},
	}
	for _, tt := range tests {
		if got := shouldNotCompress(tt.ct); got != tt.skip {
			t.Errorf("shouldNotCompress(%q) = %v, want %v", tt.ct, got, tt.skip)
		}
	}
}
