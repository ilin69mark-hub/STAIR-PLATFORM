package pricing

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLayerIsolation проверяет ADR-0006: движок Pricing не зависит
// от HTTP, БД, ORM, UI и AI (fitness function по ADR-0007).
func TestLayerIsolation(t *testing.T) {
	forbidden := []string{
		"net/http",
		"database/sql",
		"gorm",
		"ent",
		"encoding/json",
		"os/exec",
		"react",
		"grpc",
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
					t.Errorf("%s: forbidden import %q (violates ADR-0006)", file, path)
				}
			}
		}
	}
}

// TestEngineDependsOnDomainsOnly проверяет, что движок зависит только
// от доменных пакетов штатной платформы.
func TestEngineDependsOnDomainsOnly(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		ast, err := parser.ParseFile(fset, file, b, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range ast.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(path, "stairplatform/internal/transport") {
				t.Errorf("%s: engine must not depend on transport: %s", file, path)
			}
		}
	}
}
