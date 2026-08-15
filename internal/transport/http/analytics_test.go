package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/analytics"
	"stairplatform/internal/application/stair"
)

// fakeAnalyticsService — тестовая реализация AnalyticsService.
type fakeAnalyticsService struct {
	rep     *analytics.UsageReport
	err     error
	gotFrom time.Time
	gotTo   time.Time
	gotGran analytics.Granularity
}

func (f *fakeAnalyticsService) Usage(_ context.Context, _ string, from, to time.Time, g analytics.Granularity) (*analytics.UsageReport, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.gotFrom = from
	f.gotTo = to
	f.gotGran = g
	return f.rep, nil
}

// testRouterWithAnalytics собирает роутер с fake-аналитикой и admin-auth.
func testRouterWithAnalytics(a AnalyticsService) http.Handler {
	cfg := DefaultConfig()
	cfg.Analytics = a
	return NewRouter(stair.NewService(), nil, adminAuth{}, cfg)
}

func TestUsageAnalytics(t *testing.T) {
	rep := &analytics.UsageReport{
		From:        time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:          time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		Granularity: analytics.GranularityDay,
		Totals:      analytics.UsageTotals{Users: 3, ActiveUsers: 2, Projects: 1, Calculations: 4, Logins: 5, Exports: 1, Payments: 1},
		Series: []analytics.UsagePoint{{
			Bucket: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			Logins: 5, ActiveUsers: 2, ProjectsCreated: 1, Calculations: 4, Exports: 1, Payments: 1,
		}},
	}
	svc := &fakeAnalyticsService{rep: rep}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/usage?from=2026-08-01&to=2026-08-02&granularity=day", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body usageReportDTO
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Totals.Users != 3 || body.Totals.Payments != 1 {
		t.Fatalf("unexpected totals: %+v", body.Totals)
	}
	if len(body.Series) != 1 || body.Series[0].Logins != 5 {
		t.Fatalf("unexpected series: %+v", body.Series)
	}
	if svc.gotGran != analytics.GranularityDay {
		t.Fatalf("expected day granularity, got %s", svc.gotGran)
	}
}

func TestUsageAnalyticsDefaults(t *testing.T) {
	rep := &analytics.UsageReport{
		From: time.Now().UTC().AddDate(0, 0, -30), To: time.Now().UTC(),
		Granularity: analytics.GranularityDay,
	}
	svc := &fakeAnalyticsService{rep: rep}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/usage", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.gotGran != analytics.GranularityDay {
		t.Fatalf("expected default day granularity, got %s", svc.gotGran)
	}
	if svc.gotFrom.IsZero() || svc.gotTo.IsZero() {
		t.Fatal("expected default from/to")
	}
}

func TestUsageAnalyticsRequiresAdmin(t *testing.T) {
	// testAuth возвращает роль user → 403 (нет права analytics.read).
	cfg := DefaultConfig()
	cfg.Analytics = &fakeAnalyticsService{}
	router := NewRouter(stair.NewService(), nil, testAuth{}, cfg)
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/usage", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsageAnalyticsInvalidGranularity(t *testing.T) {
	svc := &fakeAnalyticsService{}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/usage?granularity=hour", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "granularity") {
		t.Fatalf("expected granularity error, got %s", rec.Body.String())
	}
}

func TestUsageAnalyticsInvalidFrom(t *testing.T) {
	svc := &fakeAnalyticsService{}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/usage?from=not-a-date", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsageAnalyticsRangeError(t *testing.T) {
	svc := &fakeAnalyticsService{err: analytics.ErrInvalidRange}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/usage?from=2026-08-05&to=2026-08-01", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUsageAnalyticsServerError(t *testing.T) {
	svc := &fakeAnalyticsService{err: context.DeadlineExceeded}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/usage", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAnalyticsNotRegisteredWhenNil — маршруты не регистрируются без сервиса.
func TestAnalyticsNotRegisteredWhenNil(t *testing.T) {
	router := NewRouter(stair.NewService(), nil, adminAuth{}, DefaultConfig())
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/usage", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
