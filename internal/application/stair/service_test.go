package stair

import (
	"reflect"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	engprc "stairplatform/internal/engine/pricing"
)

func mustLengthHelper(mm float64) engineering.Length {
	l, err := engineering.NewLength(mm)
	if err != nil {
		panic(err)
	}
	return l
}

// referenceConfig — эталонная конфигурация n=15 (совпадает с референсом
// Pricing Engine: H=2700, h0=180, шаг комфорта 630 → h=180, b=270, n=15).
func referenceConfig() Config {
	return Config{
		Width:             mustLengthHelper(900),
		Height:            mustLengthHelper(2700),
		Flight:            engineering.FlightStraight,
		StepHeight:        mustLengthHelper(180),
		StringerThickness: mustLengthHelper(50),
		StepThickness:     mustLengthHelper(40),
		Clearance:         mustLengthHelper(2500),
		RailingHeight:     mustLengthHelper(1000),
	}
}

// customRates — ставки пользователя (переопределение дефолтов).
func customRates() *engprc.Rates {
	r := engprc.DefaultRates()
	r.Material[dommfg.MaterialCode("STEEL-S235")] = domprc.NewMoney(20000) // 200 ₽/кг
	return &r
}

func TestCalculateValidPipeline(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(referenceConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}

	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.Flight.StepCount != 15 {
		t.Fatalf("step count = %d, want 15", res.Flight.StepCount)
	}
	if res.Flight.StepHeight.Millimeters() != 180 {
		t.Fatalf("step height = %v, want 180", res.Flight.StepHeight.Millimeters())
	}
	if res.Flight.TreadDepth.Millimeters() != 270 {
		t.Fatalf("tread depth = %v, want 270", res.Flight.TreadDepth.Millimeters())
	}

	if res.Package == nil || len(res.Package.Parts) == 0 {
		t.Fatal("package must contain parts")
	}
	if len(res.Package.BOM.Lines) == 0 {
		t.Fatal("package must contain BOM")
	}
	if len(res.Package.CutList.Items) == 0 {
		t.Fatal("package must contain cut list")
	}
	if res.Package.Nesting == nil || len(res.Package.Nesting.Sheets) == 0 {
		t.Fatal("package must contain nesting result")
	}

	if res.Mesh == nil {
		t.Fatal("preview mesh must be present")
	}
	if len(res.Mesh.Vertices) == 0 || len(res.Mesh.Triangles) == 0 {
		t.Fatal("preview mesh must contain vertices and triangles")
	}

	if res.Cost == nil {
		t.Fatal("cost dataset must be present")
	}
	if res.Price == nil {
		t.Fatal("price breakdown must be present")
	}

	// Финальная цена эталонного конвейера (100 ₽/кг, ставки по умолчанию).
	if res.Price.FinalPrice.Minor() != 286828274 {
		t.Fatalf("final price = %d, want 286828274", res.Price.FinalPrice.Minor())
	}
	if res.Price.FinalPrice.Major(domprc.CurrencyRUB) != 2868282.74 {
		t.Fatalf("final price = %.2f rub, want 2868282.74", res.Price.FinalPrice.Major(domprc.CurrencyRUB))
	}
}

func TestCalculateDeterminism(t *testing.T) {
	s := NewService()
	a, err := s.Calculate(referenceConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Calculate(referenceConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("calculate must be deterministic")
	}
}

func TestCalculateCustomRates(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(referenceConfig(), Options{Rates: customRates()})
	if err != nil {
		t.Fatal(err)
	}
	// Стоимость стали удвоена → финальная цена выше дефолтной.
	if res.Price.Material.Minor() <= 159359787 {
		t.Fatalf("material with doubled rate must exceed default, got %d", res.Price.Material.Minor())
	}
}

func TestCalculateBlockingValidation(t *testing.T) {
	s := NewService()
	// Целевая высота ступени 10 мм → n=270, h=10 вне диапазона 150-200 → Error.
	cfg := referenceConfig()
	cfg.StepHeight = mustLengthHelper(10)
	res, err := s.Calculate(cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking {
		t.Fatalf("expected blocking validation, got %+v", res.Validation)
	}
	if res.Validation.Valid {
		t.Fatal("blocking result must not be valid")
	}
	// Конвейер должен остановиться: цена и пакет отсутствуют.
	if res.Price != nil || res.Package != nil {
		t.Fatal("blocking validation must stop the pipeline (no price/package)")
	}
}

func TestCalculateInvalidInput(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Height = mustLengthHelper(0)
	if _, err := s.Calculate(cfg, Options{}); err == nil {
		t.Fatal("zero rise height must be rejected")
	}
}

func TestCalculateComfortStepBoundary(t *testing.T) {
	s := NewService()
	// Шаг комфорта 600 → b = 600 - 2*180 = 240 < 260 (диапазон tread 260-320)
	// → blocking с GEO-TREAD-DEPTH.
	cfg := referenceConfig()
	res, err := s.Calculate(cfg, Options{ComfortStep: 600})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking {
		t.Fatalf("expected blocking for b=240, got %+v", res.Validation)
	}
}

// referenceLShapeConfig — эталонная L-образная конфигурация (H=2700,
// n=15, n1=6, h=180, b=270, W=900, Wp=1000). Ответвление n1=6 даёт
// нижний марш 1620×1080 и верхний 2430×1620.
func referenceLShapeConfig() Config {
	cfg := referenceConfig()
	cfg.Flight = engineering.FlightLShape
	cfg.LandingWidth = mustLengthHelper(1000)
	cfg.LowerStepCount = 6
	return cfg
}

func TestCalculateLShapePipeline(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(referenceLShapeConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.LShape == nil {
		t.Fatal("l_shape flight must populate LShape result")
	}
	if res.LShape.StepCount != 15 || res.LShape.LowerStepCount != 6 || res.LShape.UpperStepCount != 9 {
		t.Fatalf("split = %d/%d/%d, want 15/6/9",
			res.LShape.StepCount, res.LShape.LowerStepCount, res.LShape.UpperStepCount)
	}
	if res.LShape.LowerHeight.Millimeters() != 1080 || res.LShape.UpperHeight.Millimeters() != 1620 {
		t.Fatalf("flight heights = %v/%v, want 1080/1620",
			res.LShape.LowerHeight.Millimeters(), res.LShape.UpperHeight.Millimeters())
	}
	// прямой марш должен оставаться нулевым.
	if res.Flight.StepCount != 0 {
		t.Fatalf("straight flight must be empty for l_shape, got %+v", res.Flight)
	}
	// полный конвейер: 35 деталей, 4 косоура, 16 проступей, 15 подступенков.
	if res.Package == nil || len(res.Package.Parts) != 35 {
		t.Fatalf("parts = %d, want 35", len(res.Package.Parts))
	}
	if res.Mesh == nil || len(res.Mesh.Vertices) == 0 {
		t.Fatal("l_shape pipeline must produce preview mesh")
	}
	if res.Measurement.Volume != 308700000 {
		t.Fatalf("volume = %v, want 308700000", res.Measurement.Volume)
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("l_shape pipeline must produce price")
	}
}

func TestCalculateLShapeLandingTooNarrow(t *testing.T) {
	s := NewService()
	// Wp=500 < W=900 → ошибка (EDR-0005 §7): невозможно выполнить расчёт.
	cfg := referenceLShapeConfig()
	cfg.LandingWidth = mustLengthHelper(500)
	if _, err := s.Calculate(cfg, Options{}); err == nil {
		t.Fatal("landing width below stair width must be rejected")
	}
}

// referenceUShapeConfig — эталонная П-образная конфигурация (EDR-0006):
// та же арифметика, что и у L-марша (H=2700, n=15, n1=6, h=180, b=270,
// W=900, Wp=1000); верхний марш разворачивается на 180°.
func referenceUShapeConfig() Config {
	cfg := referenceConfig()
	cfg.Flight = engineering.FlightUShape
	cfg.LandingWidth = mustLengthHelper(1000)
	cfg.LowerStepCount = 6
	return cfg
}

func TestCalculateUShapePipeline(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(referenceUShapeConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.UShape == nil {
		t.Fatal("u_shape flight must populate UShape result")
	}
	if res.UShape.StepCount != 15 || res.UShape.LowerStepCount != 6 || res.UShape.UpperStepCount != 9 {
		t.Fatalf("split = %d/%d/%d, want 15/6/9",
			res.UShape.StepCount, res.UShape.LowerStepCount, res.UShape.UpperStepCount)
	}
	if res.UShape.LowerHeight.Millimeters() != 1080 || res.UShape.UpperHeight.Millimeters() != 1620 {
		t.Fatalf("flight heights = %v/%v, want 1080/1620",
			res.UShape.LowerHeight.Millimeters(), res.UShape.UpperHeight.Millimeters())
	}
	if res.UShape.LowerRun.Millimeters() != 1620 || res.UShape.UpperRun.Millimeters() != 2430 {
		t.Fatalf("runs = %v/%v, want 1620/2430",
			res.UShape.LowerRun.Millimeters(), res.UShape.UpperRun.Millimeters())
	}
	// прямой марш должен оставаться нулевым.
	if res.Flight.StepCount != 0 {
		t.Fatalf("straight flight must be empty for u_shape, got %+v", res.Flight)
	}
	// полный конвейер: 35 деталей, 4 косоура, 16 проступей, 15 подступенков.
	if res.Package == nil || len(res.Package.Parts) != 35 {
		t.Fatalf("parts = %d, want 35", len(res.Package.Parts))
	}
	if res.Mesh == nil || len(res.Mesh.Vertices) == 0 {
		t.Fatal("u_shape pipeline must produce preview mesh")
	}
	if res.Measurement.Volume != 308700000 {
		t.Fatalf("volume = %v, want 308700000", res.Measurement.Volume)
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("u_shape pipeline must produce price")
	}
}

func TestCalculateUShapeLandingTooNarrow(t *testing.T) {
	s := NewService()
	// Wp=500 < W=900 → ошибка (EDR-0006 §7): невозможно выполнить расчёт.
	cfg := referenceUShapeConfig()
	cfg.LandingWidth = mustLengthHelper(500)
	if _, err := s.Calculate(cfg, Options{}); err == nil {
		t.Fatal("landing width below stair width must be rejected")
	}
}
