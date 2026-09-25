package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPublicSpiralTemporarilyDisabled — винтовой марш выведен из публичных
// расчётов (S-152): и :quote, и :validate отвечают 422 с понятным текстом.
// Код движка и внутренний расчёт не тронуты — это фича-флаг на границе API.
func TestPublicSpiralTemporarilyDisabled(t *testing.T) {
	body := `{
		"width_mm": 900, "height_mm": 2800, "flight": "spiral", "material": "ALUM-5083",
		"step_height_mm": 175, "step_thickness_mm": 6, "stringer_thickness_mm": 50,
		"clearance_mm": 2000, "railing_height_mm": 900, "outer_radius_mm": 1200
	}`
	for _, path := range []string{"/api/v1/public/stairs:quote", "/api/v1/public/stairs:validate"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		testRouter().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: want 422, got %d: %s", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "flight_temporarily_disabled") {
			t.Errorf("%s: нет кода отключения в ответе: %s", path, rec.Body.String())
		}
	}
}

// TestInternalCalculateStillAcceptsSpiral — внутренний расчёт (черновики
// проектов, тесты движка) продолжает считать спираль: флаг только публичный.
func TestInternalCalculateStillAcceptsSpiral(t *testing.T) {
	body := `{
		"width_mm": 900, "height_mm": 2800, "flight": "spiral", "material": "ALUM-5083",
		"step_height_mm": 175, "step_thickness_mm": 6, "stringer_thickness_mm": 50,
		"clearance_mm": 2000, "railing_height_mm": 900, "outer_radius_mm": 1200
	}`
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, authedRequest(http.MethodPost, "/api/v1/stairs:calculate", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("внутренний расчёт спирали должен работать, got %d: %s", rec.Code, rec.Body.String())
	}
}
