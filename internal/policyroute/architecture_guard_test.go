package policyroute_test

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

func TestPolicyRouteAdapterDoesNotImportMachineAuthorityOrLegacyInspection(t *testing.T) {
	t.Parallel()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate policyroute package")
	}
	packageDir := filepath.Dir(filename)
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"internal/codexbridge",
		"internal/cooperative",
		"internal/coordinator",
		"internal/core",
		"internal/discovery",
		"internal/integrity",
		"internal/registry",
		"internal/builtin",
		"internal/catalog",
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(files, filepath.Join(packageDir, entry.Name()), nil, parser.ImportsOnly)
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
					position := files.Position(imported.Pos())
					t.Errorf("%s:%d imports forbidden package %q", entry.Name(), position.Line, path)
				}
			}
		}
	}
}
