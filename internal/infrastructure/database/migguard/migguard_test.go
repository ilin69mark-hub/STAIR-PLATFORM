package migguard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// textQueryAllowed — правила, которым диагностический SELECT ОБЯЗАН жить в
// коде: каталог для них бесполезен. Любое новое правило из этого списка
// требует правки здесь и обоснования.
var textQueryAllowed = map[string]string{
	// Зеркалит ограничение, которого в БД ещё нет: миграция 000040 сужает
	// CHECK до трёх типов марша, а в базе пока перечень из четырёх.
	"stair_configurations_flight_check": "зеркалит ещё не применённое сужение 000040, каталог отражает прошлое",
	// Не CHECK, а UNIQUE (contype='u'): выражения в conbin у него нет.
	"integration_endpoints_tenant_kind_key": "UNIQUE, а не CHECK — выражения в conbin нет",
}

// litRe находит строковые литералы вида 'x.y.z' в тексте правила.
var litRe = regexp.MustCompile(`'[A-Za-z_][A-Za-z0-9_.]*'`)

// TestChecksDeclareExactlyOneQuerySource — каждое правило объявляет либо
// готовый Query, либо вывод из каталога. Оба сразу или ни одного — ошибка:
// первое даёт два источника правды, второе — правило, которое молча ничего
// не проверяет.
func TestChecksDeclareExactlyOneQuerySource(t *testing.T) {
	for _, c := range checks {
		switch {
		case c.Catalog && strings.TrimSpace(c.Query) != "":
			t.Errorf("%s: Catalog и Query заданы одновременно — уберите Query", c.Constraint)
		case !c.Catalog && strings.TrimSpace(c.Query) == "":
			t.Errorf("%s: не задан ни Query, ни Catalog — правило ничего не проверит", c.Constraint)
		case c.Table == "":
			t.Errorf("%s: не задана таблица", c.Constraint)
		}
	}
}

// TestNoHardcodedEnumListsInCode — регрессия DB-1, из-за которой preflight
// блокировал подъём миграций на легитимных данных.
//
// Правило с перечнем допустимых значений в коде расходится с реальностью
// каждый раз, когда миграция переписывает CHECK: audit-actions переписывали
// миграции 000037, 000038 и 000039, а перечень в коде остался от 000029. В
// итоге список знал значения, которых в CHECK давно нет, и не знал новых —
// preflight ругался на корректные строки журнала.
//
// Поэтому перечней из трёх и более строковых литералов в тексте правила быть
// не должно: для применённых ограничений есть Catalog.
func TestNoHardcodedEnumListsInCode(t *testing.T) {
	for _, c := range checks {
		if c.Catalog {
			continue
		}
		if n := len(litRe.FindAllString(c.Query, -1)); n >= 3 {
			if _, ok := textQueryAllowed[c.Constraint]; !ok {
				t.Errorf("%s: перечень из %d значений задан в коде — переведите правило "+
					"на Catalog или внесите его в textQueryAllowed с обоснованием",
					c.Constraint, n)
			}
		}
	}
}

// TestTextQueryRulesAreJustified — каждое исключение обязано быть объяснено
// и реально существовать, иначе оно молча превращается в ещё одно
// расхождение либо в мёртвую запись.
func TestTextQueryRulesAreJustified(t *testing.T) {
	known := map[string]bool{}
	for _, c := range checks {
		known[c.Constraint] = true
	}
	for name, why := range textQueryAllowed {
		if !known[name] {
			t.Errorf("textQueryAllowed содержит %q, но такого правила в checks нет", name)
		}
		if strings.TrimSpace(why) == "" {
			t.Errorf("%s: исключение без обоснования", name)
		}
	}
}

// TestEveryCheckResolvesToRealConstraint — имена ограничений должны
// встречаться в миграциях. Опечатка в имени даёт правило, которое никогда не
// срабатывает, а для Catalog — молчаливый skip.
//
// Миграции лежат от пакета на четыре уровня выше:
// internal/infrastructure/database/migguard → repo/migrations.
func TestEveryCheckResolvesToRealConstraint(t *testing.T) {
	body := migrationsSource(t)
	for _, c := range checks {
		if !strings.Contains(body, c.Constraint) {
			t.Errorf("%s: имя не найдено среди текстов миграций — проверь опечатку", c.Constraint)
		}
	}
}

// migrationsSource склеивает все up-миграции. Отсутствие каталога миграций —
// не тихая деградация: тест не проверял бы ничего.
func migrationsSource(t *testing.T) string {
	t.Helper()
	files, err := filepath.Glob("../../../../migrations/*.up.sql")
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("up-миграции не найдены — тест запущен не из ожидаемого каталога")
	}
	var b strings.Builder
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		b.Write(raw)
	}
	return b.String()
}

// TestViolationQueryEscapesIdentifier — имя таблицы попадает в SQL как есть,
// поэтому обязано быть экранировано. Выражение CHECK приходит из системного
// каталога той же базы.
func TestViolationQueryEscapesIdentifier(t *testing.T) {
	q := violationQuery("weird table", "x > 0")
	if !strings.Contains(q, `"weird table"`) {
		t.Errorf("имя таблицы не экранировано: %s", q)
	}
	if !strings.Contains(q, "NOT (x > 0)") {
		t.Errorf("выражение ограничения не подставлено: %s", q)
	}
	if !strings.HasSuffix(q, "LIMIT 10") {
		t.Errorf("в запросе нет LIMIT: %s", q)
	}
}
