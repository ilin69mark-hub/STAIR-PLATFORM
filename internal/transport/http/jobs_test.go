package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/jobs"
	"stairplatform/internal/application/stair"
)

// fakeJobsService — тестовая реализация JobsService (EDR-0035).
type fakeJobsService struct {
	submitted bool
	lastCfg   stair.Config
	job       *jobs.Job
	fail      bool
	missing   bool
}

func (f *fakeJobsService) SubmitCalculate(_ context.Context, tenantID, userID string, payload jobs.Payload) (*jobs.Job, error) {
	if f.fail {
		return nil, context.DeadlineExceeded
	}
	f.submitted = true
	f.lastCfg = payload.Config
	f.job = &jobs.Job{ID: "job-1", TenantID: tenantID, UserID: userID, Type: "calc.calculate",
		Status: jobs.StatusPending, Payload: payload}
	return f.job, nil
}

func (f *fakeJobsService) GetJob(_ context.Context, tenantID, id string) (*jobs.Job, error) {
	if f.missing {
		return nil, jobs.ErrNotFound
	}
	return &jobs.Job{ID: id, TenantID: tenantID, Type: "calc.calculate", Status: jobs.StatusSucceeded,
		Result: f.jobResult()}, nil
}

func (f *fakeJobsService) jobResult() *stair.Result {
	return &stair.Result{}
}

// testRouterWithJobs собирает роутер с сервисом фоновых заданий.
func testRouterWithJobs(svc JobsService) http.Handler {
	return NewRouter(stair.NewService(), newFakeProjectService(), testAuth{}, Config{Jobs: svc})
}

func TestCalculateAsyncAccepted(t *testing.T) {
	svc := &fakeJobsService{}
	body := `{"width_mm":900,"height_mm":2700,"flight":"straight","step_height_mm":180,
		"stringer_thickness_mm":50,"step_thickness_mm":40,"clearance_mm":80,"railing_height_mm":900}`
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate/async", body)
	rec := httptest.NewRecorder()
	testRouterWithJobs(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !svc.submitted {
		t.Fatal("SubmitCalculate not called")
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["job_id"] != "job-1" || resp["status"] != "pending" || resp["type"] != "calc.calculate" {
		t.Fatalf("unexpected response: %v", resp)
	}
	if svc.lastCfg.Width != 900 || svc.lastCfg.Height != 2700 {
		t.Fatalf("config not parsed: %+v", svc.lastCfg)
	}
}

// TestCalculateAsyncInvalidInput: заведомо невалидный вход (отрицательная
// ширина) отвергается синхронно (422) — в очередь не попадает (EDR-0035
// §3.4).
func TestCalculateAsyncInvalidInput(t *testing.T) {
	svc := &fakeJobsService{}
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate/async",
		`{"width_mm":-100,"height_mm":2700,"flight":"straight","step_height_mm":180}`)
	rec := httptest.NewRecorder()
	testRouterWithJobs(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	if svc.submitted {
		t.Fatal("invalid input must not be submitted to the queue")
	}
}

func TestCalculateAsyncInvalidJSON(t *testing.T) {
	svc := &fakeJobsService{}
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate/async", "{bad")
	rec := httptest.NewRecorder()
	testRouterWithJobs(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetJobOk(t *testing.T) {
	svc := &fakeJobsService{}
	req := authedRequest(http.MethodGet, "/api/v1/jobs/job-1", "")
	rec := httptest.NewRecorder()
	testRouterWithJobs(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp jobStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != "job-1" || resp.Status != "succeeded" {
		t.Fatalf("unexpected: %+v", resp)
	}
	// Результат отдаётся в формате синхронного расчёта (calculateResponse).
	if resp.Result == nil {
		t.Fatal("expected result for succeeded job")
	}
}

func TestGetJobNotFound(t *testing.T) {
	svc := &fakeJobsService{missing: true}
	req := authedRequest(http.MethodGet, "/api/v1/jobs/job-1", "")
	rec := httptest.NewRecorder()
	testRouterWithJobs(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestJobsRoutesRequireAuth: маршруты jobs за аутентификацией (SEC-0003).
func TestJobsRoutesRequireAuth(t *testing.T) {
	svc := &fakeJobsService{}
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/stairs:calculate/async"},
		{http.MethodPost, "/api/v1/stairs:calculate/async"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		testRouterWithJobs(svc).ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401, got %d", tc.method, tc.path, rec.Code)
		}
	}
}
