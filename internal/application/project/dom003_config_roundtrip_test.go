package project

import (
	"context"
	"encoding/json"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
)

// TestDOM003_ConfigRoundTripPreservesNineParams — регрессия DOM-003.
//
// stair.Config содержит 28 полей, аStairConfiguration раньше содержал 19.
// Девять параметров (turn_kind, winder_count, railing, railing_lower,
// railing_landing, railing_upper, direction, spiral_direction, material_code)
// молча терялись при сохранении ревизии, при restore и при экспорте CAD:
// экспорт пересобирал конфигурацию из усечённой ревизии и возвращал другую
// лестницу, чем рассчитал пользователь.
func TestDOM003_ConfigRoundTripPreservesNineParams(t *testing.T) {
	cfg := testConfig()
	cfg.Flight = engineering.FlightLShape
	cfg.TurnKind = engineering.TurnWinder
	cfg.WinderCount = 4
	cfg.Railing = engineering.RailingBoth
	cfg.RailingLower = engineering.RailingLeft
	cfg.RailingLanding = engineering.RailingNone
	cfg.RailingUpper = engineering.RailingRight
	cfg.Direction = engineering.TurnRight
	cfg.SpiralDir = engineering.SpiralCCW
	cfg.Material = dommfg.MaterialCode("STEEL-S235")

	e := toConfigEntity("p-1", cfg, stair.Options{ComfortStep: 630})

	// Слой persistence хранит все девять полей.
	if e.TurnKind != "winder" {
		t.Errorf("TurnKind: got %q, want %q", e.TurnKind, "winder")
	}
	if e.WinderCount != 4 {
		t.Errorf("WinderCount: got %d, want 4", e.WinderCount)
	}
	if e.Railing != "both" || e.RailingLower != "left" ||
		e.RailingLanding != "none" || e.RailingUpper != "right" {
		t.Errorf("railing fields: got %q/%q/%q/%q", e.Railing, e.RailingLower, e.RailingLanding, e.RailingUpper)
	}
	if e.Direction != "right" {
		t.Errorf("Direction: got %q, want %q", e.Direction, "right")
	}
	if e.SpiralDirection != "ccw" {
		t.Errorf("SpiralDirection: got %q, want %q", e.SpiralDirection, "ccw")
	}
	if e.MaterialCode == "" {
		t.Error("MaterialCode must not be empty after round trip")
	}

	got, opts, err := fromConfigEntity(e)
	if err != nil {
		t.Fatalf("fromConfigEntity: %v", err)
	}
	if got.TurnKind != cfg.TurnKind {
		t.Errorf("TurnKind: got %q, want %q", got.TurnKind, cfg.TurnKind)
	}
	if got.WinderCount != cfg.WinderCount {
		t.Errorf("WinderCount: got %d, want %d", got.WinderCount, cfg.WinderCount)
	}
	if got.Railing != cfg.Railing || got.RailingLower != cfg.RailingLower ||
		got.RailingLanding != cfg.RailingLanding || got.RailingUpper != cfg.RailingUpper {
		t.Errorf("railings lost: %q/%q/%q/%q", got.Railing, got.RailingLower, got.RailingLanding, got.RailingUpper)
	}
	if got.Direction != cfg.Direction {
		t.Errorf("Direction: got %q, want %q", got.Direction, cfg.Direction)
	}
	if got.SpiralDir != cfg.SpiralDir {
		t.Errorf("SpiralDir: got %q, want %q", got.SpiralDir, cfg.SpiralDir)
	}
	if got.Material != cfg.Material {
		t.Errorf("Material: got %q, want %q", got.Material, cfg.Material)
	}
	if opts.ComfortStep != 630 {
		t.Errorf("ComfortStep: got %v, want 630", opts.ComfortStep)
	}
}

// TestDOM003_SpiralDirectionSurvivesRestore — регрессия DOM-003.
//
// SpiralDirection("") по DefaultRailing() даёт RailingLeft, то есть у КАЖДОЙ
// сохранённой спирали перила оказывались с левой стороны независимо от заданных.
// Восстановление ревизии обязано возвращать закрутку пользователя.
func TestDOM003_SpiralDirectionSurvivesRestore(t *testing.T) {
	cfg := testConfig()
	cfg.Flight = engineering.FlightSpiral
	cfg.Railing = engineering.RailingNone
	cfg.SpiralDir = engineering.SpiralCW

	e := toConfigEntity("p-1", cfg, stair.Options{})
	if e.SpiralDirection != "cw" {
		t.Fatalf("SpiralDirection not persisted: %q", e.SpiralDirection)
	}
	got, _, err := fromConfigEntity(e)
	if err != nil {
		t.Fatalf("fromConfigEntity: %v", err)
	}
	if got.SpiralDir != engineering.SpiralCW {
		t.Fatalf("spiral direction lost on restore: got %q, want cw", got.SpiralDir)
	}
	if got.SpiralDir.DefaultRailing() != engineering.RailingRight {
		t.Errorf("DefaultRailing after restore: got %q, want right for cw spiral", got.SpiralDir.DefaultRailing())
	}
}

// TestDOM003_EmptyOptionalFieldsRoundTripAsUnset — исторические строки после
// миграции 000030 имеют NULL в девяти новых колонках, что читается как "" и
// должно трактоваться как «не задано» (дефолт), а не как ошибка.
func TestDOM003_EmptyOptionalFieldsRoundTripAsUnset(t *testing.T) {
	cfg := testConfig()
	cfg.TurnKind = ""
	cfg.WinderCount = 0
	cfg.Railing = ""
	cfg.RailingLower = ""
	cfg.RailingLanding = ""
	cfg.RailingUpper = ""
	cfg.Direction = ""
	cfg.SpiralDir = ""
	cfg.Material = ""

	e := toConfigEntity("p-1", cfg, stair.Options{})
	if e.TurnKind != "" || e.Railing != "" || e.Direction != "" ||
		e.SpiralDirection != "" || e.MaterialCode != "" {
		t.Fatalf("empty values must be stored as empty (written as NULL), got %+v", e)
	}
	got, _, err := fromConfigEntity(e)
	if err != nil {
		t.Fatalf("fromConfigEntity must accept NULL/empty optional fields: %v", err)
	}
	if got.TurnKind != "" || got.Direction != "" || got.SpiralDir != "" {
		t.Errorf("unset fields must stay unset, got %+v", got)
	}
}

// TestDOM003_ExportCADWithRailingsReturnsRailingMesh — регрессия DOM-003.
//
// Экспорт CAD возвращал только res.Mesh, а перила строятся отдельным телом
// (RailingMesh) и в основной меш не входят: в DXF/STL/SVG перил не было вообще.
func TestDOM003_ExportCADWithRailingsReturnsRailingMesh(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	// Перила должны быть ЗАДАНЫ: для пустого Railing геометрия строит пустую
	// сетку перил (это корректно — «не задано» = «без перил»).
	withRailings := testConfig()
	withRailings.Railing = engineering.RailingBoth
	p := mustProjectAndConfigWith(t, svc, withRailings)

	mesh, railings, err := svc.ExportCADWithRailings(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("ExportCADWithRailings: %v", err)
	}
	if mesh == nil {
		t.Fatal("stair mesh must not be nil")
	}
	if railings == nil {
		t.Fatal("DOM-003: railing mesh must not be nil (nil == «нет перил» неотличимо от ошибки)")
	}
	if len(railings.Triangles) == 0 {
		t.Fatal("DOM-003: railing mesh is empty although default config has railings")
	}
	// Перила не должны быть частью основного меша — иначе Merge() даст
	// дублирование геометрии.
	var stairTris int
	for _, pr := range mesh.PartRanges {
		if pr.Role == "railing" {
			stairTris += pr.End - pr.Start
		}
	}
	if stairTris != 0 {
		t.Errorf("DOM-003: stair mesh must not contain railing parts, found %d railing triangles", stairTris)
	}
}

// TestDOM003_ExportCADLegacyMethodStillWorks — прежняя сигнатура ExportCAD
// сохранена (её использует порт ProjectService), она возвращает только сетку
// лестницы.
func TestDOM003_ExportCADLegacyMethodStillWorks(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, stair.NewService(), DefaultRules())
	p := mustProjectAndConfig(t, svc)

	mesh, err := svc.ExportCAD(context.Background(), testTenant, testOwner, p.ID)
	if err != nil {
		t.Fatalf("ExportCAD: %v", err)
	}
	if mesh == nil || len(mesh.Triangles) == 0 {
		t.Fatal("ExportCAD must keep returning the stair mesh")
	}
}

// mustProjectAndConfig создаёт проект и сохраняет в нём рассчитанную
// конфигурацию — предусловие для экспорта CAD.
func mustProjectAndConfig(t *testing.T, svc *Service) *Project {
	t.Helper()
	return mustProjectAndConfigWith(t, svc, testConfig())
}

func mustProjectAndConfigWith(t *testing.T, svc *Service, cfg stair.Config) *Project {
	t.Helper()
	p, err := svc.CreateProject(context.Background(), testTenant, testOwner, "DOM-003", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := svc.Calculate(context.Background(), testTenant, testOwner, p.ID, cfg, stair.Options{}); err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	return p
}

// fullyPopulatedConfig — конфигурация, в которой задано КАЖДОЕ поле
// stair.Config непустым осмысленным значением. Используется тестом паритета
// проекций (ARCH-001): если новое поле забудут при персистенсе, оно останется
// пустым и тест это покажет.
func fullyPopulatedConfig() stair.Config {
	return stair.Config{
		Width:             engineering.Length(1000),
		Height:            engineering.Length(3200),
		Flight:            engineering.FlightUShape,
		StepHeight:        engineering.Length(160),
		StringerThickness: engineering.Length(60),
		StepThickness:     engineering.Length(8),
		Riser:             true,
		Clearance:         engineering.Length(2100),
		RailingHeight:     engineering.Length(950),
		LandingWidth:      engineering.Length(1100),
		LandingDepth:      engineering.Length(1200),
		RoomWidth:         engineering.Length(2600),
		RoomLength:        engineering.Length(1900),
		ApproachSpace:     engineering.Length(1000),
		LowerStepCount:    7,
		TurnKind:          engineering.TurnWinder,
		WinderCount:       3,
		OuterRadius:       engineering.Length(900),
		Material:          dommfg.MaterialCode("STEEL-S235"),
		Railing:           engineering.RailingBoth,
		RailingLower:      engineering.RailingLeft,
		RailingLanding:    engineering.RailingRight,
		RailingUpper:      engineering.RailingNone,
		Direction:         engineering.TurnLeft,
		SpiralDir:         engineering.SpiralCCW,
	}
}

// TestDOM005_SnapshotCarriesFlightWidth — DOM-005: ширина марша обязана быть в
// снапшоте.
//
// Раньше её не было ни в stair.Result, ни в Snapshot, поэтому админские
// чертежи (план/профиль) рисовались при зашитой ширине 900 мм, тогда как
// объём, масса, BOM и цена считались для фактической. Витрина при этом
// рисовала план по реальной width_mm — один расчёт выглядел двумя разными
// лестницами.
func TestDOM005_SnapshotCarriesFlightWidth(t *testing.T) {
	res, err := stair.NewService().Calculate(context.Background(), wideConfig(), stair.Options{})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if res.Width.Millimeters() != wideConfig().Width.Millimeters() {
		t.Fatalf("Result.Width = %v, want %v", res.Width, wideConfig().Width)
	}

	snap := NewSnapshot("p-1", res)
	if snap.Width != wideConfig().Width.Millimeters() {
		t.Errorf("Snapshot.width = %v, want %v (2D-схемы рисуются по этому полю)",
			snap.Width, wideConfig().Width)
	}

	// Поле обязано сериализоваться: схемы читают снапшот из JSON.
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	w, ok := doc["width"].(float64)
	if !ok {
		t.Fatalf(`snapshot JSON must contain "width", got keys %v`, keysOf(doc))
	}
	if w != wideConfig().Width.Millimeters() {
		t.Errorf(`snapshot JSON width = %v, want %v`, w, wideConfig().Width.Millimeters())
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// wideConfig — конфигурация с НЕ дефолтной шириной (1200 мм), чтобы тест
// падал, если ширина снова подменяется на 900.
func wideConfig() stair.Config {
	cfg := testConfig()
	cfg.Width = engineering.Length(1200)
	return cfg
}
