package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// allowedActionRe выделяет значения из CHECK в тексте миграции.
var allowedActionRe = regexp.MustCompile(`'([a-z][a-z0-9_.]*)'`)

// newActionRe выделяет имя константы из строки `ActionXxx Action = "x.y"`.
var newActionRe = regexp.MustCompile(`(Action\w+)\s+Action\s*=\s*"([^"]+)"`)

// TestWrittenActionsAreAllowedByMigration — регрессия: каждое действие аудита,
// которое реально пишется в журнал, должно быть разрешено CHECK из последней
// миграции.
//
// Дефект, который этот тест закрывает. Миграция 000038 задумывалась как
// «добавить user.deleted», но подменила перечень целиком на другой словарь и
// выбросила десять значений, которые код до сих пор пишет. Запись падала по
// CHECK, а Service.record ошибку глотает (только лог) — поэтому журнал
// безопасности МОЛЧА терял события: «конфигурация рассчитана», «CAD выгружен»,
// «конфигурация оптимизирована», «член добавлен/удалён/роль изменена»,
// «запрос на ревью», «смена состояния оплаты», «выдача/отзыв API-ключа».
// Ни один тест этого не замечал: событие просто не появлялось.
//
// Миграции лежат от пакета на четыре уровня выше:
// internal/application/audit → repo/migrations.
func TestWrittenActionsAreAllowedByMigration(t *testing.T) {
	allowed := liveCheckActions(t)
	written := writtenActions(t)

	var missing []string
	for name, value := range written {
		if !allowed[value] {
			missing = append(missing, value+" ("+name+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("код пишет %d действий, запрещённых CHECK последней миграции; "+
			"каждая такая запись падает, а record() глотает ошибку:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// liveCheckActions — значения из ПОСЛЕДНЕЙ миграции, переписывающей CHECK
// audit_events_action_check. Именно она определяет, что база примет сейчас.
//
// Объединение всех миграций здесь не годится: перечни накапливались слоями
// (000029 → 000037 → 000038 → 000039), и 000037 ещё содержит значения, которые
// 000038 выбросила. Такое «объединение» замаскировало бы именно тот дефект,
// ради которого тест написан.
func liveCheckActions(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("../../../migrations/*.up.sql")
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("up-миграции не найдены — тест запущен не из ожидаемого каталога")
	}

	// Последняя по номеру миграция, которая вообще задаёт этот CHECK.
	var newest string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src := string(raw)
		if !strings.Contains(src, "ADD CONSTRAINT audit_events_action_check") {
			continue
		}
		if newest == "" || f > newest {
			newest = f
		}
	}
	if newest == "" {
		t.Fatal("ни одна миграция не задаёт audit_events_action_check")
	}

	raw, err := os.ReadFile(newest)
	if err != nil {
		t.Fatalf("read %s: %v", newest, err)
	}
	// Внутри последнего перечня: между ADD CONSTRAINT и закрывающей скобкой.
	src := string(raw)
	body := src[strings.Index(src, "ADD CONSTRAINT audit_events_action_check"):]
	if i := strings.Index(body, "));"); i >= 0 {
		body = body[:i]
	}
	out := map[string]bool{}
	for _, m := range allowedActionRe.FindAllStringSubmatch(body, -1) {
		out[m[1]] = true
	}
	if len(out) == 0 {
		t.Fatalf("в %s не нашлось ни одного значения действия", newest)
	}
	return out
}

// writtenActions — константы Action, на которые ссылается production-код.
// Объявленные, но нигде не используемые константы сюда не попадают: для них
// рассинхронизация безобидна (записи не будет вовсе).
func writtenActions(t *testing.T) map[string]string {
	t.Helper()
	// Сначала имя константы → её значение.
	self, err := parser.ParseFile(token.NewFileSet(), "entity.go", nil, 0)
	if err != nil {
		t.Fatalf("parse entity.go: %v", err)
	}
	valueOf := map[string]string{}
	ast.Inspect(self, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 || vs.Type == nil {
			return true
		}
		// Только константы типа Action. Рядом в этом же файле живут
		// ResultOK / ResultFailed / ResultDenied — это РЕЗУЛЬТАТ действия
		// ("ok"/"failed"/"denied"), а не действие, и в CHECK их быть не может.
		id, ok := vs.Type.(*ast.Ident)
		if !ok || id.Name != "Action" {
			return true
		}
		lit, ok := vs.Values[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		if v, err := strconv.Unquote(lit.Value); err == nil {
			valueOf[vs.Names[0].Name] = v
		}
		return true
	})
	if len(valueOf) == 0 {
		t.Fatal("в entity.go не нашлось констант Action")
	}

	// Затем — какие из них реально упоминаются в production-коде репозитория.
	roots := []string{"..", "../../.."}
	written := map[string]string{}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if strings.HasSuffix(path, "audit/entity.go") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			src := string(raw)
			for name, value := range valueOf {
				if _, ok := written[name]; ok {
					continue
				}
				if regexp.MustCompile(`\b` + name + `\b`).MatchString(src) {
					written[name] = value
				}
			}
			return nil
		})
	}
	if len(written) == 0 {
		t.Fatal("не найдено ни одной константы Action, используемой в коде")
	}
	return written
}
