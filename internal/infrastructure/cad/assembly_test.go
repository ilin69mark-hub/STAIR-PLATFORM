package cad

import (
	"bytes"
	"strings"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
	kerngeo "stairplatform/internal/geometry"
)

// СБОРОЧНЫЙ ЧЕРТЁЖ: гейты содержимого.
//
// Чертёж без номеров — это картинка, а не чертёж. Здесь проверяется, что в
// файле есть метки на местах деталей, что детали без места перечислены
// отдельно (а не скрыты), и что чертёж не разваливается от строк.

// assemblyFixture — меш из одного треугольника и две детали: одна с
// положением, одна покупная без него.
func assemblyFixture() (*kerngeo.Mesh, []dommfg.Part) {
	m := &kerngeo.Mesh{
		Vertices: []kerngeo.Point3{
			{X: 0, Y: 0, Z: 0}, {X: 100, Y: 0, Z: 0}, {X: 0, Y: 100, Z: 0},
		},
		Triangles: [][3]int{{0, 1, 2}},
	}
	parts := []dommfg.Part{{
		Number: "TRD-01", Kind: "tread", Material: "WOOD-OAK",
		Length: 900, Width: 290, Where: "ступень 1",
		Placement: dommfg.Placement{
			Origin: dommfg.Vec3{X: 0, Y: 0, Z: 0},
			U:      dommfg.Vec3{X: 1},
			V:      dommfg.Vec3{Y: 1},
			N:      dommfg.Vec3{Z: 1},
			Known:  true,
		},
	}, {
		Number: "PST-01", Kind: "post", Material: "STAINLESS-304",
		Length: 806, Width: 42, Purchased: true,
		// Placement.Known = false: покупная, места сервер не знает.
	}}
	return m, parts
}

// TestAssemblyLabelsPartsOnPlan — номер детали стоит на плане.
func TestAssemblyLabelsPartsOnPlan(t *testing.T) {
	m, parts := assemblyFixture()
	var buf bytes.Buffer
	if err := WriteAssembly(&buf, m, parts); err != nil {
		t.Fatalf("WriteAssembly: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, ">TRD-01<") {
		t.Errorf("номер TRD-01 не попал на план:\n%s", out)
	}
	if !strings.Contains(out, "ступень 1") {
		t.Error("место детали не подписано под номером")
	}
}

// TestAssemblyListsUnlabeledSeparately — детали без места перечислены
// отдельно, а не скрыты.
func TestAssemblyListsUnlabeledSeparately(t *testing.T) {
	m, parts := assemblyFixture()
	var buf bytes.Buffer
	if err := WriteAssembly(&buf, m, parts); err != nil {
		t.Fatalf("WriteAssembly: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "PST-01") {
		t.Error("покупная деталь PST-01 пропала из чертежа вовсе")
	}
	if !strings.Contains(out, "Без места на плане") {
		t.Error("нет пометки, что часть деталей не на плане: чертёж выглядит полным, хотя он неполный")
	}
}

// TestAssemblyHasEdges — рёбра меша на плане есть: чертёж без геометрии —
// это список, а не чертёж.
func TestAssemblyHasEdges(t *testing.T) {
	m, parts := assemblyFixture()
	var buf bytes.Buffer
	if err := WriteAssembly(&buf, m, parts); err != nil {
		t.Fatalf("WriteAssembly: %v", err)
	}
	if !strings.Contains(buf.String(), "<line ") {
		t.Error("на плане нет рёбер геометрии")
	}
}

// TestAssemblyLabelAtPlacementCenter — метка стоит в центре детали, а не в
// нуле.
//
// Центр TRD-01 900×290 в проекции — (450, −145) в SVG-координатах
// (половины габаритов, Y инвертирована). Метка не в нуле означает, что
// Placement реально используется, а не принят и проигнорирован.
func TestAssemblyLabelAtPlacementCenter(t *testing.T) {
	m, parts := assemblyFixture()
	var buf bytes.Buffer
	if err := WriteAssembly(&buf, m, parts); err != nil {
		t.Fatalf("WriteAssembly: %v", err)
	}
	if !strings.Contains(buf.String(), `x="450.000" y="-145.000"`) {
		t.Errorf("метка не в центре детали:\n%s", buf.String())
	}
}

// TestAssemblyRequiresMesh — без меша чертежа нет.
func TestAssemblyRequiresMesh(t *testing.T) {
	_, parts := assemblyFixture()
	var buf bytes.Buffer
	if err := WriteAssembly(&buf, nil, parts); err == nil {
		t.Fatal("чертёж без геометрии не отменён")
	}
}

// TestAssemblyEscapesText — строки в чертеже экранированы.
func TestAssemblyEscapesText(t *testing.T) {
	m, _ := assemblyFixture()
	parts := []dommfg.Part{{
		Number: "X<&>\"", Where: "ступень <1>",
		Placement: dommfg.Placement{
			U: dommfg.Vec3{X: 1}, V: dommfg.Vec3{Y: 1}, N: dommfg.Vec3{Z: 1}, Known: true,
		},
	}}
	var buf bytes.Buffer
	if err := WriteAssembly(&buf, m, parts); err != nil {
		t.Fatalf("WriteAssembly: %v", err)
	}
	if strings.Contains(buf.String(), `X<&>"`) {
		t.Error("строка не экранирована — SVG сломан")
	}
}
