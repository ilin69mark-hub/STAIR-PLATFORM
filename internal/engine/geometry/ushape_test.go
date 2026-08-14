package geometry

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// ushapeConfig возвращает валидную конфигурацию П-образной лестницы
// (H=2700, n=15, h=180, b=270, W=900, n1=6, Wp=1000, T=50, st=40).
func ushapeConfig(t *testing.T) *engineering.StairConfiguration {
	t.Helper()
	cfg, err := engineering.NewStairConfiguration(
		mustLength(t, 900), mustLength(t, 2700), engineering.FlightUShape)
	if err != nil {
		t.Fatal(err)
	}
	cfg.StepCount = 15
	cfg.StepHeight = mustLength(t, 180)
	cfg.TreadDepth = mustLength(t, 270)
	cfg.StringerThickness = mustLength(t, 50)
	cfg.StepThickness = mustLength(t, 40)
	cfg.LowerStepCount = 6
	cfg.LandingWidth = mustLength(t, 1000)
	return cfg
}

// TestBuildUShapeFlightCount проверяет состав модели: нижний марш
// (2n1+2), площадка (1) и верхний марш (2n2+2) твёрдых тел.
func TestBuildUShapeFlightCount(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// n1=6: 14 тел; площадка: 1; n2=9: 20 тел. Итого 35.
	if len(model.Solids()) != 35 {
		t.Fatalf("solids = %d, want 35", len(model.Solids()))
	}
}

// TestBuildUShapeFlightLandingPosition проверяет положение площадки:
// верх на уровне H1 = n1·h = 1080, план [L1, L1+W]×[0,Wp] = [1620,2520]×[0,1000].
func TestBuildUShapeFlightLandingPosition(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !pointExists(model, kerngeo.NewPoint3(1620, 0, 1040)) {
		t.Fatal("landing bottom corner (1620,0,1040) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(2520, 1000, 1080)) {
		t.Fatal("landing top corner (2520,1000,1080) must exist")
	}
}

// TestBuildUShapeFlightLowerFlightPositions проверяет позиции нижнего
// марша (совпадают с прямым маршем): последняя проступь на z∈[1080,1120],
// первый подступенок на x∈[0,40].
func TestBuildUShapeFlightLowerFlightPositions(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !pointExists(model, kerngeo.NewPoint3(1620, 850, 1120)) {
		t.Fatal("lower last tread top corner (1620,850,1120) must exist")
	}
	if !pointExists(model, kerngeo.NewPoint3(0, 50, 0)) {
		t.Fatal("lower first riser corner (0,50,0) must exist")
	}
}

// TestBuildUShapeFlightUpperFlightPositions проверяет, что верхний марш
// развёрнут на 180°: возвращается вдоль −X параллельно нижнему маршу,
// ширина вдоль −Y. Поворот на 180° вокруг Z (EDR-0006 §4.8): локальный
// (x,y,z) → (2520−x, 1900−y, 1080+z).
func TestBuildUShapeFlightUpperFlightPositions(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// первая проступь верхнего марша: локальные x∈[0,270], y∈[50,850],
	// z∈[180,220] → глобальные x∈[2250,2520], y∈[1050,1850], z∈[1260,1300].
	if !pointExists(model, kerngeo.NewPoint3(2520, 1850, 1300)) {
		t.Fatal("upper first tread corner (2520,1850,1300) must exist")
	}
	// последняя проступь: локальные x∈[2160,2430], y∈[50,850], z∈[1620,1660]
	// → глобальные x∈[90,360], y∈[1050,1850], z∈[2700,2740].
	if !pointExists(model, kerngeo.NewPoint3(90, 1850, 2740)) {
		t.Fatal("upper last tread corner (90,1850,2740) must exist")
	}
	bb := kerngeo.BoundingBox(model)
	if !nearlyEqual(bb.Max.X, 2520) {
		t.Fatalf("model bbox max X = %v, want 2520 (landing edge)", bb.Max.X)
	}
	if !nearlyEqual(bb.Max.Y, 1900) {
		t.Fatalf("model bbox max Y = %v, want 1900 (1000+900)", bb.Max.Y)
	}
	if !nearlyEqual(bb.Max.Z, 2740) {
		t.Fatalf("model bbox max Z = %v, want 2740 (2700+40)", bb.Max.Z)
	}
}

// TestBuildUShapeFlightRoles проверяет семантические роли тел: для
// повёрнутого верхнего марша роли обязательны (без них decompose
// классифицировал бы stringer как riser).
func TestBuildUShapeFlightRoles(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, s := range model.Solids() {
		counts[s.Role()]++
	}
	if counts["stringer"] != 4 {
		t.Fatalf("stringer count = %d, want 4 (2 per flight)", counts["stringer"])
	}
	if counts["tread"] != 15 {
		t.Fatalf("tread count = %d, want 15", counts["tread"])
	}
	if counts["riser"] != 15 {
		t.Fatalf("riser count = %d, want 15", counts["riser"])
	}
	if counts["landing"] != 1 {
		t.Fatalf("landing count = %d, want 1", counts["landing"])
	}
}

func TestBuildUShapeFlightErrors(t *testing.T) {
	// не-U конфигурация отклоняется.
	if _, err := BuildUShapeFlight(testConfig(t)); err == nil {
		t.Fatal("straight config must be rejected")
	}
	// Wp < W отклоняется доменной валидацией (StepCount > 1).
	cfg := ushapeConfig(t)
	cfg.LandingWidth = mustLength(t, 500)
	if _, err := BuildUShapeFlight(cfg); err == nil {
		t.Fatal("landing width below stair width must be rejected")
	}
	// нулевой нижний марш.
	cfg = ushapeConfig(t)
	cfg.LowerStepCount = 0
	if _, err := BuildUShapeFlight(cfg); err == nil {
		t.Fatal("zero lower step count must be rejected")
	}
	if _, err := BuildUShapeFlight(nil); err == nil {
		t.Fatal("nil config must be rejected")
	}
}

func TestGenerateUShape(t *testing.T) {
	cfg := ushapeConfig(t)
	res, err := Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Measurement.SolidCount != 35 {
		t.Fatalf("solid count = %d, want 35", res.Measurement.SolidCount)
	}
	if len(res.Issues) != 0 {
		t.Fatalf("expected no validation issues, got %+v", res.Issues)
	}
	if res.Mesh == nil || len(res.Mesh.Triangles) == 0 {
		t.Fatal("preview mesh must be present")
	}
	// объём идентичен L-образному (вращение не меняет объём): нижний и
	// верхний марши + площадка. Площадка: Wp·W·st = 1000·900·40.
	// Профиль косоура (shoelace): n1=6 → 226800, n2=9 → 340200, толщина 50.
	want := 2.0*(226800*50) + 6.0*(270*800*40) + 6.0*(800*180*40) +
		2.0*(340200*50) + 9.0*(270*800*40) + 9.0*(800*180*40) +
		1000*900*40
	if !nearlyEq(res.Measurement.Volume, want) {
		t.Fatalf("volume = %v, want %v", res.Measurement.Volume, want)
	}
}

func TestGenerateUShapeDeterminism(t *testing.T) {
	a, err := Generate(ushapeConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(ushapeConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if a.Measurement != b.Measurement {
		t.Fatal("u-shape generation must be deterministic")
	}
	if len(a.Mesh.Vertices) != len(b.Mesh.Vertices) || len(a.Mesh.Triangles) != len(b.Mesh.Triangles) {
		t.Fatal("u-shape mesh must be deterministic")
	}
}

// TestUShapeBBoxMath — независимая проверка геометрии: полный bbox должен
// покрывать оба марша и площадку (с учётом heel косоура -50).
func TestUShapeBBoxMath(t *testing.T) {
	cfg := ushapeConfig(t)
	model, err := BuildUShapeFlight(cfg)
	if err != nil {
		t.Fatal(err)
	}
	bb := kerngeo.BoundingBox(model)
	if bb.Min.X < -1e-6 || bb.Min.Y < -1e-6 || bb.Min.Z > -49 {
		t.Fatalf("bbox min = %+v, want near (0,0,-50)", bb.Min)
	}
	if math.Abs(bb.Max.X-2520) > 1e-6 || math.Abs(bb.Max.Y-1900) > 1e-6 || math.Abs(bb.Max.Z-2740) > 1e-6 {
		t.Fatalf("bbox max = %+v, want (2520,1900,2740)", bb.Max)
	}
}
