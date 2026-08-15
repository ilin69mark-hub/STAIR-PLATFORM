package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/infrastructure/metrics"
)

// TestMetricsPublic: /metrics доступен без аутентификации и без rate-limit.
func TestMetricsPublic(t *testing.T) {
	r := NewRouter(stair.NewService(), nil, testAuth{}, DefaultConfig())

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"# TYPE http_requests_total counter",
		"# TYPE http_request_duration_seconds histogram",
		"# TYPE go_goroutines gauge",
		"process_uptime_seconds",
		"go_memstats_alloc_bytes",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics output missing %q", want)
		}
	}
}

// TestMetricsRecorded: запрос через роутер инкрементит http_requests_total.
func TestMetricsRecorded(t *testing.T) {
	r := NewRouter(stair.NewService(), nil, testAuth{}, DefaultConfig())

	// Уникальный path, чтобы не пересекаться с другими тестами.
	req := httptest.NewRequest(http.MethodGet, "/metrics-test-x", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown path, got %d", rec.Code)
	}

	// Read из реестра напрямую (до flush в /metrics).
	body := metricsBody(t)
	if !strings.Contains(body, `http_requests_total{method="GET",path="/metrics-test-x",status="404"`) {
		t.Fatalf("metrics missing recorded request:\n%s", body)
	}
}

// TestCanonicalPath: UUID-сегмент заменяется на {id}.
func TestCanonicalPath(t *testing.T) {
	cases := map[string]string{
		"/api/v1/projects": "/api/v1/projects",
		"/api/v1/projects/11111111-2222-3333-4444-555555555555/audit": "/api/v1/projects/{id}/audit",
		"/api/v1/admin/settings": "/api/v1/admin/settings",
	}
	for in, want := range cases {
		if got := canonicalPath(in); got != want {
			t.Fatalf("canonicalPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestMetricsRegistryWrite использует метрики напрямую (unit обвязка).
func TestMetricsRegistryWrite(t *testing.T) {
	reg := metrics.NewRegistry()
	cv := reg.Counter("test_counter", "h")
	cv.With().Inc()
	out := metricsBodyFrom(t, reg)
	if !strings.Contains(out, "test_counter 1") {
		t.Fatalf("counter missing:\n%s", out)
	}
}

// metricsBody рендерит содержимое глобального реестра.
func metricsBody(t *testing.T) string {
	t.Helper()
	return metricsBodyFrom(t, httpMetricsReg)
}

func metricsBodyFrom(t *testing.T, reg *metrics.Registry) string {
	t.Helper()
	var sb strings.Builder
	if err := reg.Write(&sb); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	return sb.String()
}
