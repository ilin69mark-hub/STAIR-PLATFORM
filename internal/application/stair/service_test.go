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
