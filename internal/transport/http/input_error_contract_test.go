package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/stair"
)

// TestInvalidInputNeverReturns500 (API-001, forensic 2026-09-24) —
// пользовательская ошибка входа не должна выглядеть как внутренний сбой.
// Контракт фронта (projects.ts: validateStair) — 200 с validation.valid=false.
func TestInvalidInputNeverReturns500(t *testing.T) {
	router := NewRouter(stair.NewService(), nil, newFakeAuth(), DefaultConfig())
	cases := []struct {
		name string
		body string
	}{
		{"approach_space_out_of_range", `{"width_mm":900,"height_mm":2700,"step_height_mm":180,"flight":"straight","approach_space_mm":500}`},
		{"landing_width_lt_flight", `{"width_mm":900,"height_mm":2700,"step_height_mm":180,"flight":"l_shape","landing_width_mm":600,"lower_step_count":8}`},
		{"negative_railing", `{"width_mm":900,"height_mm":2700,"step_height_mm":180,"flight":"straight","railing_height_mm":-50}`},
		{"lower_step_count_zero", `{"width_mm":900,"height_mm":2700,"step_height_mm":180,"flight":"u_shape","landing_width_mm":900,"lower_step_count":0}`},
		{"zero_thickness_no_geometry", `{"width_mm":900,"height_mm":2700,"step_height_mm":180,"flight":"straight"}`},
		{"unknown_railing_side", `{"width_mm":900,"height_mm":2700,"step_height_mm":180,"flight":"straight","railing":"diagonal"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := assistantAuthedRequest("/api/v1/stairs:calculate", c.body)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusInternalServerError {
				t.Fatalf("input error leaked as 500: %s", rec.Body.String())
			}
			// Контракт фронта: 200 + блокирующий результат валидации.
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"valid":false`) {
				t.Fatalf("want 200 + valid:false, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
