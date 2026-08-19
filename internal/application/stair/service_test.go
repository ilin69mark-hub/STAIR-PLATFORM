package stair

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	domprc "stairplatform/internal/domain/pricing"
	"stairplatform/internal/engine/constraint"
	engprc "stairplatform/internal/engine/pricing"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
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
		Riser:             true,
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
	res, err := s.Calculate(context.Background(), referenceConfig(), Options{})
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
	if res.Price.FinalPrice.Minor() != 327218990 {
		t.Fatalf("final price = %d, want 327218990", res.Price.FinalPrice.Minor())
	}
	if res.Price.FinalPrice.Major(domprc.CurrencyRUB) != 3272189.90 {
		t.Fatalf("final price = %.2f rub, want 3272189.90", res.Price.FinalPrice.Major(domprc.CurrencyRUB))
	}
}

func TestCalculateDeterminism(t *testing.T) {
	s := NewService()
	a, err := s.Calculate(context.Background(), referenceConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Calculate(context.Background(), referenceConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("calculate must be deterministic")
	}
}

func TestCalculateCustomRates(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), referenceConfig(), Options{Rates: customRates()})
	if err != nil {
		t.Fatal(err)
	}
	// Стоимость стали удвоена → финальная цена выше дефолтной.
	if res.Price.Material.Minor() <= 182033277 {
		t.Fatalf("material with doubled rate must exceed default, got %d", res.Price.Material.Minor())
	}
}

func TestCalculateBlockingValidation(t *testing.T) {
	s := NewService()
	// Целевая высота ступени 10 мм → n=270, h=10 вне диапазона 150-200 → Error.
	cfg := referenceConfig()
	cfg.StepHeight = mustLengthHelper(10)
	res, err := s.Calculate(context.Background(), cfg, Options{})
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

// TestCalculateTallFlightSucceeds — прямая лестница на пределе энвелопа
// H=6000 (максимально поддерживаемая высота) должна проходить весь конвейер:
// косоур (≈10200×6050) помещается на добавленный лист стали 10400×6200,
// поэтому выдаётся валидный результат с ценой, а не блокировка MFG-0012.
func TestCalculateTallFlightSucceeds(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Height = mustLengthHelper(6000)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("tall flight within envelope must calculate: %v", err)
	}
	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("tall flight must produce price")
	}
	if res.Package == nil || len(res.Package.Parts) == 0 {
		t.Fatal("tall flight must produce nesting package")
	}
}

// assertBlockingInput проверяет, что результат блокирующий (advisory) с
// русским объяснением и указанным кодом — вместо технической ошибки.
func assertBlockingInput(t *testing.T, res *Result, wantCode constraint.RuleCode, wantSubstring string) {
	t.Helper()
	if res == nil {
		t.Fatal("expected advisory result, got nil")
	}
	if !res.Validation.Blocking || res.Validation.Valid {
		t.Fatalf("expected blocking result, got %+v", res.Validation)
	}
	if len(res.Validation.Issues) == 0 {
		t.Fatalf("expected an issue, got none")
	}
	it := res.Validation.Issues[0]
	if string(it.Code) != string(wantCode) {
		t.Fatalf("issue code = %q, want %q", it.Code, wantCode)
	}
	if it.Param == "" || it.Guide == "" {
		t.Fatalf("issue must carry Param and Guide: %+v", it)
	}
	if wantSubstring != "" && !strings.Contains(it.Guide, wantSubstring) {
		t.Fatalf("guide %q does not contain %q", it.Guide, wantSubstring)
	}
	if res.Price != nil || res.Package != nil {
		t.Fatal("blocked result must not carry price/package")
	}
}

// TestCalculateExceedsMaxHeight — высота за пределами поддерживаемого
// энелопа (6000 мм) возвращается как блокирующая подсказка, а не ошибка.
func TestCalculateExceedsMaxHeight(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Height = mustLengthHelper(7000)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.GEO_HEIGHT, "6000 мм")
}

// TestCalculateExceedsMaterialMaxHeight — высота в пределах глобального
// энелопа (6000 мм), но выше предела конкретного алюминия (4550 мм),
// возвращается как блокирующая подсказка по материалу.
func TestCalculateExceedsMaterialMaxHeight(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Material = dommfg.MaterialCode("ALUM-5083")
	cfg.Height = mustLengthHelper(5000)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.MFG_MATERIAL, "4550 мм")
}

// TestCalculateExceedsMaterialMaxWidth — ширина марша выше предела
// материала (3000 мм) возвращается как блокирующая подсказка.
func TestCalculateExceedsMaterialMaxWidth(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Material = dommfg.MaterialCode("STEEL-S235")
	cfg.Width = mustLengthHelper(4000)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.MFG_MATERIAL, "3000 мм")
}

// TestCalculateWithinMaterialLimits — габариты на границе предела каждого
// материала проходят весь конвейер (не блокируются размерными лимитами):
// макс. высота при типовой ширине и макс. ширина при типовой высоте.
func TestCalculateWithinMaterialLimits(t *testing.T) {
	s := NewService()
	limits := map[dommfg.MaterialCode]struct{ height, width int }{
		"STEEL-S235": {6000, 3000},
		"ALUM-5083":  {4550, 3000},
		"WOOD-OAK":   {4550, 3000},
	}
	for code, lim := range limits {
		cfgH := referenceConfig()
		cfgH.Material = code
		cfgH.Height = mustLengthHelper(float64(lim.height))
		resH, err := s.Calculate(context.Background(), cfgH, Options{})
		if err != nil {
			t.Fatalf("%s (height limit): input issues must not be hard errors: %v", code, err)
		}
		if resH.Validation.Blocking {
			t.Fatalf("%s: height %d must pass material limit, got %+v",
				code, lim.height, resH.Validation)
		}

		cfgW := referenceConfig()
		cfgW.Material = code
		cfgW.Width = mustLengthHelper(float64(lim.width))
		resW, err := s.Calculate(context.Background(), cfgW, Options{})
		if err != nil {
			t.Fatalf("%s (width limit): input issues must not be hard errors: %v", code, err)
		}
		if resW.Validation.Blocking {
			t.Fatalf("%s: width %d must pass material limit, got %+v",
				code, lim.width, resW.Validation)
		}
	}
}

// TestCalculateSpiralExceedsMaxRadius — наружный радиус спирали за пределами
// поддерживаемого максимума (5000 мм) возвращается как блокирующая подсказка.
func TestCalculateSpiralExceedsMaxRadius(t *testing.T) {
	s := NewService()
	cfg := referenceSpiralConfig()
	cfg.OuterRadius = mustLengthHelper(6000)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.GEO_SPIRAL_RADIUS, "5000 мм")
}

func TestCalculateInvalidInput(t *testing.T) {
	s := NewService()
	cfg := referenceConfig()
	cfg.Height = mustLengthHelper(0)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.GEO_HEIGHT, "больше 0 мм")
}

func TestCalculateComfortStepBoundary(t *testing.T) {
	s := NewService()
	// Шаг комфорта 600 → b = 600 - 2*180 = 240 < 260 (диапазон tread 260-320)
	// → blocking с GEO-TREAD-DEPTH.
	cfg := referenceConfig()
	res, err := s.Calculate(context.Background(), cfg, Options{ComfortStep: 600})
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
	res, err := s.Calculate(context.Background(), referenceLShapeConfig(), Options{})
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
	// полный конвейер: 35 деталей, 4 косоура, 16 проступей, 15 подступенков
	// подступенков.
	if res.Package == nil || len(res.Package.Parts) != 35 {
		t.Fatalf("parts = %d, want 35", len(res.Package.Parts))
	}
	if res.Mesh == nil || len(res.Mesh.Vertices) == 0 {
		t.Fatal("l_shape pipeline must produce preview mesh")
	}
	if math.Abs(res.Measurement.Volume-338451360.854) > 1 {
		t.Fatalf("volume = %v, want 338451360.854", res.Measurement.Volume)
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("l_shape pipeline must produce price")
	}
}

func TestCalculateLShapeLandingTooNarrow(t *testing.T) {
	s := NewService()
	// Wp=500 < W=900 → блокирующая подсказка (EDR-0005 §7): площадка
	// должна быть не уже марша.
	cfg := referenceLShapeConfig()
	cfg.LandingWidth = mustLengthHelper(500)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.GEO_LANDING_WIDTH, "меньше ширины марша 900 мм")
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
	res, err := s.Calculate(context.Background(), referenceUShapeConfig(), Options{})
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
	// полный конвейер: 35 деталей, 4 косоура, 16 проступей, 15 подступенков
	// подступенков.
	if res.Package == nil || len(res.Package.Parts) != 35 {
		t.Fatalf("parts = %d, want 35", len(res.Package.Parts))
	}
	if res.Mesh == nil || len(res.Mesh.Vertices) == 0 {
		t.Fatal("u_shape pipeline must produce preview mesh")
	}
	if math.Abs(res.Measurement.Volume-338451360.854) > 1 {
		t.Fatalf("volume = %v, want 338451360.854", res.Measurement.Volume)
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("u_shape pipeline must produce price")
	}
}

func TestCalculateUShapeLandingTooNarrow(t *testing.T) {
	s := NewService()
	// Wp=500 < W=900 → блокирующая подсказка (EDR-0006 §7).
	cfg := referenceUShapeConfig()
	cfg.LandingWidth = mustLengthHelper(500)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.GEO_LANDING_WIDTH, "меньше ширины марша 900 мм")
}

// referenceSpiralConfig — эталонная спиральная конфигурация (EDR-0007):
// H=2700, h0=180, W=500, R=800 → n=15, h=180, r=300, r_walk=633.33,
// b_walk≈265.29 (в диапазоне 260–320), S≈625.29 (600–640), α≈34.15°.
func referenceSpiralConfig() Config {
	cfg := referenceConfig()
	cfg.Width = mustLengthHelper(500)
	cfg.Flight = engineering.FlightSpiral
	cfg.OuterRadius = mustLengthHelper(800)
	return cfg
}

func TestCalculateSpiralPipeline(t *testing.T) {
	s := NewService()
	res, err := s.Calculate(context.Background(), referenceSpiralConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Valid || res.Validation.Blocking {
		t.Fatalf("expected valid configuration, got %+v", res.Validation)
	}
	if res.Spiral == nil {
		t.Fatal("spiral flight must populate Spiral result")
	}
	if res.Spiral.StepCount != 15 {
		t.Fatalf("step count = %d, want 15", res.Spiral.StepCount)
	}
	if res.Spiral.StepHeight.Millimeters() != 180 {
		t.Fatalf("step height = %v, want 180", res.Spiral.StepHeight.Millimeters())
	}
	if res.Spiral.ColumnRadius.Millimeters() != 300 {
		t.Fatalf("column radius = %v, want 300", res.Spiral.ColumnRadius.Millimeters())
	}
	if res.Spiral.OuterRadius.Millimeters() != 800 {
		t.Fatalf("outer radius = %v, want 800", res.Spiral.OuterRadius.Millimeters())
	}
	// полный конвейер: 1 колонна + 15 проступей = 16 деталей.
	if res.Package == nil || len(res.Package.Parts) != 16 {
		t.Fatalf("parts = %d, want 16", len(res.Package.Parts))
	}
	if res.Mesh == nil || len(res.Mesh.Vertices) == 0 {
		t.Fatal("spiral pipeline must produce preview mesh")
	}
	if res.Price == nil || res.Price.FinalPrice.Minor() <= 0 {
		t.Fatal("spiral pipeline must produce price")
	}
}

func TestCalculateSpiralOuterRadiusTooSmall(t *testing.T) {
	s := NewService()
	// R=400 ≤ W=500 → блокирующая подсказка (EDR-0007 §7): радиус должен
	// превышать ширину марша.
	cfg := referenceSpiralConfig()
	cfg.OuterRadius = mustLengthHelper(400)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	assertBlockingInput(t, res, constraint.GEO_SPIRAL_RADIUS, "больше ширины марша 500 мм")
}

func TestCalculateSpiralBlockedCarriesSuggestions(t *testing.T) {
	s := NewService()
	// W=3000 при H=6000: проступь у колонны < 100 мм — блокирующая
	// подсказка. Советник должен прикрепить варианты с уменьшенной шириной,
	// каждый из которых решается без ошибок.
	cfg := referenceSpiralConfig()
	cfg.Height = mustLengthHelper(6000)
	cfg.Width = mustLengthHelper(3000)
	cfg.OuterRadius = mustLengthHelper(3100)
	cfg.StepHeight = mustLengthHelper(190)
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("input issues must not be hard errors: %v", err)
	}
	if !res.Validation.Blocking {
		t.Fatalf("expected blocking result, got %+v", res.Validation)
	}
	var issue *validation.Issue
	for i := range res.Validation.Issues {
		if res.Validation.Issues[i].Code == constraint.GEO_SPIRAL_TREAD {
			issue = &res.Validation.Issues[i]
			break
		}
	}
	if issue == nil {
		t.Fatalf("missing GEO_SPIRAL_TREAD issue: %+v", res.Validation.Issues)
	}
	if len(issue.Suggestions) == 0 {
		t.Fatalf("spiral blocking must carry width-reduced suggestions")
	}
	for _, s := range issue.Suggestions {
		if s.WidthMm >= 3000 || s.OuterRadiusMm <= 0 {
			t.Fatalf("suggestion %+v must reduce width and carry radius", s)
		}
		if _, err := solver.SolveSpiral(
			engineering.Length(s.StepHeightMm*float64(s.StepCount)),
			engineering.Length(s.StepHeightMm),
			engineering.Length(s.WidthMm),
			engineering.Length(s.OuterRadiusMm),
		); err != nil {
			t.Fatalf("suggestion %+v must solve without errors: %v", s, err)
		}
	}
}

func BenchmarkCalculatePipeline(b *testing.B) {
	s := NewService()
	cfg := referenceConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Calculate(context.Background(), cfg, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestCalculateCancelled(t *testing.T) {
	// Отменённый контекст: конвейер не выполняется, возвращается
	// context.Canceled (B2, EDR-0033 §3.1).
	s := NewService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.Calculate(ctx, referenceConfig(), Options{}); err == nil {
		t.Fatal("expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCalculateMaterialSelection(t *testing.T) {
	s := NewService()
	materials := []dommfg.MaterialCode{"STEEL-S235", "ALUM-5083", "WOOD-OAK"}
	prices := make(map[dommfg.MaterialCode]int64)
	for _, m := range materials {
		cfg := referenceConfig()
		cfg.Material = m
		res, err := s.Calculate(context.Background(), cfg, Options{})
		if err != nil {
			t.Fatalf("%s: %v", m, err)
		}
		if !res.Validation.Valid || res.Validation.Blocking {
			t.Fatalf("%s: validation %+v", m, res.Validation)
		}
		if res.Package == nil || len(res.Package.Parts) == 0 {
			t.Fatalf("%s: no parts in package", m)
		}
		for _, p := range res.Package.Parts {
			if p.Material != m {
				t.Fatalf("%s: part %s material = %s, want %s", m, p.Number, p.Material, m)
			}
		}
		// Материалы имеют разные плотность/ставку → финальная цена различается.
		prices[m] = res.Price.FinalPrice.Minor()
		if res.Price == nil {
			t.Fatalf("%s: price missing", m)
		}
		// Раскрой выполнен (дерево покрыто крупными листами каталога).
		if res.Package.Nesting == nil || len(res.Package.Nesting.Sheets) == 0 {
			t.Fatalf("%s: nesting missing", m)
		}
	}
	if prices["STEEL-S235"] == prices["ALUM-5083"] || prices["STEEL-S235"] == prices["WOOD-OAK"] {
		t.Fatalf("prices must differ across materials, got %v", prices)
	}
}

func TestCalculateMaterialValidation(t *testing.T) {
	s := NewService()

	// Неизвестный материал — блокирующий MFG-MATERIAL.
	cfg := referenceConfig()
	cfg.Material = "TITANIUM-X"
	res, err := s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking || res.Validation.Valid {
		t.Fatalf("unknown material must block, got %+v", res.Validation)
	}
	iss := res.Validation.Issues[0]
	if iss.Code != constraint.MFG_MATERIAL || !strings.Contains(iss.Message, "не найден") {
		t.Fatalf("unexpected issue %+v", iss)
	}

	// Дуб не поддерживает косоур 150 мм — блокирующий MFG-MATERIAL.
	cfg = referenceConfig()
	cfg.Material = "WOOD-OAK"
	cfg.StringerThickness = mustLengthHelper(150)
	res, err = s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking || res.Validation.Valid {
		t.Fatalf("wood+150mm stringer must block, got %+v", res.Validation)
	}
	iss = res.Validation.Issues[0]
	if iss.Code != constraint.MFG_MATERIAL {
		t.Fatalf("code = %s, want %s", iss.Code, constraint.MFG_MATERIAL)
	}
	if !strings.Contains(iss.Guide, "20–60") {
		t.Fatalf("guide must mention wood thickness range, got %q", iss.Guide)
	}

	// Дуб при большом подъёме (H=5000 выше предела дуба 4550 мм) —
	// блокирующая подсказка про предельную высоту материала.
	cfg = referenceConfig()
	cfg.Material = "WOOD-OAK"
	cfg.Height = mustLengthHelper(5000)
	res, err = s.Calculate(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Validation.Blocking || res.Validation.Valid {
		t.Fatalf("wood + H=5000 must block, got %+v", res.Validation)
	}
	iss = res.Validation.Issues[0]
	if iss.Code != constraint.MFG_MATERIAL {
		t.Fatalf("code = %s, want %s", iss.Code, constraint.MFG_MATERIAL)
	}
	if !strings.Contains(iss.Guide, "4550") {
		t.Fatalf("guide must mention oak max height 4550, got %q", iss.Guide)
	}
}
