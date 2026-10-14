package geometry

// library_test.go — гейты библиотеки шаблонов (S-202).
//
// Библиотека — реестр «каждая позиция каталога: exact или computed
// с причиной». Тесты держат три инварианта: покрытие полное (дыры
// запрещены), exact действительно строится из размеров каталога
// (труба — аналитический объём оболочки, коробка — L×W×T),
// exact-позиции присутствуют в манифесте (иначе шаблону не на что
// опереться в этапе 2).

import (
	"math"
	"testing"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
	"stairplatform/models"
)

// TestLibraryCoversCatalog — каждая позиция каталога ровно один раз:
// либо exact с рабочим Build, либо computed с причиной.
//
// Ловит «деталь появилась в производстве раньше, чем в библиотеке»
// и «computed без объяснения, почему не шаблон».
func TestLibraryCoversCatalog(t *testing.T) {
	lib, err := Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	all := engineering.AllParts()
	if len(lib) != len(all) {
		t.Fatalf("в библиотеке %d шаблонов, в каталоге %d позиций", len(lib), len(all))
	}
	seen := map[string]bool{}
	for _, tmpl := range lib {
		if seen[tmpl.PartID] {
			t.Errorf("дубликат шаблона %s", tmpl.PartID)
		}
		seen[tmpl.PartID] = true
		switch tmpl.Kind {
		case TemplateExact:
			if tmpl.Build == nil {
				t.Errorf("%s: exact без Build", tmpl.PartID)
			}
			if tmpl.Reason != "" {
				t.Errorf("%s: у exact не должно быть причины", tmpl.PartID)
			}
		case TemplateComputed:
			if tmpl.Reason == "" {
				t.Errorf("%s: computed без причины — молчаливая дыра", tmpl.PartID)
			}
			if tmpl.Build != nil {
				t.Errorf("%s: у computed не должно быть Build", tmpl.PartID)
			}
		default:
			t.Errorf("%s: неизвестный вид шаблона %q", tmpl.PartID, tmpl.Kind)
		}
	}
	for _, spec := range all {
		if !seen[spec.ID] {
			t.Errorf("%s (%s): нет в библиотеке", spec.ID, spec.Name)
		}
	}
}

// byID — шаблон из библиотеки по ID позиции.
func byID(t *testing.T, lib []Template, id string) Template {
	t.Helper()
	for _, tmpl := range lib {
		if tmpl.PartID == id {
			return tmpl
		}
	}
	t.Fatalf("шаблона %s нет в библиотеке", id)
	return Template{}
}

// TestLibraryExactTube — трубчатый шаблон строит оболочку с аналитическим
// объёмом: π/4·(OD²−(OD−2w)²)·Len. Формула в тесте написана независимо
// от tubeAlong (не вызывает его): ловит перепутанные радиус/диаметр,
// длину и забытую стенку (цельный цилиндр дал бы +630 %).
//
// Допуск — математический, а не подогнанный: оба кольца строятся тем же
// числом сегментов на тех же углах, поэтому объём тесселяции равен
// k·аналитика с k<1 (многоугольник внутри окружности). Значит объём
// обязан быть НЕ БОЛЬШЕ аналитики — верхняя граница строгая. Нижняя
// 0,95 оставлена с запасом на случай смены пресетов качества;
// замерено 0,989 при ProductionQuality.
func TestLibraryExactTube(t *testing.T) {
	lib, err := Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	tmpl := byID(t, lib, "POST-001")
	spec, _ := engineering.PartByID("POST-001")
	s, err := tmpl.Build(ProductionQuality)
	if err != nil {
		t.Fatalf("Build POST-001: %v", err)
	}
	area := math.Pi / 4 * (spec.TubeOD*spec.TubeOD -
		(spec.TubeOD-2*spec.TubeWall)*(spec.TubeOD-2*spec.TubeWall))
	want := area * spec.Len
	got, err := kerngeo.Volume(s)
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	ratio := got / want
	if ratio > 1.0+1e-9 {
		t.Errorf("POST-001: объём %.1f больше аналитики %.1f — тесселяция не может дать больше", got, want)
	}
	if ratio < 0.95 {
		t.Errorf("POST-001: объём %.1f, аналитика %.1f (k=%.3f) — потеряно больше 5 %%", got, want, ratio)
	}
	bb := kerngeo.SolidBoundingBox(s)
	if dz := bb.Max.Z - bb.Min.Z; math.Abs(dz-spec.Len) > 0.5 {
		t.Errorf("POST-001: длина %.1f, каталог %.1f", dz, spec.Len)
	}
	if s.Role() != "post" {
		t.Errorf("POST-001: роль %q, ожидалась post", s.Role())
	}
}

// TestLibraryExactBox — коробчатый шаблон: объём ровно L×W×T
// (плоские грани триангулируются точно), габариты — из каталога.
func TestLibraryExactBox(t *testing.T) {
	lib, err := Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	for _, id := range []string{"TRED-001", "RISR-001"} {
		tmpl := byID(t, lib, id)
		spec, _ := engineering.PartByID(id)
		s, err := tmpl.Build(ProductionQuality)
		if err != nil {
			t.Fatalf("Build %s: %v", id, err)
		}
		want := spec.SheetL * spec.SheetW * spec.SheetT
		got, err := kerngeo.Volume(s)
		if err != nil {
			t.Fatalf("Volume %s: %v", id, err)
		}
		if math.Abs(got-want)/want > 0.0001 {
			t.Errorf("%s: объём %.1f, L×W×T %.1f", id, got, want)
		}
		bb := kerngeo.SolidBoundingBox(s)
		dims := [][2]float64{
			{bb.Max.X - bb.Min.X, spec.SheetL},
			{bb.Max.Y - bb.Min.Y, spec.SheetW},
			{bb.Max.Z - bb.Min.Z, spec.SheetT},
		}
		for i, d := range dims {
			if math.Abs(d[0]-d[1]) > 0.01 {
				t.Errorf("%s: габарит %d %.2f, каталог %.2f", id, i, d[0], d[1])
			}
		}
	}
}

// TestLibraryExactIDsInManifest — у каждого exact-шаблона есть опора
// в манифесте. Шаблон без замеров эталона — это чертёж без проверки
// в этапе 2: запрещён заранее.
func TestLibraryExactIDsInManifest(t *testing.T) {
	lib, err := Library()
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	m, err := models.Load()
	if err != nil {
		t.Fatalf("манифест: %v", err)
	}
	for _, tmpl := range lib {
		if tmpl.Kind != TemplateExact {
			continue
		}
		if _, ok := m.ByID(tmpl.PartID); !ok {
			t.Errorf("%s: exact-шаблон без замеров в манифесте", tmpl.PartID)
		}
	}
}
