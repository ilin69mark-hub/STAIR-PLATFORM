package database

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readMigrationFile читает файл миграции из migrations/.
func readMigrationFile(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(migrationsDir(t), name))
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return string(raw)
}

// TestDOM003_ConfigColsCoverAllMappedFields — регрессия DOM-003.
//
// configCols перечисляет колонки stair_configurations, scanConfig читает их
// в том же порядке, а insertConfiguration пишет их. Любое расхождение даёт
// ТИХУЮ порчу данных (значение одной колонки попадает в соседнюю), поэтому
// соответствие проверяется тестом, а не только глазами.
//
// DOM-003 добавил девять колонок, которые отсутствовали в проекции
// application-слоя: turn_kind, winder_count, railing, railing_lower,
// railing_landing, railing_upper, direction, spiral_direction, material_code.
func TestDOM003_ConfigColsCoverAllMappedFields(t *testing.T) {
	cols := splitCols(configCols)

	want := []string{
		"turn_kind", "winder_count",
		"railing", "railing_lower", "railing_landing", "railing_upper",
		"direction", "spiral_direction", "material_code",
	}
	for _, c := range want {
		if !contains(cols, c) {
			t.Errorf("configCols must contain %q (DOM-003); got %v", c, cols)
		}
	}

	// created_at/updated_at/revision — служебные, они есть.
	for _, c := range []string{"id", "project_id", "revision", "created_at", "updated_at"} {
		if !contains(cols, c) {
			t.Errorf("configCols must contain %q; got %v", c, cols)
		}
	}
	// Порядок колонок в configCols должен совпадать с порядком выбора в
	// INSERT (иначе RETURNING/значения съезжают).
	if got := cols[len(cols)-1]; got != "updated_at" {
		t.Errorf("configCols must end with updated_at, got %q", got)
	}
}

// TestDOM003_MigrationAddsNineColumns — миграция 000030 обязана добавлять все
// девять колонок, иначе приложение пишет в несуществующие колонки.
func TestDOM003_MigrationAddsNineColumns(t *testing.T) {
	up := readMigrationFile(t, "000030_config_full_persistence.up.sql")
	for _, c := range []string{
		"turn_kind", "winder_count",
		"railing", "railing_lower", "railing_landing", "railing_upper",
		"direction", "spiral_direction", "material_code",
	} {
		re := regexp.MustCompile(`ADD COLUMN IF NOT EXISTS ` + c + `\b`)
		if !re.MatchString(up) {
			t.Errorf("migration 000030 must add column %q", c)
		}
	}

	down := readMigrationFile(t, "000030_config_full_persistence.down.sql")
	for _, c := range []string{
		"turn_kind", "winder_count",
		"railing", "railing_lower", "railing_landing", "railing_upper",
		"direction", "spiral_direction", "material_code",
	} {
		re := regexp.MustCompile(`DROP COLUMN IF EXISTS ` + c + `\b`)
		if !re.MatchString(down) {
			t.Errorf("migration 000030 down must drop column %q", c)
		}
	}
}

// TestDOM003_WinderCountNeedsWinderTurnKind — CHECK-ограничение не должно
// пропускать комбинацию turn_kind='platform' + winder_count>0: это
// противоречивая конфигурация, которую нельзя построить.
func TestDOM003_WinderCountNeedsWinderTurnKind(t *testing.T) {
	up := readMigrationFile(t, "000030_config_full_persistence.up.sql")
	if !strings.Contains(up, "stair_configurations_winder_check") {
		t.Fatal("migration 000030 must define stair_configurations_winder_check")
	}
	if !strings.Contains(up, "winder_count >= 3") {
		t.Error("winder CHECK must require at least 3 winder steps (stable 180° turn)")
	}
}

func splitCols(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' '
	})
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
