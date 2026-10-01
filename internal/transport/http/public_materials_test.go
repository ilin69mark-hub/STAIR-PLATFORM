package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	engmfg "stairplatform/internal/engine/manufacturing"
)

// TestPublicMaterials — витрина (этап 3) читает каталог материалов с API:
// коды, плотности, диапазоны толщины, ставка ₽/кг и ссылка на PBR-превью.
// Контракт — источник истины для маркетинга, поэтому проверяем полноту.
func TestPublicMaterials(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/public/materials", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got []materialDTO
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Сверка с реестром, а не с константой: публичный каталог обязан отдавать
	// ровно то, что умеет изготовить движок, и меняться вместе с ним.
	reg, err := engmfg.DefaultMaterialRegistry()
	if err != nil {
		t.Fatalf("material registry: %v", err)
	}
	if len(got) != len(reg.Materials()) {
		t.Fatalf("want %d материалов каталога, got %d", len(reg.Materials()), len(got))
	}
	byCode := map[string]materialDTO{}
	for _, m := range got {
		byCode[m.Code] = m
	}
	// Перечень берём из реестра MFG-0005, а не прописываем строкой.
	for _, mat := range reg.Materials() {
		code := string(mat.Code)
		m, ok := byCode[code]
		if !ok {
			t.Fatalf("материал %s отсутствует в публичном каталоге", code)
		}
		if m.PricePerKgRub <= 0 {
			t.Errorf("%s: ставка должна быть > 0, got %v", code, m.PricePerKgRub)
		}
		if m.SwatchURL != "/static-assets/pbr/"+code+"/color.jpg" {
			t.Errorf("%s: swatch_url %q", code, m.SwatchURL)
		}
		if m.NameRu == "" {
			t.Errorf("%s: name_ru обязателен для витрины", code)
		}
		if m.Density <= 0 || m.MinThicknessMM <= 0 || m.MaxThicknessMM <= m.MinThicknessMM {
			t.Errorf("%s: физические параметры некорректны: %+v", code, m)
		}
		if len(m.Finishes) == 0 {
			t.Errorf("%s: нет финишей", code)
		}
	}
	// Дерево ограничено толщиной снизу — витрина обязана это показывать.
	if byCode["WOOD-OAK"].MinThicknessMM < 20 {
		t.Errorf("дуб: MinThickness %v, ожидалось ≥ 20", byCode["WOOD-OAK"].MinThicknessMM)
	}
}
