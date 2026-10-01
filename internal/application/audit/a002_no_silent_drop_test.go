package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestAUDIT002_NoSilentlyDiscardedRecordCalls — архитектурный гейт (AUDIT-002,
// 2026-09-27): вызов Record обязан проверять ошибку либо явно передавать её
// дальше. Молчаливый `_ = auditSvc.Record(...)` больше не допускается.
//
// Почему это гейт, а не просто замечание. Девять вызовов из двенадцати
// использовали `_ =`, и именно поэтому забытое поле Result приводило к
// тихой потере события аудита: операция проходила, клиент получал 200, а в
// журнале не появлялось ничего. Девять одинаково неправильных мест — это уже
// не опечатка, а норма, которую закрепили примером. Норма без гейта
// восстанавливается при первом же добавлении нового маршрута.
//
// Два разрешённых способа обработать ошибку:
//   - `if err := svc.Record(...); err != nil { ... }` — с логированием;
//   - `return svc.Record(...)` / присваивание в переменную, которая
//     возвращается или проверяется.
//
// Проверка построена на go/ast: сигнатуры не нужны, достаточно факта
// присваивания результата в пустой идентификатор.
func TestAUDIT002_NoSilentlyDiscardedRecordCalls(t *testing.T) {
	// Корень модуля: internal/application/audit/audit_test.go → ../../../..
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	fset := token.NewFileSet()
	var violations []string

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			// Пропускаем не-our-code и vendor.
			if base == "node_modules" || base == "vendor" || base == ".git" ||
				strings.HasPrefix(base, "zz_") || base == "dist" || base == "build" {
				return filepath.SkipDir
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			// Только наш код Go: internal/, cmd/, tests/.
			top := strings.SplitN(rel, string(filepath.Separator), 2)[0]
			if top != "internal" && top != "cmd" && top != "tests" {
				return nil
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		top := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		if top != "internal" && top != "cmd" && top != "tests" {
			return nil
		}

		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // файл не парсится — не наша забота здесь
		}
		ast.Inspect(f, func(n ast.Node) bool {
			// `_ = <expr>` — AssignStmt с одним LHS: Ident "_".
			as, ok := n.(*ast.AssignStmt)
			if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			id, ok := as.Lhs[0].(*ast.Ident)
			if !ok || id.Name != "_" {
				return true
			}
			call, ok := as.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Record" {
				return true
			}
			pos := fset.Position(as.Pos())
			violations = append(violations,
				pos.Filename+":"+strconv.Itoa(pos.Line)+
					": результат Record выброшен в _ — событие аудита может потеряться молча (AUDIT-002)")
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("найдено %d мест, где ошибка Record выбрасывается:\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}
