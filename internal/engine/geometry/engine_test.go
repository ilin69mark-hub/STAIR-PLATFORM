package geometry

import (
	"context"
	"math"
	"reflect"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// nearlyEq сравнивает с относительной точностью 1e-6 (для больших величин,
// где абсолютный допуск недопустим из-за float64).
func nearlyEq(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func TestGenerateResult(t *testing.T) {
	res, err := Generate(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.Model == nil || res.Mesh == nil {
		t.Fatal("result must contain model and mesh")
	}
	// валидация модели не должна находить ошибок (нормальный марш).
	if len(res.Issues) != 0 {
		t.Fatalf("expected no validation issues, got %+v", res.Issues)
	}
	// 2 косоура + 15 проступей + 15 подступенков (полотно на ступень).
	if res.Measurement.SolidCount != 32 {
		t.Fatalf("solid count = %d, want 32", res.Measurement.SolidCount)
	}
	if len(res.Model.Solids()) != res.Measurement.SolidCount {
		t.Fatal("model solids and measurement disagaree")
	}
}

func TestGenerateMeasurement(t *testing.T) {
	cfg := testConfig(t)
	res, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	// объём: 2 косоура (профиль-гребёнка) + проступи во всю ширину [0,w]
	// (глубина шага + толщина подступенка: 270+40) + подступенки во всю
	// ширину (материал w шириной).
	want := 2.0*(527995.956797*50) + 15.0*(310*900*40) + 15.0*(900*140*40)
	if !nearlyEq(res.Measurement.Volume, want) {
		t.Fatalf("volume = %v, want %v", res.Measurement.Volume, want)
	}
	// bounding box: минимум у задней кромки нижней проступи (x=−st),
	// максимум у хвоста косоура (n·b, вертикальный срез) и верха последней проступи (H).
	// Прямой марш сдвигается от стены на свободное пространство (approach,
	// по умолчанию 1000 мм, если не задано), поэтому весь габарит по X
	// смещается на эту величину (EDR-0023).
	approach := cfg.ApproachSpace.Millimeters()
	if approach == 0 {
		approach = 1000
	}
	bb := res.Measurement.BoundingBox
	if !nearlyEq(bb.Min.X, -40+approach) || !nearlyEq(bb.Min.Y, 0) || !nearlyEq(bb.Min.Z, 0) {
		t.Fatalf("bbox min = %+v, want (%v,0,0)", bb.Min, -40+approach)
	}
	if !nearlyEq(bb.Max.X, 4050+approach) || !nearlyEq(bb.Max.Y, 900) || !nearlyEq(bb.Max.Z, 2700) {
		t.Fatalf("bbox max = %+v, want (%v,900,2700)", bb.Max, 4050+approach)
	}
	// площадь поверхности: 2 косоура (2 крышки по 527995.96 + периметр×50)
	// + 15 проступей во всю ширину + 15 подступенков (полотна 900 шириной).
	perimeter := 11594.389483
	stringerArea := 2 * (2*527995.956797 + 50*perimeter)
	treadArea := 15.0 * (2*900*310 + 2*900*40 + 2*310*40)
	riserArea := 15.0 * (2*140*900 + 2*40*900 + 2*140*40)
	if !nearlyEq(res.Measurement.SurfaceArea, stringerArea+treadArea+riserArea) {
		t.Fatalf("surface area = %v, want %v", res.Measurement.SurfaceArea, stringerArea+treadArea+riserArea)
	}
}

func TestGenerateErrors(t *testing.T) {
	cfg := testConfig(t)
	cfg.StepHeight = mustLength(t, 0)
	if _, err := Generate(context.Background(), cfg); err == nil {
		t.Fatal("invalid config must be rejected")
	}
	if _, err := Generate(context.Background(), nil); err == nil {
		t.Fatal("nil config must be rejected")
	}
}

func TestGenerateNoSteps(t *testing.T) {
	cfg := testConfig(t)
	cfg.StepThickness = mustLength(t, 0)
	res, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Measurement.SolidCount != 2 {
		t.Fatalf("solid count = %d, want 2", res.Measurement.SolidCount)
	}
	if len(res.Issues) != 0 {
		t.Fatalf("stringer-only model must validate clean, got %+v", res.Issues)
	}
	// объём двух косоуров (профиль при st=0: седло на уровне носика).
	want := 2.0 * (531692.107680 * 50)
	if !nearlyEq(res.Measurement.Volume, want) {
		t.Fatalf("volume = %v, want %v", res.Measurement.Volume, want)
	}
}

func TestGenerateDeterminism(t *testing.T) {
	a, err := Generate(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("generation must be deterministic")
	}
}

func TestGenerateMeshConsistency(t *testing.T) {
	res, err := Generate(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Mesh.Vertices) == 0 || len(res.Mesh.Triangles) == 0 {
		t.Fatal("preview mesh must not be empty")
	}
	for _, tr := range res.Mesh.Triangles {
		for _, v := range tr {
			if v < 0 || v >= len(res.Mesh.Vertices) {
				t.Fatalf("triangle %v references vertex %d out of range", tr, v)
			}
		}
	}
	// независимость производных величин от модели: модель не мутируется.
	if !reflect.DeepEqual(res.Model, kerngeo.NewCompound(res.Model.Solids()...)) {
		t.Fatal("generation must not mutate the model")
	}
}

func hasIssue(issues []kerngeo.ValidationIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

// TestGenerateStraightApproach — свободное пространство перед первой ступенью
// (EDR-0023): прямой марш сдвигается от стены на ApproachSpace, и fit-check
// требует, чтобы забег + ApproachSpace помещались в ширину помещения.
func TestGenerateStraightApproach(t *testing.T) {
	cfg := testConfig(t)
	cfg.ApproachSpace = mustLength(t, 1200)
	res, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Модель сдвинута на 1200 от стены (исходный min.X = −40).
	if !nearlyEq(res.Measurement.BoundingBox.Min.X, -40+1200) {
		t.Fatalf("bbox min.X = %v, want %v", res.Measurement.BoundingBox.Min.X, -40+1200)
	}
	// Помещение впритык к забегу без учёта зазора → room_fit.
	cfg.RoomWidth = mustLength(t, res.Measurement.BoundingBox.Max.X-1)
	cfg.RoomLength = mustLength(t, res.Measurement.BoundingBox.Max.Y+500)
	resTight, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue(resTight.Issues, "room_fit") {
		t.Fatal("expected room_fit when room is too narrow for run+approach")
	}
	// Помещение с запасом под зазор → без предупреждения.
	cfg.RoomWidth = mustLength(t, res.Measurement.BoundingBox.Max.X+100)
	resOk, err := Generate(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if hasIssue(resOk.Issues, "room_fit") {
		t.Fatal("unexpected room_fit when room fits run+approach")
	}
}

// TestGenerateApproachShiftAllKinds — свободное пространство перед первой
// ступенью (EDR-0023) сдвигает модель (и, значит, габаритный бокс) по оси X
// для ВСЕХ типов марша, а не только прямого. Проверяем, что при увеличении
// ApproachSpace на Δ габарит по X сдвигается ровно на Δ, а по Y/Z не меняется.
//
// Важно: при ApproachSpace == 0 движок использует значение по умолчанию 1000
// (см. Generate), поэтому для чистого сравнения берём два явных ненулевых
// значения (500 и 1500) — их разница и есть ожидаемый сдвиг.
func TestGenerateApproachShiftAllKinds(t *testing.T) {
	cases := []struct {
		name string
		make func(t *testing.T) *engineering.StairConfiguration
	}{
		{"l_shape", makeLShapeConfigForTest},
		{"u_shape", makeUShapeConfigForTest},
		{"spiral", makeSpiralConfigForTest},
	}
	const a1, a2 = 500.0, 1500.0
	delta := a2 - a1
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c1 := tc.make(t)
			c1.ApproachSpace = mustLength(t, a1)
			c2 := tc.make(t)
			c2.ApproachSpace = mustLength(t, a2)

			r1, err := Generate(context.Background(), c1)
			if err != nil {
				t.Fatalf("generate a1: %v", err)
			}
			r2, err := Generate(context.Background(), c2)
			if err != nil {
				t.Fatalf("generate a2: %v", err)
			}
			b1 := r1.Measurement.BoundingBox
			b2 := r2.Measurement.BoundingBox

			if !nearlyEq(b2.Min.X-b1.Min.X, delta) {
				t.Fatalf("bbox.Min.X shift = %v, want %v (a2-a1) for %s", b2.Min.X-b1.Min.X, delta, tc.name)
			}
			if !nearlyEq(b2.Max.X-b1.Max.X, delta) {
				t.Fatalf("bbox.Max.X shift = %v, want %v for %s", b2.Max.X-b1.Max.X, delta, tc.name)
			}
			// Подход сдвигает только по X — границы по Y и Z не должны меняться.
			if !nearlyEq(b2.Min.Y, b1.Min.Y) || !nearlyEq(b2.Max.Y, b1.Max.Y) {
				t.Fatalf("Y bounds changed under approach shift for %s: %+v vs %+v", tc.name, b1, b2)
			}
			if !nearlyEq(b2.Min.Z, b1.Min.Z) || !nearlyEq(b2.Max.Z, b1.Max.Z) {
				t.Fatalf("Z bounds changed under approach shift for %s: %+v vs %+v", tc.name, b1, b2)
			}
		})
	}
}

func makeLShapeConfigForTest(t *testing.T) *engineering.StairConfiguration {
	c := testConfig(t)
	c.Flight = engineering.FlightLShape
	c.LowerStepCount = 7
	// Для платформенного поворота ширина площадки ≥ ширины марша (cfg.Validate).
	c.LandingWidth = mustLength(t, 1000)
	return c
}

func makeUShapeConfigForTest(t *testing.T) *engineering.StairConfiguration {
	c := testConfig(t)
	c.Flight = engineering.FlightUShape
	c.LowerStepCount = 7
	c.LandingWidth = mustLength(t, 1000)
	return c
}

func makeSpiralConfigForTest(t *testing.T) *engineering.StairConfiguration {
	c := testConfig(t)
	c.Flight = engineering.FlightSpiral
	// Наружный радиус должен превышать ширину марша (радиус колонны = R − W > 0).
	c.OuterRadius = mustLength(t, 1300)
	return c
}
