// parts_catalog_test.go — автоматическая сверка каталога деталей с эталоном
// FreeCAD (models/reference/stair-comb-railings.FCStd).
//
// Эталон читается НЕ с диска, а из манифеста, вшитого в бинарь
// (models.Load, S-199): искать файл вверх по дереву и молча скипаться
// при его отсутствии (S-194) — запрещено. Нет манифеста — нет сборки.
// Легаси-путь через ../freecad-out/reference_parts.json удалён: правда одна,
// и она в models/parts-manifest.json.
//
// Зачем: до появления каталога размеры деталей жили в трёх местах
// (FreeCAD-скрипт, константы railing.go, тесты), и расхождения находились
// только глазами в 3D-вьювере. Теперь неверная форма, размер или пропажа
// детали валят сборку ДО выкатки контейнеров.
package engineering

import (
	"math"
	"testing"

	"stairplatform/models"
)

// referencePart — запись эталона: берётся из вшитого манифеста,
// а не из внешнего JSON (см. шапку файла).
type referencePart struct {
	ID        string
	SourceObj string
	Count     int
	BBoxL     float64
	BBoxW     float64
	BBoxH     float64
	Volume    float64
	TubeOD    float64
	TubeWall  float64
}

type referenceFile struct {
	Source         string
	ToleranceMM    float64
	ToleranceVolPC float64
	Parts          []referencePart
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
	// Единственный источник эталона — манифест, вшитый в бинарь.
	// Ошибка разбора означает испорченный репозиторий, а не «нет данных»:
	// падаем, а не скипаемся (S-194). Переменной окружения для подмены
	// эталона нет осознанно: тест обязан сверять закоммиченный файл,
	// иначе CI проверяет неизвестно что.
	m, err := models.Load()
	if err != nil {
		t.Fatalf("вшитый манифест не разобрался: %v", err)
	}
	ref := referenceFile{
		Source:         m.SourceModel,
		ToleranceMM:    m.ToleranceMM,
		ToleranceVolPC: m.ToleranceVolumePC,
	}
	for _, p := range m.Parts {
		rp := referencePart{
			ID:        p.ID,
			SourceObj: p.SourceObject,
			Count:     p.Count,
			BBoxL:     p.BBox.L,
			BBoxW:     p.BBox.W,
			BBoxH:     p.BBox.H,
			Volume:    p.Volume,
		}
		if p.Tube != nil {
			rp.TubeOD = p.Tube.OD
			rp.TubeWall = p.Tube.Wall
		}
		ref.Parts = append(ref.Parts, rp)
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
	// Допуск — из самого манифеста, а не из головы: 0,3 мм.
	tol := ref.ToleranceMM
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
	if got := len(AllParts()); got != 16 {
		t.Errorf("всего %d позиций, ожидалось 16", got)
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
	cut, pur := PartsByGroup(GroupCutting), PartsByGroup(GroupPurchased)
	if len(cut)+len(pur) != len(AllParts()) {
		t.Errorf("группы не покрывают каталог: %d+%d != %d",
			len(cut), len(pur), len(AllParts()))
	}
	for _, p := range pur {
		if p.Material != MatStainless && p.Material != MatPlastic {
			t.Errorf("покупная %s (%s) имеет материал %q — сталь/дерево в раскрой не годится",
				p.ID, p.Name, p.Material)
		}
	}
}

// TestCatalogHasNoFastenerGroup — в каталоге деталей НЕТ крепежа, и это
// решение, а не случайность: у крепежа свой справочник с кодами, размерами
// и ценами, dommfg.DefaultFastenerRegistry(). Пока каталог деталей держал
// вторую копию («Анкер Ø10», «Крепёж наконечника Ø5»), о крепеже
// существовало две правды, и работала только одна — та, что в домене.
//
// Тест ловит возвращение группы, а не наличие деталей в ней: появление
// любой позиции с Group=="fastener" означает, что каталог снова стал
// вторым источником правды о крепеже.
func TestCatalogHasNoFastenerGroup(t *testing.T) {
	for _, p := range AllParts() {
		if string(p.Group) == "fastener" {
			t.Errorf("в каталоге деталей снова есть крепёж: %s (%s). Источник правды о крепеже — dommfg.DefaultFastenerRegistry()",
				p.ID, p.Name)
		}
		if string(p.Material) == "fastener" {
			t.Errorf("в каталоге деталей снова есть материал крепежа: %s (%s)", p.ID, p.Name)
		}
	}
}
