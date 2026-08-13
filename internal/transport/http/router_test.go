package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/stair"
)

func testRouter() http.Handler {
	return NewRouter(stair.NewService())
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("invalid response body: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %q", body.Status)
	}
}

func TestNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

// referenceJSON — эталонный запрос конвейера n=15 (см. application layer).
const referenceJSON = `{
	"width_mm": 900,
	"height_mm": 2700,
	"flight": "straight",
	"step_height_mm": 180,
	"stringer_thickness_mm": 50,
	"step_thickness_mm": 40,
	"clearance_mm": 2500,
	"railing_height_mm": 1000
}`

func TestCalculateReference(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stairs:calculate",
		strings.NewReader(referenceJSON))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp calculateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Valid || resp.Validation.Blocking {
		t.Fatalf("expected valid, got %+v", resp.Validation)
	}
	if resp.Flight.StepCount != 15 {
		t.Fatalf("step count = %d, want 15", resp.Flight.StepCount)
	}
	if resp.Pricing.FinalPriceRub != 2868282.74 {
		t.Fatalf("final price = %v, want 2868282.74", resp.Pricing.FinalPriceRub)
	}
	if len(resp.Manufacturing.Parts) == 0 || len(resp.Manufacturing.BOM) == 0 ||
		len(resp.Manufacturing.CutList) == 0 || len(resp.Manufacturing.Nesting.Sheets) == 0 {
		t.Fatal("full manufacturing package must be present")
	}
	if len(resp.Pricing.Lines) == 0 {
		t.Fatal("pricing lines must be present")
	}
}

func TestCalculateInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stairs:calculate",
		strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCalculateBlocking(t *testing.T) {
	body := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 10,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stairs:calculate",
		strings.NewReader(body))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 with report, got %d", rec.Code)
	}
	var resp calculateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Blocking {
		t.Fatalf("expected blocking, got %+v", resp.Validation)
	}
	if len(resp.Pricing.Lines) != 0 || len(resp.Manufacturing.Parts) != 0 {
		t.Fatal("blocking result must have empty manufacturing/pricing")
	}
}

func TestCalculateInvalidInput(t *testing.T) {
	body := `{
		"width_mm": 900,
		"height_mm": 0,
		"flight": "straight",
		"step_height_mm": 180
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stairs:calculate",
		strings.NewReader(body))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("invalid error body: %v", err)
	}
	if _, ok := payload["error"]; !ok {
		t.Fatal("error body must contain error object")
	}
}

func TestCalculateCustomRates(t *testing.T) {
	body := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000,
		"rates": {
			"material_per_kg_rub": {"STEEL-S235": 200},
			"machine_per_hour_rub": 8000,
			"labor_per_hour_rub": 3000,
			"overhead_percent": 20,
			"margin_percent": 30,
			"discount_percent": 5,
			"tax_percent": 20
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stairs:calculate",
		strings.NewReader(body))
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp calculateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	// Удвоенная цена стали → материал и финальная цена выше дефолта.
	if resp.Pricing.MaterialRub <= 1593597.87 {
		t.Fatalf("material with doubled rate must exceed default, got %v", resp.Pricing.MaterialRub)
	}
	if resp.Pricing.FinalPriceRub <= 2868282.74 {
		t.Fatalf("final price with doubled steel must exceed default, got %v", resp.Pricing.FinalPriceRub)
	}
}
