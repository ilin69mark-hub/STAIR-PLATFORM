package database

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readMigrationFile читает файл миграции из migrations/.
func readMigrationFile(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(migrationsDir(t), name)) //nolint:gosec // G304: путь к эталону/фикстуре задаёт сам тест
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
//
// После удаления винтового марша (решение владельца 2026-10-06) spiral_direction
// из проекции убрана: колонка в схеме осталась историей миграции 000030, но
// приложение её ни читает, ни пишет. Проверяем восемь оставшихся.
func TestDOM003_ConfigColsCoverAllMappedFields(t *testing.T) {
	cols := splitCols(configCols)

	want := []string{
		"turn_kind", "winder_count",
		"railing", "railing_lower", "railing_landing", "railing_upper",
		"direction", "material_code",
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

// TestConfigColsMatchScanArgs — регрессия: число колонок в configCols должно
// совпадать с числом аргументов row.Scan в scanConfig.
//
// Предыдущий тест проверял лишь НАЛИЧИЕ нужных колонок, но не их количество.
// Поэтому лишний аргумент Scan (исторически — &c.OuterRadiusMM после того, как
// колонку outer_radius_mm выбросили из проекции и из схемы миграцией
// 000040_drop_spiral_flight) остался незамеченным: любое чтение конфигурации
// падало в рантайме с ошибкой сканирования, а интеграционные тесты этого не
// покрывали. Считаем оба числа через go/ast, чтобы регрессия ловилась сразу.
func TestConfigColsMatchScanArgs(t *testing.T) {
	ncols := len(splitCols(configCols))

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "project_repo.go", nil, 0)
	if err != nil {
		t.Fatalf("parse project_repo.go: %v", err)
	}

	var scanArgs int
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "scanConfig" || fn.Body == nil {
			return true
		}
		ast.Inspect(fn.Body, func(m ast.Node) bool {
			call, ok := m.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Scan" {
				return true
			}
			scanArgs = len(call.Args)
			found = true
			return false
		})
		return false
	})

	if !found {
		t.Fatal("row.Scan call not found inside scanConfig")
	}
	if scanArgs != ncols {
		t.Fatalf("scanConfig reads %d columns but configCols lists %d: %v", scanArgs, ncols, splitCols(configCols))
	}
}

// TestInsertConfigurationPlaceholdersMatch — регрессия: число выражений в
// VALUES обязано совпадать с числом колонок в списке INSERT (подзапрос для
// revision считается выражением).
//
// Тот же класс ошибки, что и с лишним аргументом Scan: при удалении колонок
// outer_radius_mm / spiral_direction из списка колонок плейсхолдеры $28,$29
// остались в VALUES, и КАЖДОЕ сохранение конфигурации падало на уровне SQL
// («INSERT has more expressions than target columns»). Интеграционные тесты
// без БД этого не видят. Считаем через go/ast, чтобы поймать наCI.
func TestInsertConfigurationPlaceholdersMatch(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "project_repo.go", nil, 0)
	if err != nil {
		t.Fatalf("parse project_repo.go: %v", err)
	}

	var targets, exprs int
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "insertConfiguration" || fn.Body == nil {
			return true
		}
		ast.Inspect(fn.Body, func(m ast.Node) bool {
			call, ok := m.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "QueryRow" {
				return true
			}
			var sql string
			for _, a := range call.Args {
				if lit, ok := a.(*ast.BasicLit); ok {
					sql = strings.ReplaceAll(lit.Value, "`", "")
					break
				}
			}
			if !strings.Contains(sql, "INSERT INTO stair_configurations") {
				return true
			}
			parts := strings.SplitN(sql, "VALUES", 2)
			if len(parts) != 2 {
				t.Fatal("INSERT без VALUES")
			}
			targets = countSplit(parts[0])
			exprs = countSplit(parts[1])
			found = true
			return false
		})
		return false
	})

	if !found {
		t.Fatal("INSERT INTO stair_configurations not found inside insertConfiguration")
	}
	if targets != exprs {
		t.Fatalf("insertConfiguration lists %d columns but VALUES has %d expressions",
			targets, exprs)
	}
}

// countSplit считает элементы списка через запятую на верхнем уровне,
// игнорируя запятые внутри выражений: COALESCE(MAX(...),0) — это один
// элемент, хотя запятая внутри есть.
func countSplit(s string) int {
	n := 0
	depth := 0
	for _, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 1 {
				n++
			}
		}
	}
	return n
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
