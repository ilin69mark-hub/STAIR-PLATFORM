package auth

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLayerIsolation проверяет ADR-0006: application layer (DOM-0008)
// не зависит от HTTP, БД, ORM, UI и AI (fitness function по ADR-0007).
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
		b, err := os.ReadFile(file) //nolint:gosec // test reads local package source files
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

// TestNoInfrastructureDependency проверяет, что application layer
// зависит от порта Repository, а не от реализации (инверсия зависимостей).
func TestNoInfrastructureDependency(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		b, err := os.ReadFile(file) //nolint:gosec // test reads local package source files
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		ast, err := parser.ParseFile(fset, file, b, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, imp := range ast.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(path, "stairplatform/internal/infrastructure") {
				t.Errorf("%s: application layer must not depend on infrastructure: %s", file, path)
			}
		}
	}
}
