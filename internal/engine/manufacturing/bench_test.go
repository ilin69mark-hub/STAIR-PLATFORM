package manufacturing

import (
	"strconv"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
)

func mfgMustLength(t testing.TB, mm float64) engineering.Length {
	t.Helper()
	l, err := engineering.NewLength(mm)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// BenchmarkManufacture измеряет Manufacturing Stage (разложение на детали,
// BOM, раскрой) — наследует затраты Geometry, но фиксирует стоимость
// decompose+nesting (B3.1: параллельный per-material-group Nest).
func BenchmarkManufacture(b *testing.B) {
	cfg, _ := engineering.NewStairConfiguration(
		mfgMustLength(b, 900), mfgMustLength(b, 2700), engineering.FlightStraight)
	cfg.StepCount = 15
	cfg.StepHeight = mfgMustLength(b, 180)
	cfg.TreadDepth = mfgMustLength(b, 270)
	cfg.StringerThickness = mfgMustLength(b, 50)
	cfg.StepThickness = mfgMustLength(b, 40)
	gen, err := enggeo.Generate(cfg)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var last *dommfg.ManufacturingPackage
	for i := 0; i < b.N; i++ {
		pkg, err := Manufacture(cfg, gen)
		if err != nil {
			b.Fatal(err)
		}
		last = pkg
	}
	_ = last
}

// BenchmarkNest изолирует раскрой на крупном множестве деталей — цель
// B3.1 параллелизации по материальным группам.
func BenchmarkNest(b *testing.B) {
	// 1 материал × 60 позиций, имитация крупного BOM прямого марша.
	cut := dommfg.CutList{Items: make([]dommfg.CutItem, 0, 60)}
	for i := 0; i < 60; i++ {
		cut.Items = append(cut.Items, dommfg.CutItem{
			PartNumber:   dommfg.PartNumber("P-" + strconv.Itoa(i)),
			MaterialCode: "STEEL-S235",
			Thickness:    mfgMustLength(b, 40),
			Length:       mfgMustLength(b, 800),
			Width:        mfgMustLength(b, 270),
			Quantity:     1,
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Nest(cut, DefaultStockSheetRegistry(), DefaultKerf); err != nil {
			b.Fatal(err)
		}
	}
}
