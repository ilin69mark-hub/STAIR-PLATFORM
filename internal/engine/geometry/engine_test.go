package geometry

import (
	"math"
	"reflect"
	"testing"

	kerngeo "stairplatform/internal/geometry"
)

// nearlyEq сравнивает с относительной точностью 1e-6 (для больших величин,
// где абсолютный допуск недопустим из-за float64).
func nearlyEq(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func TestGenerateResult(t *testing.T) {
	res, err := Generate(testConfig(t))
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
	// 2 косоура + 15 проступей + 15 подступенков.
	if res.Measurement.SolidCount != 32 {
		t.Fatalf("solid count = %d, want 32", res.Measurement.SolidCount)
	}
	if len(res.Model.Solids()) != res.Measurement.SolidCount {
		t.Fatal("model solids and measurement disagaree")
	}
}

func TestGenerateMeasurement(t *testing.T) {
	res, err := Generate(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	// объём: 2 косоура + проступи + подступенки.
	want := 2.0*(567000*50) + 15.0*(270*800*40) + 15.0*(800*180*40)
	if !nearlyEq(res.Measurement.Volume, want) {
		t.Fatalf("volume = %v, want %v", res.Measurement.Volume, want)
	}
	// bounding box: минимум у левого косоура (с низом -50), максимум у
	// последней проступи на высоте H+st.
	bb := res.Measurement.BoundingBox
	if !nearlyEq(bb.Min.X, 0) || !nearlyEq(bb.Min.Y, 0) || !nearlyEq(bb.Min.Z, -50) {
		t.Fatalf("bbox min = %+v, want (0,0,-50)", bb.Min)
	}
	if !nearlyEq(bb.Max.X, 4050) || !nearlyEq(bb.Max.Y, 900) || !nearlyEq(bb.Max.Z, 2740) {
		t.Fatalf("bbox max = %+v, want (4050,900,2740)", bb.Max)
	}
	// площадь поверхности: 2 косоура (2 крышки по 567000 + периметр×50)
	// + 15 проступей + 15 подступенков.
	d := math.Sqrt(270*270 + 180*180)
	perimeter := 15.0*(270+180+d) + 100
	stringerArea := 2 * (2*567000 + 50*perimeter)
	treadArea := 15.0 * (2*270*800 + 2*270*40 + 2*800*40)
	riserArea := 15.0 * (2*40*800 + 2*40*180 + 2*800*180)
	if !nearlyEq(res.Measurement.SurfaceArea, stringerArea+treadArea+riserArea) {
		t.Fatalf("surface area = %v, want %v", res.Measurement.SurfaceArea, stringerArea+treadArea+riserArea)
	}
}

func TestGenerateErrors(t *testing.T) {
	cfg := testConfig(t)
	cfg.StepHeight = mustLength(t, 0)
	if _, err := Generate(cfg); err == nil {
		t.Fatal("invalid config must be rejected")
	}
	if _, err := Generate(nil); err == nil {
		t.Fatal("nil config must be rejected")
	}
}

func TestGenerateNoSteps(t *testing.T) {
	cfg := testConfig(t)
	cfg.StepThickness = mustLength(t, 0)
	res, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Measurement.SolidCount != 2 {
		t.Fatalf("solid count = %d, want 2", res.Measurement.SolidCount)
	}
	if len(res.Issues) != 0 {
		t.Fatalf("stringer-only model must validate clean, got %+v", res.Issues)
	}
	// объём двух косоуров.
	want := 2.0 * (567000 * 50)
	if !nearlyEq(res.Measurement.Volume, want) {
		t.Fatalf("volume = %v, want %v", res.Measurement.Volume, want)
	}
}

func TestGenerateDeterminism(t *testing.T) {
	a, err := Generate(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("generation must be deterministic")
	}
}

func TestGenerateMeshConsistency(t *testing.T) {
	res, err := Generate(testConfig(t))
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
