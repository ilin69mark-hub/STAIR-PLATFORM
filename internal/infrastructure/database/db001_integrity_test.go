package database

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// DB-001 (2026-09-26): целостность схемы.
//
// Проверка по всем 26 таблицам показала, что FK/CHECK/UNIQUE есть у большинства,
// но у ряда колонок инварианты проверялись только кодом приложения. Тест
// фиксирует, что миграция 000031 закрывает именно эти пробелы, и что набор
// ограничений не «поехал» при будущих правках.

// TestDB001_IntegrityMigrationDefinesConstraints — миграция 000031 обязана
// объявлять все шесть ограничений, которые закрывают найденные пробелы.
func TestDB001_IntegrityMigrationDefinesConstraints(t *testing.T) {
	up := readMigrationFile(t, "000031_integrity_constraints.up.sql")
	down := readMigrationFile(t, "000031_integrity_constraints.down.sql")

	want := []string{
		// Один эндпоинт на вид интеграции в пределах tenant.
		"integration_endpoints_tenant_kind_key",
		// Статус проекта: 4 значения из project/entity.go.
		"projects_status_check",
		// Тип марша и геометрия конфигурации.
		"stair_configurations_flight_check",
		"stair_configurations_positive_geometry",
		// Сумма платежа и счётчик попыток.
		"payment_intents_amount_check",
		"integration_events_attempts_check",
	}
	for _, c := range want {
		if !strings.Contains(up, "ADD CONSTRAINT "+c) &&
			!strings.Contains(up, "ADD CONSTRAINT\n\t"+c) &&
			!strings.Contains(strings.ReplaceAll(up, "\n\t", " "), "ADD CONSTRAINT "+c) {
			t.Errorf("000031 up must add constraint %q", c)
		}
		if !strings.Contains(down, "DROP CONSTRAINT IF EXISTS "+c) &&
			!strings.Contains(strings.ReplaceAll(down, "\n\t", " "), "DROP CONSTRAINT IF EXISTS "+c) {
			t.Errorf("000031 down must drop constraint %q", c)
		}
	}

	// Статусы проекта — ровно доменные константы, а не «на глаз».
	for _, st := range []string{"draft", "in_review", "approved", "changes_requested"} {
		if !strings.Contains(up, "'"+st+"'") {
			t.Errorf("projects_status_check must allow %q (project.Status*)", st)
		}
	}
	// Типы марша — доменные значения Flight.
	for _, f := range []string{"straight", "l_shape", "u_shape", "spiral"} {
		if !strings.Contains(up, "'"+f+"'") {
			t.Errorf("stair_configurations_flight_check must allow %q", f)
		}
	}
}

// TestDB001_ProjectStatusConstantsMatchConstraint — константы домена и CHECK
// обязаны совпадать. Расхождение означало бы, что приложение пишет статус,
// который БД не примет (500 на ровном месте), либо, наоборот, принимает
// «будущий» статус, которого нет в коде.
func TestDB001_ProjectStatusConstantsMatchConstraint(t *testing.T) {
	up := readMigrationFile(t, "000031_integrity_constraints.up.sql")
	start := strings.Index(up, "CHECK (status IN (")
	if start < 0 {
		t.Fatal("projects_status_check not found in 000031 up")
	}
	rest := up[start+len("CHECK (status IN ("):]
	end := strings.Index(rest, ")")
	if end < 0 {
		t.Fatal("malformed projects_status_check")
	}
	inSQL := map[string]bool{}
	for _, v := range strings.Split(rest[:end], ",") {
		inSQL[strings.Trim(strings.TrimSpace(v), "'")] = true
	}

	// Константы из internal/application/project/entity.go.
	inCode := map[string]bool{
		"draft": true, "in_review": true, "approved": true, "changes_requested": true,
	}
	for st := range inCode {
		if !inSQL[st] {
			t.Errorf("domain status %q is not allowed by projects_status_check", st)
		}
	}
	for st := range inSQL {
		if !inCode[st] {
			t.Errorf("projects_status_check allows %q, which the domain does not define", st)
		}
	}
}

// TestDB001_UniqueEndpointPerKindInMigration — UNIQUE (tenant_id, kind)
// должен быть именно на паре tenant+kind: UNIQUE на один kind запретил бы
// разным tenant иметь свои эндпоинты.
func TestDB001_UniqueEndpointPerKindInMigration(t *testing.T) {
	up := readMigrationFile(t, "000031_integrity_constraints.up.sql")
	needle := "UNIQUE (tenant_id, kind)"
	if !strings.Contains(up, needle) {
		t.Errorf("000031 must declare %q on integration_endpoints", needle)
	}
	// Не должно быть уникальности только по kind (сломало бы мультиарендность).
	if strings.Contains(up, "UNIQUE (kind)") {
		t.Error("UNIQUE (kind) would forbid per-tenant endpoints")
	}
}

// TestDB001_ComfortStepAllowsZeroDefault — регрессия CRITICAL-01 (2026-09-26).
//
// `comfort_step_mm = 0` — не «битое» значение, а доменный сентинел: application-слой
// трактует 0 как «взять solver.DefaultComfortStep» (application/stair/service.go:76,
// application/stair/optimize.go:266). Ограничение `> 0` роняло сохранение такой
// конфигурации ошибкой 23514 — то есть сам домен оказывался недостижим через API.
//
// Проверка парная: нестрогие колонки (comfort/clearance/railing) обязаны быть
// >= 0, а обязательная геометрия остаться строгой (> 0) — «ослабили три CHECK» не
// должно означать «ослабили все».
func TestDB001_ComfortStepAllowsZeroDefault(t *testing.T) {
	up := readMigrationFile(t, "000031_integrity_constraints.up.sql")

	// comfort_step_mm — неотрицательное.
	if !regexp.MustCompile(`comfort_step_mm\s+>= 0`).MatchString(up) {
		t.Error("stair_configurations_positive_geometry must allow comfort_step_mm >= 0 " +
			"(0 = solver.DefaultComfortStep, application/stair/service.go:76)")
	}
	if regexp.MustCompile(`comfort_step_mm\s+> 0`).MatchString(up) {
		t.Error("comfort_step_mm > 0 rejects the legal default-step sentinel " +
			"(CRITICAL-01: save of a config with ComfortStepMM=0 failed with 23514)")
	}

	// Остальные геометрические колонки остаются строгими.
	for _, col := range []string{
		"width_mm", "height_mm", "step_height_mm", "stringer_thickness_mm",
		"step_thickness_mm",
	} {
		re := regexp.MustCompile(col + `\s+> 0`)
		if !re.MatchString(up) {
			t.Errorf("geometry CHECK must still require %s > 0", col)
		}
	}

	// clearance_mm и railing_height_mm, как и comfort_step_mm, допускают 0 —
	// это легальные доменные значения, а не «битые» данные. Их нестрогий
	// вариант — та же регрессия CRITICAL-01, только на других колонках:
	//   * clearance_mm = 0 заведено в демо-фикстуре, из-за чего `make seed`
	//     падал с 23514;
	//   * railing_height_mm = 0 = перил нет (engine/geometry/railing.go:53,
	//     RailingNone в domain/engineering/stair.go:35).
	for _, col := range []string{"clearance_mm", "railing_height_mm"} {
		if !regexp.MustCompile(col + `\s+>= 0`).MatchString(up) {
			t.Errorf("geometry CHECK must allow %s >= 0 (0 is a legal domain value)", col)
		}
		if regexp.MustCompile(col + `\s+> 0`).MatchString(up) {
			t.Errorf("%s > 0 rejects the legal zero value (CRITICAL-01)", col)
		}
	}
}

// TestDB001_MigrationsHaveDownForEveryUp — все миграции парные.
func TestDB001_MigrationsHaveDownForEveryUp(t *testing.T) {
	entries, err := os.ReadDir(migrationsDir(t))
	if err != nil {
		t.Fatal(err)
	}
	ups := map[string]bool{}
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".up.sql") {
			ups[strings.TrimSuffix(n, ".up.sql")] = true
		}
	}
	if len(ups) == 0 {
		t.Fatal("no up migrations found")
	}
	for n := range ups {
		if _, err := os.Stat(filepath.Join(migrationsDir(t), n+".down.sql")); err != nil {
			t.Errorf("migration %s has no .down.sql", n)
		}
	}
}

// TestMIG001_QueryIndexesDefined — миграция 000032 обязана добавлять индексы
// под реальные запросы кода (MIG-001/002):
//   - audit_events.created_at — под retention DELETE (DeleteBefore);
//   - (tenant_id, action, created_at DESC) — под аналитику;
//   - ai_corpus_chunks.tenant_id — под выборку корпуса (tenant или общий);
//   - project_members(user_id) — под список проектов пользователя.
func TestMIG001_QueryIndexesDefined(t *testing.T) {
	up := readMigrationFile(t, "000032_query_indexes.up.sql")
	down := readMigrationFile(t, "000032_query_indexes.down.sql")
	for _, idx := range []string{
		"idx_audit_events_created_at",
		"idx_audit_events_tenant_action_created",
		"idx_ai_corpus_chunks_tenant",
		"idx_project_members_user",
	} {
		if !strings.Contains(up, "CREATE INDEX IF NOT EXISTS "+idx) {
			t.Errorf("000032 must create index %q", idx)
		}
		if !strings.Contains(down, "DROP INDEX IF EXISTS "+idx) {
			t.Errorf("000032 down must drop index %q", idx)
		}
	}
	// Индекс по ai_corpus_chunks.tenant_id отсутствовал вообще — именно это
	// делало каждый запрос ассистента полным сканированием корпуса.
	prior := ""
	for i := 1; i <= 31; i++ {
		p := filepath.Join(migrationsDir(t), fmt.Sprintf("%06d_", i))
		matches, _ := filepath.Glob(p + "*.up.sql")
		for _, m := range matches {
			raw, err := os.ReadFile(m)
			if err != nil {
				continue
			}
			prior += string(raw)
		}
	}
	if strings.Contains(prior, "ON ai_corpus_chunks (tenant_id)") {
		t.Error("idx_ai_corpus_chunks_tenant уже существовал в ранних миграциях — " +
			"тогда список выше некорректен")
	}
}
