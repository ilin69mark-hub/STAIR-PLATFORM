package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	limiter := NewRateLimiter(3, time.Minute)

	// Первые 3 запроса должны быть разрешены
	for i := 0; i < 3; i++ {
		if !limiter.Allow("test-key") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// Четвертый запрос должен быть заблокирован
	if limiter.Allow("test-key") {
		t.Error("request 4 should be blocked")
	}
}

func TestRateLimiterWindow(t *testing.T) {
	limiter := NewRateLimiter(2, 100*time.Millisecond)

	// Первые 2 запроса разрешены
	if !limiter.Allow("test-key") {
		t.Error("request 1 should be allowed")
	}
	if !limiter.Allow("test-key") {
		t.Error("request 2 should be allowed")
	}

	// Третий запрос заблокирован
	if limiter.Allow("test-key") {
		t.Error("request 3 should be blocked")
	}

	// Ждем окончания window
	time.Sleep(150 * time.Millisecond)

	// Запрос снова разрешен
	if !limiter.Allow("test-key") {
		t.Error("request after window should be allowed")
	}
}

func TestRateLimiterDifferentKeys(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)

	// Запрос для ключа 1
	if !limiter.Allow("key-1") {
		t.Error("request for key-1 should be allowed")
	}

	// Запрос для ключа 2 должен быть разрешен (другой ключ)
	if !limiter.Allow("key-2") {
		t.Error("request for key-2 should be allowed")
	}

	// Второй запрос для ключа 1 должен быть заблокирован
	if limiter.Allow("key-1") {
		t.Error("second request for key-1 should be blocked")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	handler := limiter.RateLimit(ByIP)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// Первые 2 запроса должны пройти
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d should return 200, got %d", i+1, rr.Code)
		}
	}

	// Третий запрос должен вернуть 429
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("request 3 should return 429, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header")
	}
}

func TestByIP(t *testing.T) {
	tests := []struct {
		name     string
		remoteAddr string
		xff      string
		xri      string
		expected string
	}{
		{
			name:       "remote addr only",
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.195, 70.41.3.18",
			expected:   "203.0.113.195",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xri:        "203.0.113.195",
			expected:   "203.0.113.195",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			result := ByIP(req)
			if result != tt.expected {
				t.Errorf("ByIP() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestByEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stairs", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	result := ByEndpoint(req)
	expected := "/api/v1/stairs:192.168.1.1"
	if result != expected {
		t.Errorf("ByEndpoint() = %q, want %q", result, expected)
	}
}

func TestByUser_Authenticated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	ctx := req.Context()
	ctx = context.WithValue(ctx, ctxUserID, "user-123")
	req = req.WithContext(ctx)

	result := ByUser(req)
	if result != "user:user-123" {
		t.Errorf("ByUser() = %q, want %q", result, "user:user-123")
	}
}

func TestByUser_Unauthenticated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	result := ByUser(req)
	if result != "192.168.1.1" {
		t.Errorf("ByUser() = %q, want %q", result, "192.168.1.1")
	}
}

func TestRateLimiterStats(t *testing.T) {
	limiter := NewRateLimiter(3, time.Minute)
	limiter.Allow("key-1")
	limiter.Allow("key-1")
	limiter.Allow("key-1")
	limiter.Allow("key-1") // blocked
	limiter.Allow("key-2")

	total, blocked, active := limiter.Stats()
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if blocked != 1 {
		t.Errorf("expected blocked=1, got %d", blocked)
	}
	if active != 2 {
		t.Errorf("expected active=2, got %d", active)
	}
}

func TestRateLimiterHeaders(t *testing.T) {
	limiter := NewRateLimiter(5, time.Minute)
	handler := limiter.RateLimit(ByIP)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-RateLimit-Limit") != "5" {
		t.Errorf("expected X-RateLimit-Limit=5, got %q", rr.Header().Get("X-RateLimit-Limit"))
	}
	if rr.Header().Get("X-RateLimit-Remaining") != "4" {
		t.Errorf("expected X-RateLimit-Remaining=4, got %q", rr.Header().Get("X-RateLimit-Remaining"))
	}
	if rr.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("expected X-RateLimit-Reset header")
	}
}

func TestMultiRateLimiter(t *testing.T) {
	mrl := NewMultiRateLimiter(10, time.Minute) // default: 10 req/min
	mrl.AddEndpoint("/api/v1/auth/login", 3, time.Minute) // login: 3 req/min

	handler := mrl.RateLimit(ByIP)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Login endpoint — 3 requests
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	loginReq.RemoteAddr = "10.0.0.1:8080"

	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, loginReq)
		if rr.Code != http.StatusOK {
			t.Errorf("login request %d should return 200, got %d", i+1, rr.Code)
		}
	}

	// 4th login request should be blocked
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, loginReq)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("login request 4 should return 429, got %d", rr.Code)
	}

	// But /api/v1/projects still allows 10 requests
	projReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	projReq.RemoteAddr = "10.0.0.1:8080"
	for i := 0; i < 10; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, projReq)
		if rr.Code != http.StatusOK {
			t.Errorf("project request %d should return 200, got %d", i+1, rr.Code)
		}
	}
}

func TestMultiRateLimiterDefault(t *testing.T) {
	mrl := NewMultiRateLimiter(2, time.Minute)

	handler := mrl.RateLimit(ByIP)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	req.RemoteAddr = "10.0.0.1:8080"

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d should return 200, got %d", i+1, rr.Code)
		}
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("request 3 should return 429, got %d", rr.Code)
	}
}

func TestIntToString(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{100, "100"},
		{-1, "-1"},
		{86400, "86400"},
	}
	for _, tt := range tests {
		if got := intToString(tt.n); got != tt.want {
			t.Errorf("intToString(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestByUserEndpoint_Authenticated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	ctx := req.Context()
	ctx = context.WithValue(ctx, ctxUserID, "user-123")
	req = req.WithContext(ctx)

	result := ByUserEndpoint(req)
	expected := "user:user-123:/api/v1/projects"
	if result != expected {
		t.Errorf("ByUserEndpoint() = %q, want %q", result, expected)
	}
}

func TestByUserEndpoint_Unauthenticated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	result := ByUserEndpoint(req)
	expected := "192.168.1.1:/api/v1/projects"
	if result != expected {
		t.Errorf("ByUserEndpoint() = %q, want %q", result, expected)
	}
}
