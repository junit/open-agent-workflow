package check_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/check"
)

func TestExecuteResolvesUserTargetsInRegistryOrder(t *testing.T) {
	root := t.TempDir()
	result, err := check.Execute(testCatalog(t), check.Environment{
		Home:       filepath.Join(root, "home"),
		ConfigHome: filepath.Join(root, "config"),
		StateHome:  filepath.Join(root, "state"),
		Path:       filepath.Join(root, "bin"),
	}, check.Request{Targets: "opencode,claude,codex,claude,gemini"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := []string{
		"version: 0.1.0",
		"scope: user",
		"targets: claude,codex,gemini,opencode",
	}
	if got := result.Lines[:3]; !reflect.DeepEqual(got, want) {
		t.Fatalf("leading lines = %#v, want %#v", got, want)
	}
}

func TestExecuteResolvesPhysicalProjectAndProjectDefaults(t *testing.T) {
	root := t.TempDir()
	realProject := filepath.Join(root, "real project")
	projectLink := filepath.Join(root, "project link")
	for _, path := range []string{filepath.Join(root, "home"), filepath.Join(root, "config"), filepath.Join(root, "state"), realProject} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", path, err)
		}
	}
	if err := os.Symlink(realProject, projectLink); err != nil {
		t.Fatalf("Symlink(): %v", err)
	}
	physical, err := filepath.EvalSymlinks(realProject)
	if err != nil {
		t.Fatalf("EvalSymlinks(): %v", err)
	}
	result, err := check.Execute(testCatalog(t), check.Environment{
		Home:       filepath.Join(root, "home"),
		ConfigHome: filepath.Join(root, "config"),
		StateHome:  filepath.Join(root, "state"),
		Path:       filepath.Join(root, "bin"),
	}, check.Request{Project: projectLink})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := []string{
		"version: 0.1.0",
		"scope: project (" + physical + ")",
		"targets: claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot",
	}
	if got := result.Lines[:3]; !reflect.DeepEqual(got, want) {
		t.Fatalf("leading lines = %#v, want %#v", got, want)
	}
}

func TestExecuteRejectsInvalidScopeAndTargets(t *testing.T) {
	root := t.TempDir()
	environment := check.Environment{Home: root, ConfigHome: root, StateHome: root}
	tests := []struct {
		name    string
		request check.Request
		message string
	}{
		{name: "whitespace", request: check.Request{Targets: "claude, codex"}, message: "target selection must not contain whitespace"},
		{name: "empty member", request: check.Request{Targets: "claude,,codex"}, message: "target selection contains an empty member"},
		{name: "unknown", request: check.Request{Targets: "vscode"}, message: "unknown target 'vscode'"},
		{name: "extension user target", request: check.Request{Targets: "cursor"}, message: "target 'cursor' does not support user scope"},
		{name: "missing project", request: check.Request{Project: filepath.Join(root, "missing")}, message: "project directory does not exist"},
		{name: "control project", request: check.Request{Project: root + "\nbad"}, message: "project path contains control characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := check.Execute(testCatalog(t), environment, tt.request)
			var checkError *check.Error
			if err == nil || !strings.Contains(err.Error(), tt.message) || !errors.As(err, &checkError) || checkError.Status != 64 {
				t.Fatalf("Execute() error = %v, want status 64 containing %q", err, tt.message)
			}
		})
	}
}

func testCatalog(t *testing.T) catalog.Catalog {
	t.Helper()
	value, err := builtin.Load()
	if err != nil {
		t.Fatalf("builtin.Load(): %v", err)
	}
	return value
}
