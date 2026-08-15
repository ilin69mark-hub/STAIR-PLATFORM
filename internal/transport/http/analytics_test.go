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

	projRep *analytics.ProjectReport
	mfgRep  *analytics.ManufacturingReport
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

func (f *fakeAnalyticsService) Projects(_ context.Context, _ string, from, to time.Time) (*analytics.ProjectReport, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.gotFrom = from
	f.gotTo = to
	return f.projRep, nil
}

func (f *fakeAnalyticsService) Manufacturing(_ context.Context, _ string, from, to time.Time, g analytics.Granularity) (*analytics.ManufacturingReport, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.gotFrom = from
	f.gotTo = to
	f.gotGran = g
	return f.mfgRep, nil
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

func TestProjectsAnalytics(t *testing.T) {
	valid := true
	svc := &fakeAnalyticsService{projRep: &analytics.ProjectReport{
		From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		Totals: analytics.ProjectTotals{
			Projects: 2, ProjectsCreated: 1, ByStatus: map[string]int{"draft": 1, "approved": 1},
			ProjectsWithCalculation: 1, ValidProjects: 1, Configurations: 2,
			Calculations: 1, Comments: 3,
		},
		Projects: []analytics.ProjectRow{
			{ID: "p-1", Name: "A", Status: "approved", OwnerEmail: "o@e.com",
				Configurations: 2, Calculations: 1, LatestCalculationValid: &valid, Comments: 3, Members: 2},
		},
	}}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/projects?from=2026-08-01&to=2026-08-02", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body projectReportDTO
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Totals.Projects != 2 || body.Totals.ByStatus["approved"] != 1 {
		t.Fatalf("unexpected totals: %+v", body.Totals)
	}
	if len(body.Projects) != 1 || body.Projects[0].Name != "A" {
		t.Fatalf("unexpected projects: %+v", body.Projects)
	}
	if body.Projects[0].LatestCalculationValid == nil || !*body.Projects[0].LatestCalculationValid {
		t.Fatalf("expected latest_calculation_valid=true, got %+v", body.Projects[0])
	}
	if svc.gotFrom.IsZero() || svc.gotTo.IsZero() {
		t.Fatal("expected parsed from/to")
	}
}

func TestProjectsAnalyticsRequiresAdmin(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Analytics = &fakeAnalyticsService{}
	router := NewRouter(stair.NewService(), nil, testAuth{}, cfg)
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/projects", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectsAnalyticsInvalidRange(t *testing.T) {
	svc := &fakeAnalyticsService{err: analytics.ErrInvalidRange}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/projects?from=2026-08-05&to=2026-08-01", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectsAnalyticsServerError(t *testing.T) {
	svc := &fakeAnalyticsService{err: context.DeadlineExceeded}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/projects", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestManufacturingAnalytics(t *testing.T) {
	svc := &fakeAnalyticsService{mfgRep: &analytics.ManufacturingReport{
		From:        time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:          time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		Granularity: analytics.GranularityDay,
		Totals: analytics.ManufacturingTotals{
			Calculations: 2, Parts: 10, BomLines: 3, CutItems: 2, Sheets: 1,
			PartArea: 1000, SheetArea: 2000, WasteArea: 1000, Utilization: 0.5,
			Materials: map[string]int{"STEEL-S235": 10},
		},
		Series: []analytics.ManufacturingPoint{
			{Bucket: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
				Calculations: 2, Parts: 10, Sheets: 1, Utilization: 0.5},
		},
	}}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/manufacturing?from=2026-08-01&to=2026-08-02&granularity=day", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body manufacturingReportDTO
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Totals.Parts != 10 || body.Totals.Materials["STEEL-S235"] != 10 {
		t.Fatalf("unexpected totals: %+v", body.Totals)
	}
	if len(body.Series) != 1 || body.Series[0].Sheets != 1 {
		t.Fatalf("unexpected series: %+v", body.Series)
	}
	if svc.gotGran != analytics.GranularityDay {
		t.Fatalf("expected day granularity, got %s", svc.gotGran)
	}
}

func TestManufacturingAnalyticsRequiresAdmin(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Analytics = &fakeAnalyticsService{}
	router := NewRouter(stair.NewService(), nil, testAuth{}, cfg)
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/manufacturing", "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestManufacturingAnalyticsInvalidGranularity(t *testing.T) {
	svc := &fakeAnalyticsService{}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/manufacturing?granularity=hour", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestManufacturingAnalyticsServerError(t *testing.T) {
	svc := &fakeAnalyticsService{err: context.DeadlineExceeded}
	req := authedRequest(http.MethodGet, "/api/v1/admin/analytics/manufacturing", "")
	rec := httptest.NewRecorder()
	testRouterWithAnalytics(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAnalyticsNotRegisteredWhenNil — маршруты не регистрируются без сервиса.
func TestAnalyticsNotRegisteredWhenNil(t *testing.T) {
	router := NewRouter(stair.NewService(), nil, adminAuth{}, DefaultConfig())
	for _, path := range []string{
		"/api/v1/admin/analytics/usage",
		"/api/v1/admin/analytics/projects",
		"/api/v1/admin/analytics/manufacturing",
	} {
		req := authedRequest(http.MethodGet, path, "")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: expected 404, got %d", path, rec.Code)
		}
	}
}
