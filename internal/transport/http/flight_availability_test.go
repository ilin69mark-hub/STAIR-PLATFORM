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

// TestPublicQuoteDropsSpiralVariations — советник может предлагать спиральные
// варианты (движок их строит), но публичный расчёт их не отдаёт: «Спасти
// расчёт» не должна уводить в 422 (S-152). Внутренняя ручка сохраняет всё.
func TestPublicQuoteDropsSpiralVariations(t *testing.T) {
	body := `{
		"width_mm": 1100, "height_mm": 2700, "flight": "straight", "material": "STEEL-S235",
		"step_height_mm": 168, "step_thickness_mm": 6, "stringer_thickness_mm": 50,
		"riser": true, "clearance_mm": 2000, "railing_height_mm": 900,
		"room_width_mm": 4500, "room_length_mm": 900
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/stairs:quote", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"flight":"spiral"`) {
		t.Errorf("публичный ответ не должен предлагать спиральные варианты: %s", rec.Body.String())
	}
}
