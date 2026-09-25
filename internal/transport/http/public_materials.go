package http

import (
	"net/http"

	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	engmfg "stairplatform/internal/engine/manufacturing"
	engprc "stairplatform/internal/engine/pricing"
)

// materialDTO — публичное описание материала каталога для витрины (этап 3).
// Источник истины — MFG-0005 (DefaultMaterialRegistry) и PRC-0006
// (DefaultRates): витрина не дублирует каталог кодом, а читает его с API,
// поэтому новый материал появляется на сайте вместе с бэкендом.
type materialDTO struct {
	Code string `json:"code"`
	Name string `json:"name"`
	// NameRu — витринное имя на русском (MFG-0005, NameRu). Пустое у нового
	// материала — клиент покажет Name, поэтому поле не ломает контракт.
	NameRu         string  `json:"name_ru,omitempty"`
	Category       string  `json:"category"`
	Density        float64 `json:"density_kg_m3"`
	MinThicknessMM float64 `json:"min_thickness_mm"`
	MaxThicknessMM float64 `json:"max_thickness_mm"`
	MaxWidthMM     float64 `json:"max_width_mm"`
	MaxHeightMM    float64 `json:"max_height_mm"`
	// PricePerKgRub — ставка каталога (₽/кг); цены-заглушки, помечены в UI.
	PricePerKgRub float64 `json:"price_per_kg_rub"`
	// SwatchURL — превью PBR-текстуры (public read-only mount, этап 1).
	SwatchURL string `json:"swatch_url"`
	// Finishes — доступные финиши (вид, не код материала и не цена).
	Finishes []string `json:"finishes,omitempty"`
}

// materialFinishes — финиши по коду материала. Держим рядом с каталогом на
// бэкенде, чтобы витрина и калькулятор показывали один и тот же список.
var materialFinishes = map[dommfg.MaterialCode][]string{
	"STEEL-S235":   {"raw", "black", "white"},
	"STEEL-CORTEN": {"raw"},
	"ALUM-5083":    {"natural", "black"},
	"WOOD-OAK":     {"oil", "matte", "toned"},
	"WOOD-WALNUT":  {"oil", "matte"},
	"WOOD-ASH":     {"oil", "matte"},
	"WOOD-SOFT":    {"oil", "matte"},
}

// handlePublicMaterials — GET /api/v1/public/materials.
// Публичный каталог материалов: код, плотность, диапазоны толщины и габаритов,
// ставка ₽/кг и ссылка на PBR-превью. Цена — прайс магазина поверх встроенных
// ставок движка (волна 0), поэтому витрина и расчёт показывают одно число.
// Каталог кэшируется на edge-CDN; цена магазина меняется редко, а сброс
// кэша — через Cache-Control админки.
func handlePublicMaterials(storeSvc StoreService, authSvc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry, err := engmfg.DefaultMaterialRegistry()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Внутренняя ошибка сервера")
			return
		}
		rates := engprc.DefaultRates()
		if storeRates, ok := publicStoreRates(r, storeSvc, authSvc); ok {
			rates = *storeRates
		}
		out := make([]materialDTO, 0, 8)
		for _, m := range registry.Materials() {
			dto := materialDTO{
				Code:           string(m.Code),
				Name:           m.Name,
				NameRu:         m.NameRu,
				Category:       m.Category,
				Density:        m.Density,
				MinThicknessMM: m.MinThickness,
				MaxThicknessMM: m.MaxThickness,
				MaxWidthMM:     m.MaxWidthMm,
				MaxHeightMM:    m.MaxHeightMm,
				SwatchURL:      "/static-assets/pbr/" + string(m.Code) + "/color.jpg",
				Finishes:       materialFinishes[m.Code],
			}
			if price, ok := rates.Material[m.Code]; ok {
				dto.PricePerKgRub = rubMajor(price)
			}
			out = append(out, dto)
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, http.StatusOK, out)
	}
}

// rubMajor — цена в рублях в major-единицах (₽), округление до копеек не нужно:
// каталог-заглушка отдаётся витрине «как есть», форматирование — на клиенте.
func rubMajor(m domprc.Money) float64 {
	return m.Major(domprc.CurrencyRUB)
}
