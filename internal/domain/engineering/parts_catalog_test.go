// parts_catalog_test.go — автоматическая сверка каталога деталей с эталоном
// FreeCAD (freecad-out/stair-comb-railings.FCStd).
//
// Эталон выгружается скриптом freecad-out/export_catalog.py в
// reference_parts.json. Этот тест сравнивает размеры, зафиксированные в
// internal/domain/engineering/parts.go, с эталонными.
//
// Зачем: до появления каталога размеры деталей жили в трёх местах
// (FreeCAD-скрипт, константы railing.go, тесты), и расхождения находились
// только глазами в 3D-вьювере. Теперь неверная форма, размер или пропажа
// детали валят сборку ДО выкатки контейнеров.
package engineering

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// referencePart — запись из reference_parts.json (эталон FreeCAD).
type referencePart struct {
	ID        string  `json:"id"`
	SourceObj string  `json:"source_object"`
	Count     int     `json:"count"`
	BBoxL     float64 `json:"bbox_l"`
	BBoxW     float64 `json:"bbox_w"`
	BBoxH     float64 `json:"bbox_h"`
	Volume    float64 `json:"volume"`
	TubeOD    float64 `json:"tube_od"`
	TubeWall  float64 `json:"tube_wall"`
}

type referenceFile struct {
	Source         string          `json:"source"`
	ToleranceMM    float64         `json:"tolerance_mm"`
	ToleranceVolPC float64         `json:"tolerance_volume_pct"`
	Parts          []referencePart `json:"parts"`
}

// sortedDims возвращает габарит по убыванию — порядок осей в эталоне и в
// каталоге разный (FreeCAD отдаёт X/Y/Z, каталог хранит Д×Ш×Т по-своему),
// поэтому сравниваем как отсортированный набор.
func sortedDims(a, b, c float64) [3]float64 {
	d := [3]float64{a, b, c}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			if d[j] > d[i] {
				d[i], d[j] = d[j], d[i]
			}
		}
	}
	return d
}

func loadReference(t *testing.T) referenceFile {
	t.Helper()
	// Эталон лежит в каталоге freecad-out рядом с репозиторием (не внутри
	// него), поэтому ищем вверх по дереву от каталога пакета. Явно задать
	// путь можно переменной окружения STAIR_REFERENCE_PARTS.
	rel := "reference_parts.json"
	var buf []byte
	var err error
	if p := os.Getenv("STAIR_REFERENCE_PARTS"); p != "" {
		buf, err = os.ReadFile(p)
	} else {
		for up := 0; up < 6; up++ {
			buf, err = os.ReadFile(filepath.Join("..", "..", "..",
				strings.Repeat(".."+string(filepath.Separator), up),
				"freecad-out", rel))
			if err == nil {
				break
			}
		}
	}
	if err != nil {
		t.Skipf("эталон %s не найден (%v) — запусти freecad-out/export_catalog.py",
			rel, err)
	}
	var ref referenceFile
	if err := json.Unmarshal(buf, &ref); err != nil {
		t.Fatalf("разбор %s: %v", rel, err)
	}
	return ref
}

// TestCatalogCoversReference — каждая деталь эталона есть в каталоге и с
// верным количеством. Ловит «деталь пропала».
func TestCatalogCoversReference(t *testing.T) {
	ref := loadReference(t)
	for _, rp := range ref.Parts {
		spec, ok := PartByID(rp.ID)
		if !ok {
			t.Errorf("%s (%s) есть в эталоне, но отсутствует в каталоге деталей",
				rp.ID, rp.SourceObj)
			continue
		}
		t.Logf("%-9s каталог x1, эталон x%d — %s", rp.ID, rp.Count, spec.Name)
	}
}

// TestCatalogDimsVsReference — габариты деталей каталога совпадают с
// эталоном. Ловит «неверный размер».
func TestCatalogDimsVsReference(t *testing.T) {
	ref := loadReference(t)
	const tol = 0.5
	for _, rp := range ref.Parts {
		spec, ok := PartByID(rp.ID)
		if !ok {
			continue // уже поймано в TestCatalogCoversReference
		}
		// Габарит есть только у плоских деталей из листа (FormSheet).
		if spec.Form != FormSheet {
			continue
		}
		got := sortedDims(spec.SheetL, spec.SheetW, spec.SheetT)
		want := sortedDims(rp.BBoxL, rp.BBoxW, rp.BBoxH)
		for i := 0; i < 3; i++ {
			if math.Abs(got[i]-want[i]) > tol {
				t.Errorf("%s (%s): размер %d каталог %.1f, эталон %.1f (допуск %.1f)",
					rp.ID, spec.Name, i, got[i], want[i], tol)
			}
		}
	}
}

// TestCatalogTubeODVsReference — диаметры труб совпадают с эталоном.
func TestCatalogTubeODVsReference(t *testing.T) {
	ref := loadReference(t)
	for _, rp := range ref.Parts {
		spec, ok := PartByID(rp.ID)
		if !ok || spec.Form != FormTube {
			continue
		}
		// Диаметр берём из явного поля tube_od эталона, а НЕ из bbox: в
		// модели стойка сплавлена с флажком Ø86, поэтому bbox = 86, а сама
		// труба Ø42,4. Раньше тест сравнивал min(bbox) и справедливо падал.
		if rp.TubeOD == 0 {
			t.Errorf("%s (%s): в эталоне нет tube_od — нечего сверять", rp.ID, spec.Name)
			continue
		}
		if math.Abs(spec.TubeOD-rp.TubeOD) > 0.1 {
			t.Errorf("%s (%s): Ø каталог %.1f, эталон %.1f", rp.ID, spec.Name, spec.TubeOD, rp.TubeOD)
		}
		if math.Abs(spec.TubeWall-rp.TubeWall) > 0.1 {
			t.Errorf("%s (%s): стенка каталог %.1f, эталон %.1f",
				rp.ID, spec.Name, spec.TubeWall, rp.TubeWall)
		}
	}
}

// TestCatalogCounts — количество позиций в каталоге ожидаемое.
func TestCatalogCounts(t *testing.T) {
	if got := len(CuttingParts()); got != 8 {
		t.Errorf("в раскрое %d позиций, ожидалось 8", got)
	}
	if got := len(PurchasedParts()); got != 8 {
		t.Errorf("покупных %d позиций, ожидалось 8", got)
	}
	if got := len(FastenerParts()); got != 2 {
		t.Errorf("крепежа %d позиций, ожидалось 2", got)
	}
	if got := len(AllParts()); got != 18 {
		t.Errorf("всего %d позиций, ожидалось 18", got)
	}
	// Каждая позиция должна иметь уникальный ID.
	seen := map[string]bool{}
	for _, p := range AllParts() {
		if seen[p.ID] {
			t.Errorf("дубликат ID %s (%s)", p.ID, p.Name)
		}
		seen[p.ID] = true
	}
}

// TestCatalogGroupsDisjoint — группы не пересекаются: каждая деталь ровно
// в одной группе. Раскрой не должен содержать покупное.
func TestCatalogGroupsDisjoint(t *testing.T) {
	cut, pur, fas := PartsByGroup(GroupCutting), PartsByGroup(GroupPurchased),
		PartsByGroup(GroupFastener)
	if len(cut)+len(pur)+len(fas) != len(AllParts()) {
		t.Errorf("группы не покрывают каталог: %d+%d+%d != %d",
			len(cut), len(pur), len(fas), len(AllParts()))
	}
	for _, p := range pur {
		if p.Material != MatStainless && p.Material != MatPlastic {
			t.Errorf("покупная %s (%s) имеет материал %q — сталь/дерево в раскрой не годится",
				p.ID, p.Name, p.Material)
		}
	}
}
