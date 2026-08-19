package geometry

import (
	"context"
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
	res, err := Generate(context.Background(), testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	// объём: 2 косоура (профиль-гребёнка) + проступи во всю ширину [0,w]
	// (глубина шага + толщина подступенка: 270+40) + подступенки во всю
	// ширину (материал w шириной).
	want := 2.0*(601194.16*50) + 15.0*(310*900*40) + 15.0*(900*140*40)
	if !nearlyEq(res.Measurement.Volume, want) {
		t.Fatalf("volume = %v, want %v", res.Measurement.Volume, want)
	}
	// bounding box: минимум у задней кромки нижней проступи (x=−st),
	// максимум у хвоста косоура (nb+t·h/L) и верха последней проступи (H).
	bb := res.Measurement.BoundingBox
	if !nearlyEq(bb.Min.X, -40) || !nearlyEq(bb.Min.Y, 0) || !nearlyEq(bb.Min.Z, 0) {
		t.Fatalf("bbox min = %+v, want (−40,0,0)", bb.Min)
	}
	if !nearlyEq(bb.Max.X, 4077.73501) || !nearlyEq(bb.Max.Y, 900) || !nearlyEq(bb.Max.Z, 2700) {
		t.Fatalf("bbox max = %+v, want (4077.7,900,2700)", bb.Max)
	}
	// площадь поверхности: 2 косоура (2 крышки по 601194.16 + периметр×50)
	// + 15 проступей во всю ширину + 15 подступенков (полотна 900 шириной).
	perimeter := 11630.521978
	stringerArea := 2 * (2*601194.16 + 50*perimeter)
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
	want := 2.0 * (605999.711094 * 50)
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
