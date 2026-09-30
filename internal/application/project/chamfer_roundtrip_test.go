package project

import (
	"context"
	"testing"

	"stairplatform/internal/application/stair"
	kerngeo "stairplatform/internal/geometry"
)

// Скругление носа ступени — производная величина от материала ступеней, и
// поэтому в базе её нет: она пересчитывается на каждом расчёте. Это плюс (не
// нужна миграция и нечему разойтись в базе) и риск (любой путь, который строит
// геометрию в обход Calculate, потеряет фаску). Таких путей два: экспорт CAD и
// восстановление проекта — оба идут через Calculate, и оба проверяются здесь.
//
// Проверяем не «сколько там вершин вообще», а ровно то, что доказывает
// скругление: число треугольников в первой проступи. Прямой угол даёт
// 12 треугольников (4 точки сечения), скруглённый нос — 56 (15 точек:
// 4 угла + 2 точки касания + 9 сегментов дуги).

func woodTreads() stair.Config {
	cfg := testConfig()
	cfg.Material = "STEEL-S235"
	cfg.TreadMaterial = "WOOD-OAK"
	cfg.Riser = true
	return cfg
}

func steelTreads() stair.Config {
	cfg := testConfig()
	cfg.Material = "STEEL-S235"
	cfg.TreadMaterial = "STEEL-S235"
	cfg.Riser = true
	return cfg
}

// trianglesInFirstPartOfRole считает треугольники первой детали с указанной
// ролью: у скруглённой проступи их больше, чем у прямой.
func trianglesInFirstPartOfRole(t *testing.T, mesh *kerngeo.Mesh, role string) int {
	t.Helper()
	if mesh == nil {
		return 0
	}
	for _, pr := range mesh.PartRanges {
		if pr.Role == role {
			return pr.End - pr.Start
		}
	}
	return 0
}

func TestChamferSurvivesProjectRoundTripAndCADExport(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p := mustProjectAndConfigWith(t, svc, woodTreads())

	mesh, _, err := svc.ExportCADWithRailings(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("ExportCADWithRailings: %v", err)
	}
	if mesh == nil || len(mesh.PartRanges) == 0 {
		t.Fatal("exported mesh must carry part ranges")
	}
	woodTreadTris := trianglesInFirstPartOfRole(t, mesh, "tread")
	if woodTreadTris == 0 {
		t.Fatal("exported mesh has no tread part")
	}
	// Прямой угол даёт 12 треугольников на проступь; скругление — больше.
	if woodTreadTris <= 12 {
		t.Fatalf("tread has %d triangles — the chamfer is missing from the export", woodTreadTris)
	}
	t.Logf("проступь с фаской: %d треугольников (прямой угол дал бы 12)", woodTreadTris)
}

func TestSteelTreadsExportHasNoChamfer(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p := mustProjectAndConfigWith(t, svc, steelTreads())

	mesh, _, err := svc.ExportCADWithRailings(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("ExportCADWithRailings: %v", err)
	}
	// Стальной проступ по-прежнему прямоугольный: 12 треугольников.
	if got := trianglesInFirstPartOfRole(t, mesh, "tread"); got != 12 {
		t.Fatalf("steel tread has %d triangles, want 12 (no chamfer on metal)", got)
	}
}

func TestChamferRadiusNotPersistedButRecomputed(t *testing.T) {
	// Гарантия «радиус не хранится» структурная: в сохранённой ревизии
	// (project.StairConfiguration) такого поля просто нет, поэтому оно не может
	// разойтись с материалом. Проверяем то, что из этого следует: материал
	// ступеней переживает круг сохранения, и восстановленный расчёт получает
	// ту же фаску.
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p := mustProjectAndConfigWith(t, svc, woodTreads())

	sc, err := repo.GetLatestConfiguration(context.Background(), testTenant, p.ID)
	if err != nil {
		t.Fatalf("GetLatestConfiguration: %v", err)
	}
	if sc == nil {
		t.Fatal("configuration must be saved")
	}
	// Материал ступеней — единственное, из чего выводится радиус, значит он
	// обязан пережить сохранение.
	if sc.TreadMaterialCode != "WOOD-OAK" {
		t.Fatalf("tread material lost on save: %q", sc.TreadMaterialCode)
	}

	fromEntity, _, err := fromConfigEntity(sc)
	if err != nil {
		t.Fatalf("fromConfigEntity: %v", err)
	}
	res, err := svc.calc.Calculate(context.Background(), fromEntity, stair.Options{})
	if err != nil {
		t.Fatalf("Calculate after restore: %v", err)
	}
	got := trianglesInFirstPartOfRole(t, res.Mesh, "tread")
	if got == 0 || got <= 12 {
		t.Fatalf("restored project lost the chamfer: %d triangles per tread", got)
	}
	t.Logf("после восстановления: проступь %d треугольников — фаска на месте", got)
}
