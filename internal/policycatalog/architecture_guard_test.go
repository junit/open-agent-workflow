package policycatalog_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestPolicyCatalogHasNoMachineSemanticDependency(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate policycatalog package")
	}
	directory := filepath.Dir(filename)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"internal/assets", "internal/builtin", "internal/catalog", "internal/codexbridge",
		"internal/coordinator", "internal/core", "internal/discovery", "internal/integrity",
		"internal/profile", "internal/provideraudit", "internal/registry", "internal/schema",
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(files, filepath.Join(directory, entry.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			for _, fragment := range forbidden {
				if strings.Contains(path, fragment) {
					t.Errorf("%s imports machine semantic package %q", entry.Name(), path)
				}
			}
		}
	}
}
