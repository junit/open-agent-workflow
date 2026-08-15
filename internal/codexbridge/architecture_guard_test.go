package codexbridge_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/wifibaby4u/open-agent-workflow/"

func TestDefaultOAWHasNoOptionalAssuranceOrBridgeDependency(t *testing.T) {
	dependencies := goListBridgeDependencies(t, "./cmd/oaw")
	for _, forbidden := range []string{
		modulePath + "internal/assurance",
		modulePath + "internal/assurancecli",
		modulePath + "internal/bridgecli",
		modulePath + "internal/codexbridge",
		modulePath + "internal/classification",
		modulePath + "internal/coordinator",
		modulePath + "internal/core",
		modulePath + "internal/policyengagement",
		modulePath + "internal/policyflow",
		modulePath + "internal/policyrun",
		modulePath + "internal/policyroute",
		modulePath + "internal/profile",
	} {
		if dependencies[forbidden] {
			t.Errorf("default oaw imports optional component %q", forbidden)
		}
	}
}

func TestStandaloneBridgeDependsOnAssurance(t *testing.T) {
	dependencies := goListBridgeDependencies(t, "./cmd/oaw-bridge")
	for _, required := range []string{
		modulePath + "internal/assurance",
		modulePath + "internal/bridgecli",
		modulePath + "internal/codexbridge",
		modulePath + "internal/profileinspect",
	} {
		if !dependencies[required] {
			t.Errorf("oaw-bridge omits required dependency %q", required)
		}
	}
}

func TestBridgeImportsNoMachineWorkflowAuthority(t *testing.T) {
	root := bridgeRepositoryRoot(t)
	forbidden := []string{
		"internal/admission",
		"internal/builtin",
		"internal/classification",
		"internal/coordinator",
		"internal/core",
		"internal/policyengagement",
		"internal/policyflow",
		"internal/policyrun",
		"internal/policyroute",
		"internal/profile",
	}
	for _, relative := range []string{"cmd/oaw-bridge", "internal/bridgecli", "internal/codexbridge"} {
		directory := filepath.Join(root, relative)
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imported := range parsed.Imports {
				importPath, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				for _, fragment := range forbidden {
					if importPath == modulePath+fragment || strings.HasPrefix(importPath, modulePath+fragment+"/") {
						t.Errorf("%s imports retired machine workflow package %q", path, importPath)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func goListBridgeDependencies(t *testing.T, target string) map[string]bool {
	t.Helper()
	command := exec.Command("go", "list", "-deps", target)
	command.Dir = bridgeRepositoryRoot(t)
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

func bridgeRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate codexbridge package")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
