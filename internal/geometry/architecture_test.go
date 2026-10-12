package geometry

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLayerIsolation проверяет ADR-0006: Geometry Platform (ядро STAIR-KERNEL)
// не зависит от HTTP, БД, ORM, UI, AI и stair-домена (инвариант 6 —
// доменно-агностичность; лестница лишь кейс использования).
func TestLayerIsolation(t *testing.T) {
	forbidden := []string{
		"net/http",
		"database/sql",
		"gorm",
		"ent",
		"encoding/json",
		"os/exec",
		"grpc",
		"stairplatform/internal/domain",
		// ADR-0013: Geometry ничего не знает, кроме геометрии. Раньше
		// internal/geometry импортировал internal/engine/graph ради
		// ParametricModel — это был нарушенный запрет, который этот тест
		// НЕ ловил, потому что internal/engine в списке не значился.
		// Мёртвый код удалён (S-176), запрет записан.
		"stairplatform/internal/engine",
	}

	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		// #nosec G304 -- file приходит из filepath.Glob("*.go") текущей директории теста.
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		ast, err := parser.ParseFile(fset, file, b, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, imp := range ast.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, f := range forbidden {
				if strings.HasPrefix(path, f) {
					t.Errorf("%s: forbidden import %q (violates ADR-0006 / invariant 6)", file, path)
				}
			}
		}
	}
}

// TestNoPresentationDependency — ADR-0016: расчёт не зависит от средств
// отображения.
//
// Проверка выглядит формальной: three.js в Go не существует, импортировать
// его физически нельзя. Ценность в другом — правило зафиксировано в тесте,
// и когда кто-то захочет «протащить» рендеринг в расчёт (например, через
// cgo-обёртку или новый пакет с рендер-логикой), запрет будет стоять перед
// ним, а не только в документе.
//
// Отдельно проверяем, что расчётный пакет не тянет ничего из frontend —
// единственная честная точка обмена это kerngeo.Mesh через JSON.
func TestNoPresentationDependency(t *testing.T) {
	// Идентификаторы рендереров, которые не должны встречаться в импортах
	// расчётного пакета. Пустой список не означает, что правило не нужно:
	// смысл в том, чтобы список был назван и проверялся.
	forbidden := []string{
		"three",
		"babylonjs",
		"babylon",
		"@react-three",
		"webgl",
		"webgpu",
		"canvas",
		"glfw",
		"gl",
		"vulkan",
		"playwright",
		"chromedp",
	}

	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, imp := range file.Imports {
			path := imp.Path.Value
			for _, bad := range forbidden {
				if strings.Contains(strings.ToLower(path), bad) {
					t.Errorf("ADR-0016: %s импортирует %s — расчёт не зависит от представления", f, path)
				}
			}
		}
	}
}
