package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// TestPublicQuoteMeshHasPartRanges (этап 1, шаг 2) — публичный расчёт
// отдаёт роли деталей (PartRanges), чтобы 3D-вьювер мог назначать разные
// материалы ступеням/косоурам/площадке. Проверяем также инвариант покрытия:
// сумма длин диапазонов равна числу треугольников.
func TestPublicQuoteMeshHasPartRanges(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, publicQuoteRequest(
		`{"width_mm":900,"height_mm":2700,"step_height_mm":180,"flight":"straight",`+
			`"stringer_thickness_mm":40,"step_thickness_mm":40,"comfort_step_mm":630,"railing_height_mm":900}`))
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var raw struct {
		Mesh struct {
			Triangles  [][]int `json:"Triangles"`
			PartRanges []struct {
				Solid int    `json:"Solid"`
				Role  string `json:"Role"`
				Start int    `json:"Start"`
				End   int    `json:"End"`
			} `json:"PartRanges"`
		} `json:"mesh"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Mesh.Triangles) == 0 {
		t.Fatal("mesh has no triangles")
	}
	if len(raw.Mesh.PartRanges) == 0 {
		t.Fatal("mesh has no PartRanges — 3D не сможет назначить материалы по ролям")
	}
	covered := 0
	roles := map[string]bool{}
	for _, pr := range raw.Mesh.PartRanges {
		if pr.Start < 0 || pr.End > len(raw.Mesh.Triangles) || pr.Start >= pr.End {
			t.Fatalf("invalid range %+v (triangles=%d)", pr, len(raw.Mesh.Triangles))
		}
		covered += pr.End - pr.Start
		roles[pr.Role] = true
	}
	if covered != len(raw.Mesh.Triangles) {
		t.Fatalf("PartRanges cover %d triangles, want %d", covered, len(raw.Mesh.Triangles))
	}
	if !roles["tread"] || !roles["stringer"] {
		t.Fatalf("expected tread+stringer roles, got %v", roles)
	}
}
