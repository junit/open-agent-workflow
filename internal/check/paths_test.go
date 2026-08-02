package check

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeCoordinatesRejectsUnsafeRootsAndComponents(t *testing.T) {
	root := t.TempDir()
	base := Environment{
		Home: filepath.Join(root, "home"), ConfigHome: filepath.Join(root, "config"),
		StateHome: filepath.Join(root, "state"), Path: filepath.Join(root, "bin"),
	}
	for _, directory := range []string{base.Home, base.ConfigHome, base.StateHome} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name        string
		environment Environment
		setup       func(t *testing.T)
		message     string
	}{
		{name: "relative home", environment: Environment{Home: "relative", ConfigHome: base.ConfigHome, StateHome: base.StateHome}, message: "root must be an absolute path"},
		{name: "control config", environment: Environment{Home: base.Home, ConfigHome: base.ConfigHome + "\nbad", StateHome: base.StateHome}, message: "contains control characters"},
		{name: "config symlink", environment: base, setup: func(t *testing.T) {
			if err := os.Symlink(t.TempDir(), filepath.Join(base.ConfigHome, "open-agent-workflow")); err != nil {
				t.Fatal(err)
			}
		}, message: "destination path contains a symlink"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}
			if _, err := initializeCoordinates(tt.environment, resolvedRequest{scope: "user"}); err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("initializeCoordinates() error = %v, want %q", err, tt.message)
			}
		})
	}
}

func TestValidatedDestinationPathRejectsUnsafeSuffixAndNonDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		suffix  string
		message string
	}{
		{suffix: "", message: "destination suffix"},
		{suffix: "/absolute", message: "destination suffix"},
		{suffix: "a/../b", message: "unsafe component"},
		{suffix: "file/child", message: "not a directory"},
	} {
		if _, err := validatedDestinationPath(root, test.suffix); err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("validatedDestinationPath(%q) error = %v", test.suffix, err)
		}
	}
}

func TestValidatedDestinationPathPreservesConsumedRootSpelling(t *testing.T) {
	root := t.TempDir() + string(filepath.Separator)
	path, err := validatedDestinationPath(root, "open-agent-workflow/ENGINEERING.md")
	if err != nil {
		t.Fatal(err)
	}
	want := root + string(filepath.Separator) + "open-agent-workflow" + string(filepath.Separator) + "ENGINEERING.md"
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestValidateOwnedDirectoriesAcceptsOnlyNamespacesAndCreatedTargetAncestors(t *testing.T) {
	root := t.TempDir()
	environment := Environment{Home: filepath.Join(root, "home"), ConfigHome: filepath.Join(root, "config"), StateHome: filepath.Join(root, "state")}
	coords, err := initializeCoordinates(environment, resolvedRequest{scope: "user"})
	if err != nil {
		t.Fatal(err)
	}
	state := installationState{
		scope: "user",
		directories: []string{
			coords.configDir, coords.stateDir, coords.installations, coords.projects,
			filepath.Join(environment.Home, ".claude"),
		},
		targets: []targetRecord{{
			id: "claude", path: filepath.Join(environment.Home, ".claude", "CLAUDE.md"),
			mode: "managed-block", checksum: "1:1", origin: "created-file",
		}},
	}
	if err := validateOwnedDirectories(state, coords); err != nil {
		t.Fatalf("validateOwnedDirectories(valid): %v", err)
	}
	state.directories = append(state.directories, filepath.Join(root, "outside"))
	if err := validateOwnedDirectories(state, coords); err == nil {
		t.Fatal("outside owned directory was accepted")
	}
	state.directories = []string{environment.Home + "/x/../.claude"}
	if err := validateOwnedDirectories(state, coords); err == nil {
		t.Fatal("unclean owned directory was accepted")
	}
}

func TestTargetDestinationRejectsUnsupportedCoordinates(t *testing.T) {
	root := t.TempDir()
	coords := coordinates{environment: Environment{Home: root, ConfigHome: root}}
	for _, test := range []struct {
		scope, id string
	}{
		{scope: "user", id: "cursor"},
		{scope: "unknown", id: "claude"},
		{scope: "user", id: "unknown"},
	} {
		if _, err := targetDestination(coords, test.scope, root, test.id); err == nil {
			t.Errorf("targetDestination(%q, %q) accepted", test.scope, test.id)
		}
	}
}

func TestWriteRendersOneLinePerResultEntry(t *testing.T) {
	var output bytes.Buffer
	if err := Write(Result{Lines: []string{"one", "two"}, Trailing: "partial"}, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "one\ntwo\npartial" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestWriteReturnsOutputFailure(t *testing.T) {
	if err := Write(Result{Lines: []string{"one"}}, failingWriter{}); err == nil {
		t.Fatal("Write() ignored output failure")
	}
}

func TestUnknownTargetHasNoRegistryPosition(t *testing.T) {
	if targetPosition("unknown") != 0 {
		t.Fatal("unknown target received a registry position")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
