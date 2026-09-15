package variation

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/constraint"
	enggeo "stairplatform/internal/engine/geometry"
	"stairplatform/internal/engine/solver"
	"stairplatform/internal/engine/validation"
)

func buildLShape(roomW, roomL float64) *engineering.StairConfiguration {
	c := &engineering.StairConfiguration{
		Width:             engineering.Length(1000),
		Height:            engineering.Length(3000),
		Flight:            engineering.FlightLShape,
		Riser:             true,
		LandingWidth:      engineering.Length(1200),
		LandingDepth:      engineering.Length(1500),
		LowerStepCount:    6,
		RoomWidth:         engineering.Length(roomW),
		RoomLength:        engineering.Length(roomL),
		Clearance:         engineering.Length(50),
		RailingHeight:     engineering.Length(900),
		StringerThickness: engineering.Length(40),
		StepThickness:     engineering.Length(40),
	}
	r, err := solver.SolveLShape(engineering.Length(3000), engineering.Length(180), 6, engineering.Length(1200), 630)
	if err != nil {
		panic(err)
	}
	r.Apply(c)
	return c
}

func TestForRoomFit_ProducesFittingVariants(t *testing.T) {
	cfg := buildLShape(4500, 4500)
	vars := ForRoomFit(context.Background(), cfg, nil)
	if len(vars) == 0 {
		t.Fatalf("expected variants, got 0")
	}
	for _, v := range vars {
		if len(v.Config) == 0 {
			t.Errorf("variant %q has empty config", v.Title)
		}
		if !v.Fits {
			t.Errorf("variant %q marked as not fitting", v.Title)
		}
	}
}

func TestForRoomFit_SmallRoom_NoVariants(t *testing.T) {
	cfg := buildLShape(600, 600)
	if got := ForRoomFit(context.Background(), cfg, nil); len(got) != 0 {
		t.Fatalf("with too small room expected 0 variants, got %d", len(got))
	}
}

func TestForRoomFit_NoRoom_Empty(t *testing.T) {
	cfg := buildLShape(0, 0)
	if got := ForRoomFit(context.Background(), cfg, nil); got != nil {
		t.Fatalf("without room expected nil, got %d", len(got))
	}
}

// buildStraight — прямой марш для проверки fit-check (зеркало buildLShape).
func buildStraight(roomW, roomL float64) *engineering.StairConfiguration {
	c := &engineering.StairConfiguration{
		Width:             engineering.Length(1000),
		Height:            engineering.Length(3000),
		Flight:            engineering.FlightStraight,
		Riser:             true,
		RoomWidth:         engineering.Length(roomW),
		RoomLength:        engineering.Length(roomL),
		Clearance:         engineering.Length(50),
		RailingHeight:     engineering.Length(900),
		StringerThickness: engineering.Length(40),
		StepThickness:     engineering.Length(40),
	}
	r, err := solver.Solve(engineering.Length(3000), engineering.Length(180), 630)
	if err != nil {
		panic(err)
	}
	r.Apply(c)
	return c
}

func TestForRoomFit_Straight_ProducesFittingVariants(t *testing.T) {
	cfg := buildStraight(4500, 4500)
	vars := ForRoomFit(context.Background(), cfg, nil)
	if len(vars) == 0 {
		t.Fatalf("expected variants for straight, got 0")
	}
	for _, v := range vars {
		if len(v.Config) == 0 {
			t.Errorf("variant %q has empty config", v.Title)
		}
	}
}

func TestForRoomFit_Straight_SmallRoom_NoVariants(t *testing.T) {
	cfg := buildStraight(600, 600)
	if got := ForRoomFit(context.Background(), cfg, nil); len(got) != 0 {
		t.Fatalf("with too small room expected 0 variants, got %d", len(got))
	}
}

// TestForRoomFit_VariantsPassNorms — регрессия бага «сделать круче»:
// варианты, предлагаемые пользователю (в т.ч. «круче»), не должны
// нарушать нормы (проступь 260–320 мм и пр.), иначе применение ведёт к
// блокировке расчёта. H=3000, шаг комфорта 640: исходный прямой (забег
// ~4760) не влезает в 4300×4300, а «круче» (забег ~4240) влезает и обязан
// проходить нормы (проступь 265 мм).
func TestForRoomFit_VariantsPassNorms(t *testing.T) {
	set := constraint.StandardProfile("STANDARD")
	c := &engineering.StairConfiguration{
		Width:             engineering.Length(1000),
		Height:            engineering.Length(3000),
		Flight:            engineering.FlightStraight,
		Riser:             true,
		RoomWidth:         engineering.Length(5300),
		RoomLength:        engineering.Length(5300),
		Clearance:         engineering.Length(50),
		RailingHeight:     engineering.Length(900),
		StringerThickness: engineering.Length(40),
		StepThickness:     engineering.Length(40),
		Length:            engineering.Length(640), // шаг комфорта
	}
	r, err := solver.Solve(engineering.Length(3000), engineering.Length(180), 640)
	if err != nil {
		panic(err)
	}
	r.Apply(c)
	c.Length = engineering.Length(640) // шаг комфорта (Apply перезаписывает Length на забег)

	vars := ForRoomFit(context.Background(), c, set)
	if len(vars) == 0 {
		t.Fatalf("expected at least the 'круче' variant for 4300x4300 room, got 0")
	}
	for _, v := range vars {
		if !v.PassesNorms {
			t.Errorf("variant %q is offered but does not pass norms (would block on apply)", v.Title)
		}
	}
	foundSteeper := false
	for _, v := range vars {
		if strings.Contains(v.Title, "круче") {
			foundSteeper = true
		}
	}
	if !foundSteeper {
		t.Errorf("expected a 'круче' (steeper) variant among %d offered", len(vars))
	}
}

// ширина марша 1200, помещение 3000×3000, H=2000: прямой не влезает по
// длине, но L/П-образная влезают при уменьшенной ширине марша. Проверяем,
// что адаптивный подбор предлагает вариант с шириной < 1200 и Fits=true.
func TestForRoomFit_Straight_AdaptiveWidth(t *testing.T) {
	cfg := buildStraight(3000, 3000)
	cfg.Width = engineering.Length(1200)
	cfg.Height = engineering.Length(2000)
	r, err := solver.Solve(engineering.Length(2000), engineering.Length(180), 630)
	if err != nil {
		panic(err)
	}
	r.Apply(cfg)

	vars := ForRoomFit(context.Background(), cfg, nil)
	if len(vars) == 0 {
		t.Fatalf("expected adaptive variants, got 0")
	}
	foundNarrower := false
	for _, v := range vars {
		w, perr := strconv.ParseFloat(v.Config["widthMM"], 64)
		if perr != nil {
			t.Fatalf("bad widthMM in variant %q: %v", v.Title, perr)
		}
		if !v.Fits {
			t.Errorf("variant %q must fit", v.Title)
		}
		if w < 1200 {
			foundNarrower = true
		}
	}
	if !foundNarrower {
		t.Errorf("expected at least one variant with reduced width (<1200), got %d variants", len(vars))
	}
}

// ширина марша 1200, помещение 4300×2000, H=2700: по физике L не влезает
// (нужна глубина ≥3 м), а П-образная влезает при достаточной ширине помещения.
// Учитываем, что габарит теперь включает зону подхода (EDR-0023, approachSpace
// применяется ко всем типам), поэтому ширина помещения взята с запасом под
// забег + площадку + подход (~1000 мм).
func TestForRoomFit_Straight_SpiralOnlyAt2700(t *testing.T) {
	cfg := buildStraight(4300, 2000)
	cfg.Width = engineering.Length(1200)
	cfg.Height = engineering.Length(2700)
	r, err := solver.Solve(engineering.Length(2700), engineering.Length(180), 630)
	if err != nil {
		panic(err)
	}
	r.Apply(cfg)

	vars := ForRoomFit(context.Background(), cfg, nil)
	var hasSpiral, hasL, hasU bool
	for _, v := range vars {
		switch v.Config["flight"] {
		case "spiral":
			hasSpiral = true
		case "l_shape":
			hasL = true
		case "u_shape":
			hasU = true
		}
	}
	if !hasSpiral {
		t.Errorf("expected a spiral variant for 4300x2000@2700, got %d variants", len(vars))
	}
	// П-образная (П) при ширине 1200 и комнате 4300 влезает (учтён подход),
	// а L-образная требует глубины ≥3 м и здесь не помещается.
	if hasL {
		t.Errorf("L-shape must not fit in 4300x2000@2700, but a variant was offered")
	}
	if !hasU {
		t.Errorf("expected a U-shape variant for 4300x2000@2700 (it fits with approach), got %d variants", len(vars))
	}
}

// TestForAngle_ProducesFittingVariants — при большой проступи (400 мм) и
// H=3000 мм угол получается слишком пологим (<30°), и советник при
// фиксированной проступи не может его исправить. ForAngle должен подобрать
// и число ступеней, и проступь, чтобы угол вошёл в норму 30–45°.
func TestForAngle_ProducesFittingVariants(t *testing.T) {
	set := constraint.StandardProfile("test")
	cfg := &engineering.StairConfiguration{
		Flight:            engineering.FlightStraight,
		Height:            engineering.Length(3000),
		Width:             engineering.Length(1000),
		StepHeight:        engineering.Length(190),
		Length:            engineering.Length(400),
		RailingHeight:     engineering.Length(900),
		StringerThickness: engineering.Length(40),
		StepThickness:     engineering.Length(40),
	}
	vars := ForAngle(context.Background(), cfg, set)
	if len(vars) == 0 {
		t.Fatalf("expected angle variations, got 0")
	}
	for _, v := range vars {
		if !v.Fits {
			t.Errorf("variation %q must fit", v.Title)
		}
		sh, err := strconv.ParseFloat(v.Config["stepHeightMM"], 64)
		if err != nil {
			t.Fatalf("bad stepHeightMM: %v", err)
		}
		bs, err := strconv.ParseFloat(v.Config["comfortStepMM"], 64)
		if err != nil {
			t.Fatalf("bad comfortStepMM: %v", err)
		}
		r, err := solver.Solve(engineering.Length(3000), engineering.Length(sh), bs)
		if err != nil {
			t.Fatalf("solve failed: %v", err)
		}
		deg := r.Angle.Degrees()
		if deg < 29.9 || deg > 45.1 {
			t.Errorf("variant %q gives angle %.1f°, out of 30–45°", v.Title, deg)
		}
	}
}

// TestFromSuggestions_Converts — готовые варианты советника превращаются в
// интерактивные Variations с заполненным конфигом.
func TestFromSuggestions_Converts(t *testing.T) {
	cfg := &engineering.StairConfiguration{
		Flight:     engineering.FlightStraight,
		Height:     engineering.Length(3000),
		Width:      engineering.Length(1000),
		StepHeight: engineering.Length(190),
		Length:     engineering.Length(280),
	}
	issue := &validation.Issue{
		Code: constraint.GEO_STEP_HEIGHT,
		Suggestions: []validation.Suggestion{
			{StepCount: 16, StepHeightMm: 187.5, TreadDepthMm: 280, AngleDeg: 33.8},
		},
	}
	vars := FromSuggestions(issue, cfg)
	if len(vars) != 1 {
		t.Fatalf("expected 1 variation, got %d", len(vars))
	}
	if vars[0].Config["stepHeightMM"] == "" {
		t.Fatalf("config missing stepHeightMM")
	}
}

// nearlyEq сравнивает float64 с относительной точностью 1e-6.
func nearlyEq(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

// testLShapeConfig строит минимальный L-образный конфиг, пригодный для
// enggeo.Generate (аналог makeLShapeConfigForTest в пакете geometry).
func testLShapeConfig() *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2700),
		Flight:            engineering.FlightLShape,
		StepCount:         15,
		StepHeight:        engineering.Length(180),
		TreadDepth:        engineering.Length(270),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		LowerStepCount:    7,
		LandingWidth:      engineering.Length(1000),
	}
}

// testUShapeConfig строит минимальный П-образный конфиг для enggeo.Generate.
func testUShapeConfig() *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2700),
		Flight:            engineering.FlightUShape,
		StepCount:         15,
		StepHeight:        engineering.Length(180),
		TreadDepth:        engineering.Length(270),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		LowerStepCount:    7,
		LandingWidth:      engineering.Length(1000),
	}
}

// testSpiralConfig строит минимальный спиральный конфиг для enggeo.Generate.
func testSpiralConfig() *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2700),
		Flight:            engineering.FlightSpiral,
		StepCount:         15,
		StepHeight:        engineering.Length(180),
		TreadDepth:        engineering.Length(270),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		OuterRadius:       engineering.Length(1300), // R > W → колонна > 0
	}
}

// TestRoomFitBBoxIncludesApproach — прямой unit-тест габаритного бокса,
// по которому ForRoomFit принимает решение о вписываемости в помещение.
// Увеличение ApproachSpace на Δ должно сдвинуть bbox ровно на Δ по оси X
// (EDR-0023) для L-образного, П-образного и спирального марша; комната,
// впритык к меньшему bbox, должна вмещать вариант с меньшим подходом и
// НЕ вмещать вариант с бóльшим — доказательство, что bbox → room_fit → Fits
// корректно учитывает зону подхода для всех типов.
func TestRoomFitBBoxIncludesApproach(t *testing.T) {
	const a1, a2 = 500.0, 1500.0
	delta := a2 - a1
	cases := []struct {
		name string
		make func() *engineering.StairConfiguration
	}{
		{"l_shape", testLShapeConfig},
		{"u_shape", testUShapeConfig},
		{"spiral", testSpiralConfig},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c1 := tc.make()
			c1.ApproachSpace = engineering.Length(a1)
			c2 := tc.make()
			c2.ApproachSpace = engineering.Length(a2)

			r1, err := enggeo.Generate(context.Background(), c1)
			if err != nil {
				t.Fatalf("generate a1: %v", err)
			}
			r2, err := enggeo.Generate(context.Background(), c2)
			if err != nil {
				t.Fatalf("generate a2: %v", err)
			}
			bb1 := r1.Measurement.BoundingBox
			bb2 := r2.Measurement.BoundingBox

			// Бокс сдвинут ровно на Δ по X.
			if !nearlyEq(bb2.Min.X-bb1.Min.X, delta) {
				t.Fatalf("bbox.Min.X shift = %v, want %v (a2−a1) for %s", bb2.Min.X-bb1.Min.X, delta, tc.name)
			}
			if !nearlyEq(bb2.Max.X-bb1.Max.X, delta) {
				t.Fatalf("bbox.Max.X shift = %v, want %v for %s", bb2.Max.X-bb1.Max.X, delta, tc.name)
			}
			// Подход сдвигает только по X — Y и Z не меняются.
			if !nearlyEq(bb2.Min.Y, bb1.Min.Y) || !nearlyEq(bb2.Max.Y, bb1.Max.Y) {
				t.Fatalf("Y bounds changed under approach shift for %s: %+v vs %+v", tc.name, bb1, bb2)
			}
			if !nearlyEq(bb2.Min.Z, bb1.Min.Z) || !nearlyEq(bb2.Max.Z, bb1.Max.Z) {
				t.Fatalf("Z bounds changed under approach shift for %s: %+v vs %+v", tc.name, bb1, bb2)
			}

			// Комната впритык к меньшему bbox: с a1 вариант влезает, с a2 — нет.
			// Именно по этому боксу ForRoomFit решает, предлагать ли вариант.
			fitW := engineering.Length(bb1.Max.X + 1)
			fitL := engineering.Length(bb1.Max.Y + 100)
			c1.RoomWidth = fitW
			c1.RoomLength = fitL
			c2.RoomWidth = fitW
			c2.RoomLength = fitL
			if got := buildRoomFit(c1, nil); len(got) != 0 {
				t.Fatalf("expected no room_fit for a1=%v, got %v", a1, got)
			}
			if got := buildRoomFit(c2, nil); len(got) == 0 {
				t.Fatalf("expected room_fit for a2=%v (bbox wider by %v mm, room fits a1 but not a2), got none", a2, delta)
			}
		})
	}
}
