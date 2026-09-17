package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auditpkg "stairplatform/internal/application/audit"
	"stairplatform/internal/application/stair"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

type fakeStairService struct {
	calcErr error
	res     *stair.Result
	optErr  error
	optRes  *stair.OptimizeResult
}

func (f *fakeStairService) Calculate(_ context.Context, _ stair.Config, _ stair.Options) (*stair.Result, error) {
	return f.res, f.calcErr
}

func (f *fakeStairService) Optimize(_ context.Context, _ stair.Config, _ stair.Options, _ stair.OptimizeRequest) (*stair.OptimizeResult, error) {
	return f.optRes, f.optErr
}

func calculateRouter(f *fakeStairService, audit *fakeAuditRecord) http.Handler {
	var audits []AuditService
	if audit != nil {
		audits = append(audits, audit)
	}
	return NewRouter(f, nil, testAuth{}, DefaultConfig(), audits...)
}

func testCalculateRequest() *http.Request {
	return authedRequest(http.MethodPost, "/api/v1/stairs:calculate", referenceJSON)
}

func TestHandleCalculateCancelled(t *testing.T) {
	f := &fakeStairService{calcErr: context.Canceled}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, testCalculateRequest())
	if rec.Code != 499 {
		t.Fatalf("expected 499, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCalculateDeadlineExceeded(t *testing.T) {
	f := &fakeStairService{calcErr: context.DeadlineExceeded}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, testCalculateRequest())
	if rec.Code != 499 {
		t.Fatalf("expected 499, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCalculateUnknownTarget(t *testing.T) {
	f := &fakeStairService{calcErr: errors.New("unknown optimization target")}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, testCalculateRequest())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCalculateInputError(t *testing.T) {
	f := &fakeStairService{calcErr: &solver.InputError{Message: "bad geometry"}}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, testCalculateRequest())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "bad geometry") {
		t.Fatalf("expected message in body: %s", rec.Body.String())
	}
}

func TestHandleCalculateGenericError(t *testing.T) {
	f := &fakeStairService{calcErr: errors.New("kaboom")}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, testCalculateRequest())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCalculateBadJSON(t *testing.T) {
	f := &fakeStairService{res: &stair.Result{}}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/stairs:calculate", `{bad`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func invalidRatesRequest() *http.Request {
	return authedRequest(http.MethodPost, "/api/v1/stairs:calculate",
		`{"flight":"straight","width_mm":900,"height_mm":2700,"rates":{"machine_per_hour_rub":1e17}}`)
}

func TestHandleCalculateToOptionsError(t *testing.T) {
	f := &fakeStairService{}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, invalidRatesRequest())
	if rec.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200: %s", rec.Body.String())
	}
}

func TestHandleOptimizeInputError(t *testing.T) {
	f := &fakeStairService{optErr: &solver.InputError{Message: "no machines"}}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, authedRequest(http.MethodPost,
		"/api/v1/stairs:optimize", referenceJSON))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleOptimizeCancelled(t *testing.T) {
	f := &fakeStairService{optErr: context.Canceled}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, authedRequest(http.MethodPost,
		"/api/v1/stairs:optimize", referenceJSON))
	if rec.Code != 499 {
		t.Fatalf("expected 499, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleOptimizeToOptionsError(t *testing.T) {
	f := &fakeStairService{}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, authedRequest(http.MethodPost,
		"/api/v1/stairs:optimize", `{"rates":{"machine_per_hour_rub":1e17}}`))
	if rec.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200: %s", rec.Body.String())
	}
}

func TestHandleOptimizeGenericError(t *testing.T) {
	f := &fakeStairService{optErr: errors.New("kaboom")}
	rec := httptest.NewRecorder()
	calculateRouter(f, nil).ServeHTTP(rec, authedRequest(http.MethodPost,
		"/api/v1/stairs:optimize", referenceJSON))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRecordCalculationAuditBlocking(t *testing.T) {
	audit := &fakeAuditRecord{}
	f := &fakeStairService{res: &stair.Result{
		Validation: validation.Result{
			Blocking: true,
			Issues: []validation.Issue{{
				ID: "i-1", Code: "GEO_HEIGHT", Element: "h-1",
				Variations: []validation.Variation{{ID: "v-1", Fits: true, PassesNorms: true}},
			}},
		},
	}}
	rec := httptest.NewRecorder()
	calculateRouter(f, audit).ServeHTTP(rec, testCalculateRequest())
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if audit.recorded == nil {
		t.Fatal("expected audit record")
	}
	if audit.recorded.Result != auditpkg.ResultOK {
		t.Fatalf("unexpected result: %s", audit.recorded.Result)
	}
	if !strings.Contains(audit.recorded.Detail, `"blocking":true`) {
		t.Fatalf("expected blocking detail: %s", audit.recorded.Detail)
	}
	if !strings.Contains(audit.recorded.Detail, `"issues":[`) {
		t.Fatalf("expected issues detail: %s", audit.recorded.Detail)
	}
}

func TestRecordCalculationAuditNoValidation(t *testing.T) {
	audit := &fakeAuditRecord{}
	f := &fakeStairService{res: &stair.Result{
		Validation: validation.Result{Valid: true, Blocking: false},
	}}
	rec := httptest.NewRecorder()
	calculateRouter(f, audit).ServeHTTP(rec, testCalculateRequest())
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if audit.recorded == nil {
		t.Fatal("expected audit record")
	}
	if !strings.Contains(audit.recorded.Detail, `"blocking":false`) {
		t.Fatalf("expected blocking=false in detail: %s", audit.recorded.Detail)
	}
	if strings.Contains(audit.recorded.Detail, "issues") {
		t.Fatalf("non-blocking result must not list issues: %s", audit.recorded.Detail)
	}
}

func TestOptimizeSuccessNoAudit(t *testing.T) {
	audit := &fakeAuditRecord{}
	f := &fakeStairService{optRes: &stair.OptimizeResult{Valid: true, Evaluated: 3, Target: stair.TargetPrice}}
	rec := httptest.NewRecorder()
	calculateRouter(f, audit).ServeHTTP(rec, authedRequest(http.MethodPost,
		"/api/v1/stairs:optimize", referenceJSON))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if audit.recorded != nil {
		t.Fatal("optimize must not record an audit entry")
	}
}
