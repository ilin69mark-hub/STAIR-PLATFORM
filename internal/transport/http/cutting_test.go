package http

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stairplatform/internal/application/project"
	dommfg "stairplatform/internal/domain/manufacturing"
)

// ДОКУМЕНТ РЕЗА: гейты маршрута.
//
// Прежний экспорт /export/cad отдавал DXF из 3DFACE меша — дамп картинки,
// которым нельзя резать. Новый /export/cutting отдаёт контуры на плоскости
// листа с отверстиями и номерами. Здесь проверяется, что маршрут отдаёт
// именно ЕГО, а не старый файл, и что ошибки называются правильно.

// cuttingTestPackage — пакет с одной деталью 260×50 и круглым отверстием Ø10.
//
// Контур и отверстие берутся готовыми: это тест МАРШРУТА, а не сборки
// контура из геометрии (она проверена в cad и manufacturing).
func cuttingTestPackage() *dommfg.ManufacturingPackage {
	// 24 точки: прогиб 0,0428 мм — в пределах CircleFitToleranceMM, и
	// отверстие распознаётся как окружность. 12 точек дали бы прогиб
	// 0,17 мм, и CIRCLE в файл не попал бы (см. circle_test.go).
	hole := make([]dommfg.Point2D, 24)
	for i := range hole {
		a := 2 * math.Pi * float64(i) / float64(len(hole))
		hole[i] = dommfg.Point2D{X: 130 + 5*math.Cos(a), Y: 25 + 5*math.Sin(a)}
	}
	item := dommfg.CutItem{
		PartNumber:   "HLT-01",
		MaterialCode: "STEEL-S235",
		Length:       260,
		Width:        50,
		Quantity:     1,
		Profile: []dommfg.Point2D{
			{X: 0, Y: 0}, {X: 260, Y: 0}, {X: 260, Y: 50}, {X: 0, Y: 50},
		},
		Holes: [][]dommfg.Point2D{hole},
	}
	// Наклейка — на каждую деталь поименно: у каждой свой номер, поэтому
	// ведомость строится по Parts, а не по CutList (там группы с
	// количеством, и номера всех экземпляров, кроме первого, потеряны).
	return &dommfg.ManufacturingPackage{
		Parts: []dommfg.Part{{
			Number: "HLT-01", Kind: "heel", Material: "STEEL-S235",
			Thickness: 4, Length: 260, Width: 50, Where: "ступень 1, сторона А",
			Placement: dommfg.Placement{
				Origin: dommfg.Vec3{X: 100, Y: 200, Z: 0},
				U:      dommfg.Vec3{X: 1},
				V:      dommfg.Vec3{Y: 1},
				N:      dommfg.Vec3{Z: 1},
				Known:  true,
			},
		}},
		CutList: dommfg.CutList{Items: []dommfg.CutItem{item}},
		Nesting: &dommfg.NestingResult{Sheets: []dommfg.SheetLayout{{
			MaterialCode: "STEEL-S235",
			Length:       3000,
			Width:        1500,
			Placed: []dommfg.PlacedPart{{
				PartNumber: "HLT-01", Length: 260, Width: 50, X: 10, Y: 20,
			}},
		}}},
	}
}

// TestExportCutting — GET отдаёт документ реза файлом DXF.
//
// Проверяется не «ответ 200», а СОДЕРЖИМОЕ: в файле должен быть контур
// детали (LWPOLYLINE), настоящая окружность отверстия (CIRCLE) и номер
// детали. Без этих трёх сущностей файл — не документ реза.
func TestExportCutting(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.cuttingPkg = cuttingTestPackage()

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export/cutting", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/dxf" {
		t.Errorf("Content-Type = %q, want application/dxf", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "cutting.dxf") {
		t.Errorf("Content-Disposition = %q, want cutting.dxf in it", cd)
	}
	body := rec.Body.String()
	// Внешний контур детали.
	if !strings.Contains(body, "\nLWPOLYLINE\n") {
		t.Error("в файле нет контура детали — резать не по чему")
	}
	// Отверстие — настоящей окружностью, а не многоугольником.
	if !strings.Contains(body, "\nCIRCLE\n") {
		t.Error("в файле нет CIRCLE — отверстия ломаной рисовать нельзя требовать")
	}
	// Треугольников меша быть не должно: это документ реза, а не дамп картинки.
	if strings.Contains(body, "3DFACE") {
		t.Error("в документе реза 3DFACE — это дамп меша, резать им нельзя")
	}
	// Номер детали для оператора.
	if !strings.Contains(body, "HLT-01") {
		t.Error("в файле нет номера детали HLT-01")
	}
}

// TestExportLabels — GET отдаёт ведомость наклеек в CSV: строка на деталь
// с номером и местом.
func TestExportLabels(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.cuttingPkg = cuttingTestPackage()

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export/labels", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/csv; charset=utf-8", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "HLT-01") {
		t.Error("в ведомости нет номера HLT-01")
	}
	// Заголовок зафиксирован: принтер этикеток настраивается на колонки.
	if !strings.Contains(body, "номер;куда;деталь;") {
		t.Errorf("заголовок ведомости изменился:\n%s", body)
	}
}

// TestExportCuttingNotFound — без конфигурации проекта отдаётся 404, а не
// пустой файл.
//
// Пустой файл опаснее ошибки: оператор сохранит его как чертёж для заказа,
// а при открытии увидит чистый лист — заказ останется без чертежа, и это
// обнаружится на станке.
func TestExportCuttingNotFound(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	// cuttingPkg не задан — пакета нет.

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export/cutting", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestExportCuttingUnknownProject — несуществующий проект даёт 404.
func TestExportCuttingUnknownProject(t *testing.T) {
	svc := newFakeProjectService()
	req := authedRequest(http.MethodGet, "/api/v1/projects/nope/export/cutting", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestExportAssembly — GET отдаёт сборочный чертёж: вид сверху с номерами
// деталей на их местах.
func TestExportAssembly(t *testing.T) {
	svc := newFakeProjectService()
	svc.projects["p-1"] = &project.Project{ID: "p-1", Name: "А", Status: "draft"}
	svc.cuttingPkg = cuttingTestPackage()

	req := authedRequest(http.MethodGet, "/api/v1/projects/p-1/export/assembly", "")
	rec := httptest.NewRecorder()
	testRouterWithProjects(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
	// Номер детали из пакета стоит на плане.
	if !strings.Contains(rec.Body.String(), ">HLT-01<") {
		t.Error("на плане нет номера HLT-01")
	}
}
