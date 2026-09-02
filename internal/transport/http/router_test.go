package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stairplatform/internal/application/stair"
)

func testRouter() http.Handler {
	return NewRouter(stair.NewService(), nil, testAuth{}, DefaultConfig())
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
	"riser": true,
	"clearance_mm": 2500,
	"railing_height_mm": 1000
}`

func TestCalculateRailingAndDirectionEcho(t *testing.T) {
	body := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "l_shape",
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"riser": true,
		"clearance_mm": 2500,
		"railing_height_mm": 1000,
		"landing_width_mm": 1000,
		"lower_step_count": 7,
		"railing_lower": "left",
		"railing_landing": "both",
		"railing_upper": "right",
		"direction": "right"
	}`
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate", body)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp calculateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if resp.Validation.Blocking {
		t.Fatalf("expected valid, got %+v", resp.Validation)
	}
	if resp.LShape == nil {
		t.Fatal("expected lshape result")
	}
	if resp.LShape.RailingLower != "left" || resp.LShape.RailingLanding != "both" ||
		resp.LShape.RailingUpper != "right" || resp.LShape.Direction != "right" {
		t.Fatalf("railing/direction echo mismatch: %+v", resp.LShape)
	}
}

func TestCalculateSpiralAutoRailingEcho(t *testing.T) {
	// Спираль: перила выводятся автоматически из направления закрутки
	// (CONF-SPIRAL-RAILING): по часовой — справа, против — слева.
	for _, tc := range []struct {
		dir     string
		railing string
	}{
		{"cw", "right"},
		{"ccw", "left"},
	} {
		body := `{
			"width_mm": 500,
			"height_mm": 2700,
			"flight": "spiral",
			"step_height_mm": 180,
			"stringer_thickness_mm": 50,
			"step_thickness_mm": 40,
			"riser": true,
			"clearance_mm": 2500,
			"railing_height_mm": 1000,
			"outer_radius_mm": 800,
			"spiral_direction": "` + tc.dir + `"
		}`
		req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate", body)
		rec := httptest.NewRecorder()
		testRouter().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected status 200, got %d: %s", tc.dir, rec.Code, rec.Body.String())
		}
		var resp calculateResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("%s: invalid response: %v", tc.dir, err)
		}
		if resp.Spiral == nil || resp.Validation.Blocking {
			t.Fatalf("%s: expected valid spiral, got %+v", tc.dir, resp.Validation)
		}
		if resp.Spiral.Railing != tc.railing {
			t.Fatalf("%s: auto railing = %q, want %q", tc.dir, resp.Spiral.Railing, tc.railing)
		}
		if resp.Spiral.SpiralDirection != tc.dir {
			t.Fatalf("%s: spiral_direction echo = %q, want %q", tc.dir, resp.Spiral.SpiralDirection, tc.dir)
		}
	}
}

func TestCalculateInvalidRailingRejected(t *testing.T) {
	body := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000,
		"railing": "diagonal"
	}`
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate", body)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected advisory 200, got %d", rec.Code)
	}
	var resp calculateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Validation.Blocking {
		t.Fatal("invalid railing must block validation")
	}
}

func TestCalculateReference(t *testing.T) {
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate",
		referenceJSON)
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
	if resp.Flight.StepThicknessMm != 40 || resp.Flight.RailingHeightMm != 1000 || !resp.Flight.Riser || resp.Flight.StringerThicknessMm != 50 {
		t.Fatalf("flight echo mismatch: %+v", resp.Flight)
	}
	if resp.Pricing.FinalPriceRub != 3271385.47 {
		t.Fatalf("final price = %v, want 3271385.47", resp.Pricing.FinalPriceRub)
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
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate",
		"{not json")
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
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate",
		body)
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
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate",
		body)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	// Пользовательская проблема возвращается как блокирующий результат
	// (200) с русской подсказкой, а не как техническая ошибка 422.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 with advisory result, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("invalid body: %v", err)
	}
	val, ok := payload["validation"].(map[string]any)
	if !ok {
		t.Fatalf("validation section expected: %v", payload)
	}
	if blocking, _ := val["blocking"].(bool); !blocking {
		t.Fatalf("expected blocking advisory, got %v", val)
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
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate",
		body)
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
	if resp.Pricing.MaterialRub <= 1820332.77 {
		t.Fatalf("material with doubled rate must exceed default, got %v", resp.Pricing.MaterialRub)
	}
	if resp.Pricing.FinalPriceRub <= 3271385.47 {
		t.Fatalf("final price with doubled steel must exceed default, got %v", resp.Pricing.FinalPriceRub)
	}
}

func TestOptimizeReference(t *testing.T) {
	// Полный поиск по H=2700: n ∈ [14, 18]; лучший — валидная конфигурация
	// с минимальной ценой.
	req := authedRequest(http.MethodPost, "/api/v1/stairs:optimize", referenceJSON)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp optimizeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Valid {
		t.Fatal("expected valid optimization")
	}
	if resp.Best == nil {
		t.Fatal("expected best candidate")
	}
	if resp.Best.StepCount < 14 || resp.Best.StepCount > 18 {
		t.Fatalf("best step count %d outside [14, 18]", resp.Best.StepCount)
	}
	if resp.Best.StepHeightMM < 150 || resp.Best.StepHeightMM > 200 {
		t.Fatalf("best step height %v outside [150, 200]", resp.Best.StepHeightMM)
	}
	if resp.Best.Result.Pricing.FinalPriceRub <= 0 {
		t.Fatal("best result must include pricing")
	}
	if resp.Best.Result.Validation.Blocking {
		t.Fatal("best result must not be blocking")
	}
	if resp.Target != "price" {
		t.Fatalf("target = %q, want price", resp.Target)
	}
}

func TestOptimizeTargetCost(t *testing.T) {
	body := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000,
		"target": "cost"
	}`
	req := authedRequest(http.MethodPost, "/api/v1/stairs:optimize", body)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp optimizeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if !resp.Valid || resp.Target != "cost" {
		t.Fatalf("expected valid cost optimization, got valid=%v target=%q", resp.Valid, resp.Target)
	}
	if resp.Best.Result.Pricing.ProductionCostRub <= 0 {
		t.Fatal("best result must include production cost")
	}
}

func TestOptimizeNoValidCandidate(t *testing.T) {
	// n=18 → угол ≈ 26.6° < 30° → все кандидаты блокируются → valid:false.
	body := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000,
		"step_count_min": 18,
		"step_count_max": 18
	}`
	req := authedRequest(http.MethodPost, "/api/v1/stairs:optimize", body)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp optimizeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if resp.Valid {
		t.Fatal("expected valid=false for empty feasible set")
	}
	if resp.Best != nil {
		t.Fatal("expected no best for empty feasible set")
	}
}

func TestOptimizeUnknownTarget(t *testing.T) {
	body := `{
		"width_mm": 900,
		"height_mm": 2700,
		"flight": "straight",
		"step_height_mm": 180,
		"stringer_thickness_mm": 50,
		"step_thickness_mm": 40,
		"clearance_mm": 2500,
		"railing_height_mm": 1000,
		"target": "weight"
	}`
	req := authedRequest(http.MethodPost, "/api/v1/stairs:optimize", body)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOptimizeInvalidJSON(t *testing.T) {
	req := authedRequest(http.MethodPost, "/api/v1/stairs:optimize", "{not json")
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
