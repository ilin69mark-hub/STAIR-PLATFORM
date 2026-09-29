package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stairplatform/internal/application/project"
	kerngeo "stairplatform/internal/geometry"
)

// TestDOM003_ConfigurationDTOCarriesNineParams — регрессия DOM-003.
//
// configurationDTO перечислял 19 полей, а конфигурация содержит 28. Девять
// параметров не попадали в ответ, поэтому загруженная ревизия не содержала
// сведений о типе поворота, сторонах перил, направлениях и материале, и
// повторная отправка формы их теряла.
func TestDOM003_ConfigurationDTOCarriesNineParams(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.configs = []*project.StairConfiguration{{
		ID: "cfg-2", ProjectID: "p-1", Revision: 2,
		WidthMM: 1000, HeightMM: 2700, Flight: "u_shape",
		TurnKind:        "winder",
		WinderCount:     5,
		Railing:         "both",
		RailingLower:    "left",
		RailingLanding:  "none",
		RailingUpper:    "right",
		Direction:       "right",
		SpiralDirection: "cw",
		MaterialCode:    "STEEL-S235",
		CreatedAt:       time.Now(),
	}}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/configurations/cfg-2", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Проверяем сырой JSON: omitempty скрывает пустые значения, поэтому
	// сверяем именно наличие ключей.
	var raw map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for k, want := range map[string]any{
		"turn_kind":        "winder",
		"winder_count":     float64(5),
		"railing":          "both",
		"railing_lower":    "left",
		"railing_landing":  "none",
		"railing_upper":    "right",
		"direction":        "right",
		"spiral_direction": "cw",
		"material":         "STEEL-S235",
	} {
		got, ok := raw[k]
		if !ok {
			t.Errorf("DOM-003: response must contain %q", k)
			continue
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", k, got, want)
		}
	}
}

// TestDOM003_ConfigurationDTOOmitsUnsetOptionalParams — «не задано» не должно
// засорять ответ (omitempty), но при этом обязательные поля остаются.
func TestDOM003_ConfigurationDTOOmitsUnsetOptionalParams(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.configs = []*project.StairConfiguration{{
		ID: "cfg-1", ProjectID: "p-1", Revision: 1,
		WidthMM: 900, HeightMM: 2700, Flight: "straight", CreatedAt: time.Now(),
	}}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/configurations/cfg-1", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var raw map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, k := range []string{"turn_kind", "winder_count", "spiral_direction", "material"} {
		if _, ok := raw[k]; ok {
			t.Errorf("unset %q must be omitted (omitempty), got %v", k, raw[k])
		}
	}
	if raw["width_mm"].(float64) != 900 {
		t.Errorf("width_mm must stay: %v", raw["width_mm"])
	}
}

// TestDOM001_CalculateAcceptsTurnKindAndWinderCount — регрессия DOM-001.
//
// calculateRequest принимал turn_kind/winder_count, но они НЕ доходили до
// stair.Config, из-за чего поворотные ступени (winder) были недостижимы извне:
// пользователь отправлял turn_kind=winder, а расчёт шёл как с площадкой.
// Проверяем, что значения доехали до расчёта — это видно по u_shape-эхо.
func TestDOM001_CalculateAcceptsTurnKindAndWinderCount(t *testing.T) {
	// Габариты подобраны так, чтобы расчёт был валиден по геометрии (угол
	// 30–45°, проступь 260–320, просвет ≥ 2000): иначе проверка упиралась бы
	// в геометрические нормы, а не в проброс turn_kind/winder_count.
	body := `{
		"width_mm": 900, "height_mm": 2700, "flight": "u_shape",
		"landing_width_mm": 1000, "landing_depth_mm": 1000, "lower_step_count": 6,
		"step_height_mm": 180, "stringer_thickness_mm": 50, "step_thickness_mm": 40,
		"clearance_mm": 2000, "railing_height_mm": 900, "riser": true,
		"turn_kind": "winder", "winder_count": 3,
		"room_width_mm": 0, "room_length_mm": 0, "approach_space_mm": 1000
	}`
	req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate", body)
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for winder config, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		UShape *struct {
			TurnKind    string `json:"turn_kind"`
			WinderCount int    `json:"winder_count"`
		} `json:"ushape"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.UShape == nil {
		t.Fatalf("DOM-001: expected u_shape result, got: %s", rec.Body.String())
	}
	if resp.UShape.TurnKind != "winder" {
		t.Errorf("DOM-001: turn_kind must reach the calculation, got %q", resp.UShape.TurnKind)
	}
	if resp.UShape.WinderCount != 3 {
		t.Errorf("DOM-001: winder_count must reach the calculation, got %d", resp.UShape.WinderCount)
	}
}

// TestDOM001_InvalidTurnKindRejected — мусор в turn_kind не должен молча
// превращаться в дефолт: расчёт возвращает blocking-ошибку валидации.
func TestDOM001_InvalidTurnKindRejected(t *testing.T) {
	for name, body := range map[string]string{
		"unknown turn_kind": `{"width_mm":1000,"height_mm":2700,"flight":"l_shape",
			"step_height_mm":150,"stringer_thickness_mm":50,"step_thickness_mm":6,
			"riser":true,"clearance_mm":80,"railing_height_mm":900,
			"landing_width_mm":1000,"landing_depth_mm":1000,"turn_kind":"spiral"}`,
		"winder without count": `{"width_mm":900,"height_mm":2700,"flight":"u_shape",
			"landing_width_mm":1000,"landing_depth_mm":1000,"lower_step_count":6,
			"step_height_mm":180,"stringer_thickness_mm":50,"step_thickness_mm":40,
			"clearance_mm":2000,"railing_height_mm":900,"riser":true,
			"turn_kind":"winder","winder_count":0}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := authedRequest(http.MethodPost, "/api/v1/stairs:calculate", body)
			rec := httptest.NewRecorder()
			testRouter().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 with validation report, got %d: %s", rec.Code, rec.Body.String())
			}
			var resp struct {
				Validation struct {
					Valid    bool `json:"valid"`
					Blocking bool `json:"blocking"`
					Issues   []struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"issues"`
				} `json:"validation"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp.Validation.Valid || !resp.Validation.Blocking {
				t.Fatalf("DOM-001: %s must be rejected as blocking, got valid=%v blocking=%v",
					name, resp.Validation.Valid, resp.Validation.Blocking)
			}
			if len(resp.Validation.Issues) == 0 {
				t.Fatal("blocking result must explain the reason")
			}
		})
	}
}

// TestDOM003_ExportCADIncludesRailings — регрессия DOM-003.
//
// Перила строятся отдельным телом (RailingMesh) и в основной меш не входят.
// Экспорт обязан сливать их с лестницей, иначе в DXF/STL/SVG перил нет.
func TestDOM003_ExportCADIncludesRailings(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.cadMesh = &kerngeo.Mesh{
		Vertices:  []kerngeo.Point3{{X: 0, Y: 0, Z: 0}, {X: 100, Y: 0, Z: 0}, {X: 0, Y: 100, Z: 0}},
		Triangles: [][3]int{{0, 1, 2}},
	}
	svc.cadRailings = &kerngeo.Mesh{
		Vertices:  []kerngeo.Point3{{X: 0, Y: 0, Z: 900}, {X: 100, Y: 0, Z: 900}, {X: 0, Y: 100, Z: 900}},
		Triangles: [][3]int{{0, 1, 2}},
	}

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export/cad?format=stl", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// ASCII STL: одна грань = 7 строк (facet normal, 3 vertex, endfacet, facet).
	// Две сетки → две грани. Если бы перила не слились, была бы одна.
	if got := strings.Count(rec.Body.String(), "facet normal"); got != 2 {
		t.Fatalf("DOM-003: export must contain stair + railing facets, got %d facet(s)", got)
	}
}
