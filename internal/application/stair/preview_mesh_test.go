package stair

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/domain/manufacturing"
)

// baseConfig — рабочая конфигурация для проверки плотности (S-165).
// Радиус скругления носа НЕ задаётся здесь: он приходит из каталога
// материала (Material.TreadNoseRadiusMm), и у металла он нулевой — тогда
// огрублять нечего и приёмочный тест осмыслен только на дереве.
func baseConfig(material string, w, h float64) Config {
	return Config{
		Width:             engineering.Length(w),
		Height:            engineering.Length(h),
		Flight:            engineering.FlightStraight,
		StepHeight:        engineering.Length(180),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		Riser:             true,
		Clearance:         engineering.Length(2500),
		RailingHeight:     engineering.Length(1000),
		ApproachSpace:     engineering.Length(1000),
		Material:          manufacturing.MaterialCode(material),
		TreadMaterial:     manufacturing.MaterialCode(material),
	}
}

// TestPreviewMeshDoesNotChangePrice — главный приёмочный тест S-165.
//
// Витрина просит облегчённую сетку (PreviewMesh), админка не просит.
// Если цена, объём или раскрой отличаются — одна и та же лестница стоит
// по-разному в зависимости от маршрута. Это ровно тот класс ошибок, что
// уже случался в проекте дважды: со ставками и с таблицей норматива.
func TestPreviewMeshDoesNotChangePrice(t *testing.T) {
	for _, material := range []string{"WOOD-OAK", "STEEL-S235"} {
		cfg := baseConfig(material, 900, 2700)
		s := NewService()
		ctx := context.Background()

		admin, err := s.Calculate(ctx, cfg, Options{})
		if err != nil {
			t.Fatalf("%s расчёт админки: %v", material, err)
		}
		store, err := s.Calculate(ctx, cfg, Options{PreviewMesh: true})
		if err != nil {
			t.Fatalf("%s расчёт витрины: %v", material, err)
		}

		if admin.Price == nil || store.Price == nil {
			t.Fatalf("%s: цена не посчитана", material)
		}
		if admin.Price.FinalPrice != store.Price.FinalPrice {
			t.Errorf("%s: ЦЕНА РАЗОШЛАСЬ — админка %v, витрина %v",
				material, admin.Price.FinalPrice, store.Price.FinalPrice)
		}
		if admin.Measurement.Volume != store.Measurement.Volume {
			t.Errorf("%s: объём разошёлся — %v против %v",
				material, admin.Measurement.Volume, store.Measurement.Volume)
		}
		if admin.Measurement.SurfaceArea != store.Measurement.SurfaceArea {
			t.Errorf("%s: площадь разошлась — %v против %v",
				material, admin.Measurement.SurfaceArea, store.Measurement.SurfaceArea)
		}
		if len(admin.Package.Parts) != len(store.Package.Parts) {
			t.Errorf("%s: число деталей разошлось — %d против %d",
				material, len(admin.Package.Parts), len(store.Package.Parts))
		}

		mt, st := len(admin.Mesh.Triangles), len(store.Mesh.Triangles)
		if st >= mt {
			t.Errorf("%s: облегчённый меш не легче — %d против %d треугольников", material, st, mt)
		}
		t.Logf("%-11s цена %v (совпала), объём %.1f (совпал), меш %d → %d треугольников (−%.0f%%)",
			material, admin.Price.FinalPrice, admin.Measurement.Volume,
			mt, st, float64(mt-st)/float64(mt)*100)
	}
}

// TestPreviewMeshKeepsRoomFitWarning — предупреждение о вписывании в
// помещение вычисляется по производственной модели, облегчённый второй
// проход его не подменяет и не теряет.
func TestPreviewMeshKeepsRoomFitWarning(t *testing.T) {
	cfg := baseConfig("WOOD-OAK", 900, 2700)
	cfg.RoomWidth = engineering.Length(900) // заведомо мало
	cfg.RoomLength = engineering.Length(900)

	s := NewService()
	admin, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	store, err := s.Calculate(context.Background(), cfg, Options{PreviewMesh: true})
	if err != nil {
		t.Fatal(err)
	}
	has := func(r *Result) bool {
		for _, is := range r.Validation.Issues {
			if is.Code == "room_fit" {
				return true
			}
		}
		return false
	}
	if !has(admin) {
		t.Fatal("админка не выдала room_fit при заведомо тесном помещении")
	}
	if !has(store) {
		t.Error("витрина потеряла room_fit — предупреждение обязано быть и там")
	}
}
