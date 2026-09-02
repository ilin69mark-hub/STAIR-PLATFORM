package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	config := DefaultConfig()
	handler := SecurityHeaders(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Error("expected Content-Security-Policy header")
	}
	if rr.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected Strict-Transport-Security header")
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options 'nosniff', got %q", rr.Header().Get("X-Content-Type-Options"))
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected X-Frame-Options 'DENY', got %q", rr.Header().Get("X-Frame-Options"))
	}
	// X-XSS-Protection removed (deprecated)
	if rr.Header().Get("X-XSS-Protection") != "" {
		t.Errorf("expected no X-XSS-Protection header (deprecated), got %q", rr.Header().Get("X-XSS-Protection"))
	}
}

func TestSecurityHeadersDisabled(t *testing.T) {
	config := Config{
		EnableHSTS: false,
		CSPPolicy:  "",
	}
	handler := SecurityHeaders(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Strict-Transport-Security") != "" {
		t.Error("expected no Strict-Transport-Security header when HSTS disabled")
	}
	if rr.Header().Get("Content-Security-Policy") != "" {
		t.Error("expected no Content-Security-Policy header when CSP empty")
	}
}

func TestCORS(t *testing.T) {
	config := DefaultConfig()
	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Тест CORS с разрешенным origin
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://localhost:3000', got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if rr.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("expected Access-Control-Allow-Credentials 'true'")
	}
}

func TestCORSBlockedOrigin(t *testing.T) {
	config := DefaultConfig()
	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Тест CORS с неразрешенным origin
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header for blocked origin, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSPreflight(t *testing.T) {
	config := DefaultConfig()
	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for preflight, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}
	if rr.Header().Get("Access-Control-Max-Age") == "" {
		t.Error("expected Access-Control-Max-Age header")
	}
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		origin   string
		allowed  []string
		expected bool
	}{
		{"http://localhost:3000", []string{"http://localhost:3000"}, true},
		{"http://localhost:3000", []string{"http://localhost:5173"}, false},
		{"http://example.com", []string{"*"}, true},
		{"http://sub.example.com", []string{"*.example.com"}, true},
		{"http://evil.com", []string{"*.example.com"}, false},
		{"", []string{"http://localhost:3000"}, false},
	}

	for _, tt := range tests {
		result := isOriginAllowed(tt.origin, tt.allowed)
		if result != tt.expected {
			t.Errorf("isOriginAllowed(%q, %v) = %v, want %v", tt.origin, tt.allowed, result, tt.expected)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{123, "123"},
		{86400, "86400"},
		{31536000, "31536000"},
	}

	for _, tt := range tests {
		result := itoa(tt.input)
		if result != tt.expected {
			t.Errorf("itoa(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if len(config.AllowedOrigins) == 0 {
		t.Error("expected non-empty AllowedOrigins")
	}
	if len(config.AllowedMethods) == 0 {
		t.Error("expected non-empty AllowedMethods")
	}
	if len(config.AllowedHeaders) == 0 {
		t.Error("expected non-empty AllowedHeaders")
	}
	if config.MaxAge <= 0 {
		t.Error("expected positive MaxAge")
	}
	if config.HSTSMaxAge <= 0 {
		t.Error("expected positive HSTSMaxAge")
	}
}
