package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	// Предел СТУПЕНИ публикуется отдельно от общего минимума детали: у дуба
	// 40 мм при общем 20 мм. Калькулятор берёт отсюда поле «толщина ступени»,
	// и без этого он показывал бы пользователю 20 мм — а сервер отвечал бы
	// 422 MFG-MATERIAL.
	for code, mat := range byCode {
		if !strings.HasPrefix(code, "WOOD-") {
			continue
		}
		if mat.MinTreadThicknessMM != 40 {
			t.Errorf("%s: min_tread_thickness_mm %v, ожидалось 40 (решение владельца 2026-10-05)",
				code, mat.MinTreadThicknessMM)
		}
		if mat.MinTreadThicknessMM < mat.MinThicknessMM {
			t.Errorf("%s: предел ступени %v ниже общего минимума детали %v",
				code, mat.MinTreadThicknessMM, mat.MinThicknessMM)
		}
	}
	// У стали отдельного предела ступени нет: действует общий 3 мм.
	if byCode["STEEL-C245"].MinTreadThicknessMM != byCode["STEEL-C245"].MinThicknessMM {
		t.Errorf("сталь: min_tread_thickness_mm %v != min_thickness_mm %v",
			byCode["STEEL-C245"].MinTreadThicknessMM, byCode["STEEL-C245"].MinThicknessMM)
	}
	// Верхняя граница стали 12 мм, а не 8: норматив на листовой косоур
	// (решение владельца 2026-10-05) описывает строки 8 / 10 / 12 мм, и
	// публичный каталог обязан показывать предел, который сервер пропускает.
	// Иначе калькулятор не даст ввести предписанную толщину.
	if got := byCode["STEEL-C245"].MaxThicknessMM; got != 12 {
		t.Errorf("сталь: max_thickness_mm %v, ожидалось 12 (строки норматива 8/10/12)", got)
	}
}
