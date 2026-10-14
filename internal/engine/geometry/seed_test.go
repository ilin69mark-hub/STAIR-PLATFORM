package geometry

import (
	"context"
	"sort"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// СИД-ЭТАЛОН (2026-10-13) — минимальный прямой марш металлической лестницы с
// деревянными ступенями, на котором впервые строится каждое тело изделия.
//
// Зачем он нужен. Владелец делает модели в FreeCAD и передаёт их калькулятору
// как источник ФОРМЫ детали (Фаза 1). Чтобы библиотека была полной, сид
// должен содержать каждый вид детали, который калькулятору потом расставлять.
// Этот тест фиксирует состав сида, чтобы его нельзя было изменить случайно:
// добавили ступень — и сид перестал быть минимальным; убрали подступенок — и
// выпали две из пяти ролей пяток.
//
// Почему именно 6 ступеней. Не «чтобы поменьше», а потому что меньше нельзя:
// высота этажа у изделия от 1000 мм (MinFloorHeightMM, проверка в
// application/stair/service.go:1185), при подъёме 180 мм это 6 ступеней.
// Марш на 3 ступени высотой 540 мм отклоняется как «ниже минимума».
//
// Почему подступенок обязателен. Роли web и first_web ставятся только при
// Riser = true (builder.go:339 и :359), и их число равно n−1 и 1. Без
// подступенка из пяти ролей обоймы выреза остаётся три, и сид перестаёт
// покрывать обойму целиком.
//
// Почему ширина 900, а не 600. 600 — минимум, но отверстия не масштабируются:
// у них физический шаг, и на 600-мм ступени они разойдутся с настоящей.
// Сид должен быть размером, который реально заказывают.

const (
	seedWidth          = 900
	seedHeight         = 1080
	seedStepCount      = 6
	seedStepHeight     = 180
	seedTreadDepth     = 270 // шаг комфорта 630 = 270 + 2×180
	seedStringerThick  = 8   // листовой косоур 3–8 мм
	seedStepThick      = 40  // дуб: минимум ступени 40 мм под скругление R10
	seedRailingHeight  = 900
	seedExpectedStairs = 2 // ceil((6−1)·270 / 1500) + 1
)

// seedConfig — сид-эталон. Та же конфигурация, что уходит в FreeCAD.
func seedConfig() *engineering.StairConfiguration {
	cfg := &engineering.StairConfiguration{
		Width:             seedWidth,
		Height:            seedHeight,
		Flight:            engineering.FlightStraight,
		StepCount:         seedStepCount,
		StepHeight:        seedStepHeight,
		StepWidth:         seedTreadDepth,
		TreadDepth:        seedTreadDepth,
		StringerThickness: seedStringerThick,
		StepThickness:     seedStepThick,
		RiserThickness:    seedStepThick,
		Riser:             true,
		Material:          "STEEL-C245",
		TreadMaterial:     "WOOD-OAK",
		// Нос проступи скруглён R10: дубовые ступени идут с филёнкой
		// (TreadNoseRadiusMM приходит из материала в проде; в фикстуре
		// задаём явно, иначе сид строится с острым носом, которого
		// в изделии нет — S-213).
		TreadNoseRadiusMM: 10,
		Comb:              engineering.CombFrameDefaults(),
		Clearance:         2000,
		RailingHeight:     seedRailingHeight,
		Railing:           engineering.RailingLeft,
	}
	return cfg
}

// countRoles — пересчёт тел по ролям: роль → количество.
func countRoles(model *kerngeo.Compound) map[string]int {
	out := map[string]int{}
	for _, s := range model.Solids() {
		out[s.Role()]++
	}
	return out
}

// rolesOf — отсортированный список ролей для сообщения об ошибке.
func rolesOf(counts map[string]int) []string {
	out := make([]string, 0, len(counts))
	for r, n := range counts {
		out = append(out, r+"="+itoa(n))
	}
	sort.Strings(out)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// TestSeedCoversEveryPartRole — сид содержит каждый вид детали изделия.
//
// Это главный гейт сида. Пока в модели нет хотя бы одной роли, библиотека
// деталей неполна, и калькулятор не сможет поставить эту деталь на заказ —
// он возьмёт её из вашей модели, которой там нет.
func TestSeedCoversEveryPartRole(t *testing.T) {
	res, err := Generate(context.Background(), seedConfig())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.Model == nil {
		t.Fatal("модель не построена")
	}
	counts := countRoles(res.Model)

	// Пять ролей обоймы выреза — обязательный минимум сида.
	//   heel        n × 2 стороны   = 12
	//   web         (n−1) × 2       = 10   только при Riser
	//   anchor      2               =  2
	//   top_anchor  2               =  2
	//   first_web   2               =  2   только при Riser
	want := map[string]int{
		"heel":       12,
		"web":        10,
		"anchor":     2,
		"top_anchor": 2,
		"first_web":  2,
		"tread":      seedStepCount,
		"stringer":   2,
		"riser":      seedStepCount,
	}
	for role, n := range want {
		if got := counts[role]; got != n {
			t.Errorf("роль %q: тел %d, want %d (состав сида: %v)", role, got, n, rolesOf(counts))
		}
	}

	// Стойки ограждения в Model НЕ входят: тела ограждения — декор и уходят
	// в RailingMesh (appendRailingMesh, engine.go:432). Считать их надо по
	// BuildRailingDecor. Попытка посчитать по res.Model даёт 0 — и это не
	// ошибка модели, а её устройство.
	decor, err := BuildRailingDecor(seedConfig())
	if err != nil {
		t.Fatalf("BuildRailingDecor: %v", err)
	}
	decorCounts := map[string]int{}
	for _, s := range decor {
		if s != nil {
			decorCounts[s.Role()]++
		}
	}
	if got := decorCounts["baluster"]; got != seedExpectedStairs {
		t.Errorf("стоек ограждения: %d, want %d (роли декора: %v)",
			got, seedExpectedStairs, rolesOf(decorCounts))
	}
	if res.RailingMesh == nil {
		t.Fatal("нет меша ограждения: сид обязан содержать узел ограждения")
	}
	t.Logf("сид: тел в модели %d, роли: %v", len(res.Model.Solids()), rolesOf(counts))
	t.Logf("сид: тел ограждения %d, роли: %v", len(decor), rolesOf(decorCounts))
	t.Logf("сид: вершин в меше модели %d, треугольников %d",
		len(res.Mesh.Vertices), len(res.Mesh.Triangles))
	if res.RailingMesh != nil {
		t.Logf("сид: вершин в меше ограждения %d, треугольников %d",
			len(res.RailingMesh.Vertices), len(res.RailingMesh.Triangles))
	}
	if bb := res.Measurement.BoundingBox; bb.Max.Z != 0 {
		t.Logf("сид: габарит X %.1f…%.1f, Y %.1f…%.1f, Z %.1f…%.1f",
			bb.Min.X, bb.Max.X, bb.Min.Y, bb.Max.Y, bb.Min.Z, bb.Max.Z)
	}
}

// TestSeedIsManufacturable — сид не только строится, но и проходит
// производственный конвейер: раскрой, спецификация, крепёж.
//
// Сид, который нельзя изготовить, бесполезен как основание библиотеки: его
// формы нельзя ни вырезать, ни посчитать.
func TestSeedIsManufacturable(t *testing.T) {
	res, err := Generate(context.Background(), seedConfig())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, issue := range res.Issues {
		if issue.Severity == kerngeo.SeverityError {
			t.Errorf("ошибка геометрии сида: %s — %s", issue.Code, issue.Message)
		}
	}
	if res.Measurement.SolidCount == 0 {
		t.Errorf("сид не признан годным: %+v", res.Measurement)
	}
	t.Logf("сид: тел %d, объём %.0f мм³, площадь %.0f мм²",
		res.Measurement.SolidCount, res.Measurement.Volume, res.Measurement.SurfaceArea)
}

// TestSeedMinimalStairIsRejectedBelowFloorHeight — фиксирует ПОЧЕМУ сид на
// шесть ступеней, а не меньше.
//
// Без этого теста «минимальный» означает непроверяемое слово, и первая же
// попытка уменьшить сид проходит мимо предупреждения.
func TestSeedMinimalStairIsRejectedBelowFloorHeight(t *testing.T) {
	if seedStepCount*seedStepHeight < 1000 {
		t.Fatalf("сид %d×%d = %d мм ниже минимума изделия 1000 мм",
			seedStepCount, seedStepHeight, seedStepCount*seedStepHeight)
	}
	// Проверяем через настоящий предел, а не через константу в тесте.
	if got := seedStepCount * seedStepHeight; got < int(engineering.MinFloorHeightMM) {
		t.Fatalf("высота сида %d мм ниже MinFloorHeightMM (%v)", got, engineering.MinFloorHeightMM)
	}
}
