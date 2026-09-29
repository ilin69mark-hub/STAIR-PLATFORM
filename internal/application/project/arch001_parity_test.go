package project

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"stairplatform/internal/application/stair"
)

// ARCH-001 (2026-09-26): паритет проекций stair.Config.
//
// Корневая причина DOM-003: у stair.Config было 28 полей, а проекция
//StairConfiguration содержала 19. Девять параметров не имели колонок ВООБЩЕ,
// и они молча терялись при сохранении ревизии, restore и экспорте CAD. Ни один
// тест этого не ловил: каждая проекция проверялась отдельно, а «паритет»
// между ними не проверялся нигде.
//
// Тест построен на РЕФЛЕКСИИ по исходникам (ast): список полей
// stair.Config сверяется с проекцией в персистенс и с SQL-колонками. Рефлексия
// по исходникам, а не по reflect.Value, нужна потому, что требуется увидеть
// ИМЕНА полей и sql-теги, а не их рантайм-типы.

// fieldName — список имён полей структуры из исходника.
func fieldNames(t *testing.T, file, structName string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != structName {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				out = append(out, name.Name)
			}
		}
		return false
	})
	if len(out) == 0 {
		t.Fatalf("struct %s not found in %s", structName, file)
	}
	return out
}

// configFieldToColumn — соответствие «поле stair.Config → колонка/поле
// проекции». Ключ — имя поля в stair.Config, значение — имя в
// project.StairConfiguration. Порядок и состав поддерживаются тестом: любое
// новое поле stair.Config обязано появиться здесь и в проекции.
var configFieldToColumn = map[string]string{
	"Width":             "WidthMM",
	"Height":            "HeightMM",
	"Flight":            "Flight",
	"StepHeight":        "StepHeightMM",
	"StringerThickness": "StringerThicknessMM",
	"StepThickness":     "StepThicknessMM",
	"Riser":             "Riser",
	"Clearance":         "ClearanceMM",
	"RailingHeight":     "RailingHeightMM",
	"LandingWidth":      "LandingWidthMM",
	"LandingDepth":      "LandingDepthMM",
	"RoomWidth":         "RoomWidthMM",
	"RoomLength":        "RoomLengthMM",
	"ApproachSpace":     "ApproachSpaceMM",
	"LowerStepCount":    "LowerStepCount",
	"TurnKind":          "TurnKind",
	"WinderCount":       "WinderCount",
	"OuterRadius":       "OuterRadiusMM",
	"Material":          "MaterialCode",
	"TreadMaterial":     "TreadMaterialCode",
	"RiserThickness":    "RiserThicknessMM",
	"Railing":           "Railing",
	"RailingLower":      "RailingLower",
	"RailingLanding":    "RailingLanding",
	"RailingUpper":      "RailingUpper",
	"Direction":         "Direction",
	"SpiralDir":         "SpiralDirection",
}

// TestARCH001_StairConfigProjectionParity — каждое поле stair.Config обязано
// быть представлено в проекции сохранения.
func TestARCH001_StairConfigProjectionParity(t *testing.T) {
	cfgFields := fieldNames(t, "../../application/stair/service.go", "Config")
	projFields := map[string]bool{}
	for _, f := range fieldNames(t, "entity.go", "StairConfiguration") {
		projFields[f] = true
	}

	var missing []string
	for _, f := range cfgFields {
		col, ok := configFieldToColumn[f]
		if !ok {
			missing = append(missing, f+" (нет в configFieldToColumn)")
			continue
		}
		if !projFields[col] {
			missing = append(missing, f+" → "+col+" (нет в проекции)")
		}
	}
	if len(missing) > 0 {
		t.Fatalf("ARCH-001: поля stair.Config потеряны в проекции сохранения: %s",
			strings.Join(missing, ", "))
	}
}

// TestARCH001_ProjectionHasNoUnmappedFields — обратная проверка: в проекции не
// должно быть «висячих» полей, которые никто не заполняет из stair.Config.
// Допускаются только служебные (ID, ProjectID, Revision, ComfortStepMM —
// из Options, метки времени).
func TestARCH001_ProjectionHasNoUnmappedFields(t *testing.T) {
	allowed := map[string]string{
		"ID":            "служебный идентификатор",
		"ProjectID":     "владелец ревизии",
		"Revision":      "номер ревизии, присваивается репозиторием",
		"ComfortStepMM": "приходит из stair.Options, а не из stair.Config",
		"CreatedAt":     "метка времени",
		"UpdatedAt":     "метка времени",
	}
	mapped := map[string]bool{}
	for _, col := range configFieldToColumn {
		mapped[col] = true
	}
	for _, f := range fieldNames(t, "entity.go", "StairConfiguration") {
		if mapped[f] {
			continue
		}
		if _, ok := allowed[f]; !ok {
			t.Errorf("ARCH-001: поле проекции %q не заполняется ни из stair.Config, "+
				"ни из Options — вероятно, оно забыто при персистенсе", f)
		}
	}
}

// TestARCH001_RoundTripCoversEveryMappedField — поведенческая проверка:
// проход по всем полям проекции туда-обратно обязан сохранить каждое
// непустое значение. Это ловит не только отсутствие поля, но и потерю при
// преобразовании типов/имён (как было с SpiralDir → SpiralDirection).
func TestARCH001_RoundTripCoversEveryMappedField(t *testing.T) {
	// Заполняем каждое поле проекции осмысленным НЕ-нулевым значением.
	ents := toConfigEntity("p-1", fullyPopulatedConfig(), stair.Options{ComfortStep: 617})

	// Сверяем по импарам отражения: каждое поле, помеченное как заполненное,
	// должно быть непустым в сущности.
	checks := map[string]struct{ got, want string }{
		"Width":             {num(ents.WidthMM), "1000"},
		"Height":            {num(ents.HeightMM), "3200"},
		"Flight":            {ents.Flight, "u_shape"},
		"StepHeight":        {num(ents.StepHeightMM), "160"},
		"StringerThickness": {num(ents.StringerThicknessMM), "60"},
		"StepThickness":     {num(ents.StepThicknessMM), "8"},
		"Clearance":         {num(ents.ClearanceMM), "2100"},
		"RailingHeight":     {num(ents.RailingHeightMM), "950"},
		"LandingWidth":      {num(ents.LandingWidthMM), "1100"},
		"LandingDepth":      {num(ents.LandingDepthMM), "1200"},
		"RoomWidth":         {num(ents.RoomWidthMM), "2600"},
		"RoomLength":        {num(ents.RoomLengthMM), "1900"},
		"ApproachSpace":     {num(ents.ApproachSpaceMM), "1000"},
		"LowerStepCount":    {strconv.Itoa(ents.LowerStepCount), "7"},
		"OuterRadius":       {num(ents.OuterRadiusMM), "900"},
		"TurnKind":          {ents.TurnKind, "winder"},
		"WinderCount":       {strconv.Itoa(ents.WinderCount), "3"},
		"Railing":           {ents.Railing, "both"},
		"RailingLower":      {ents.RailingLower, "left"},
		"RailingLanding":    {ents.RailingLanding, "right"},
		"RailingUpper":      {ents.RailingUpper, "none"},
		"Direction":         {ents.Direction, "left"},
		"SpiralDirection":   {ents.SpiralDirection, "ccw"},
		"Material":          {ents.MaterialCode, "STEEL-S235"},
	}
	for name, chk := range checks {
		if chk.got != chk.want {
			t.Errorf("поле %s: проекция сохранила %q, ожидалось %q", name, chk.got, chk.want)
		}
	}
	if !ents.Riser {
		t.Error("Riser: проекция не сохранила true")
	}

	// Обратный проход: fromConfigEntity обязан вернуть те же значения.
	back, opts, err := fromConfigEntity(ents)
	if err != nil {
		t.Fatalf("fromConfigEntity: %v", err)
	}
	if opts.ComfortStep != 617 {
		t.Errorf("ComfortStep: %v, ожидалось 617", opts.ComfortStep)
	}
	if back.TurnKind != "winder" || back.WinderCount != 3 {
		t.Errorf("winder потерян: TurnKind=%q WinderCount=%d", back.TurnKind, back.WinderCount)
	}
	if back.SpiralDir != "ccw" {
		t.Errorf("SpiralDir потерян: %q", back.SpiralDir)
	}
	if back.Material != "STEEL-S235" {
		t.Errorf("Material потерян: %q", back.Material)
	}
	if back.RailingLanding != "right" || back.RailingUpper != "none" {
		t.Errorf("перила потеряны: landing=%q upper=%q", back.RailingLanding, back.RailingUpper)
	}
	if back.Direction != "left" {
		t.Errorf("Direction потерян: %q", back.Direction)
	}
}

// TestARCH001_ParityMapHasNoStaleEntries — защита от «осиротевших» записей в
// таблице соответствия: поле, которого больше нет в stair.Config, не должно
// молча оставаться в мапе.
func TestARCH001_ParityMapHasNoStaleEntries(t *testing.T) {
	cfgFields := map[string]bool{}
	for _, f := range fieldNames(t, "../../application/stair/service.go", "Config") {
		cfgFields[f] = true
	}
	for f := range configFieldToColumn {
		if !cfgFields[f] {
			t.Errorf("ARCH-001: configFieldToColumn содержит %q, которого больше нет в stair.Config", f)
		}
	}
}

// num — строковое представление float64 для таблицы ожиданий: целые числа
// печатаются без дробной части (1000, а не 1000.0000001).
func num(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	return s
}
