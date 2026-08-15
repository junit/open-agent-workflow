package assurance_test

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestAssuranceImportsNoLegacyWorkflowAuthority(t *testing.T) {
	repository := assuranceRepositoryRoot(t)
	forbidden := []string{
		"internal/admission", "internal/assets", "internal/builtin", "internal/catalog",
		"internal/classification", "internal/codexbridge", "internal/config", "internal/coordinator",
		"internal/core", "internal/discovery", "internal/execution", "internal/host",
		"internal/policycatalog", "internal/policyengagement", "internal/policyflow",
		"internal/policyrun", "internal/policyroute", "internal/profile", "internal/registry",
		"internal/schema",
	}
	for _, relative := range []string{"internal/assurance", "internal/assurancecli"} {
		directory := filepath.Join(repository, relative)
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
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
					if strings.HasSuffix(path, fragment) || strings.Contains(path, fragment+"/") {
						t.Errorf("%s imports legacy workflow authority package %q", entry.Name(), path)
					}
				}
			}
		}
	}
}

func TestDefaultOAWHasNoAssuranceDependency(t *testing.T) {
	dependencies := goListDependencies(t, "./cmd/oaw")
	for _, forbidden := range []string{
		"github.com/wifibaby4u/open-agent-workflow/internal/assurance",
		"github.com/wifibaby4u/open-agent-workflow/internal/assurancecli",
	} {
		if dependencies[forbidden] {
			t.Errorf("default oaw imports optional package %q", forbidden)
		}
	}
}

func TestStandaloneAssuranceDependencyDirection(t *testing.T) {
	dependencies := goListDependencies(t, "./cmd/oaw-assurance")
	for _, required := range []string{
		"github.com/wifibaby4u/open-agent-workflow/internal/assurance",
		"github.com/wifibaby4u/open-agent-workflow/internal/assurancecli",
		"github.com/wifibaby4u/open-agent-workflow/internal/profileinspect",
	} {
		if !dependencies[required] {
			t.Errorf("oaw-assurance omits required dependency %q", required)
		}
	}
	for _, forbidden := range []string{
		"github.com/wifibaby4u/open-agent-workflow/internal/admission",
		"github.com/wifibaby4u/open-agent-workflow/internal/classification",
		"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge",
		"github.com/wifibaby4u/open-agent-workflow/internal/coordinator",
		"github.com/wifibaby4u/open-agent-workflow/internal/core",
		"github.com/wifibaby4u/open-agent-workflow/internal/profile",
	} {
		if dependencies[forbidden] {
			t.Errorf("oaw-assurance imports forbidden workflow package %q", forbidden)
		}
	}
}

func goListDependencies(t *testing.T, target string) map[string]bool {
	t.Helper()
	command := exec.Command("go", "list", "-deps", target)
	command.Dir = assuranceRepositoryRoot(t)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", target, err)
	}
	result := make(map[string]bool)
	for _, dependency := range strings.Fields(string(output)) {
		result[dependency] = true
	}
	return result
}

func assuranceRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate assurance package")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
