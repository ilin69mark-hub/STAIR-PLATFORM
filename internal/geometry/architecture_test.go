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
