package management

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
		{name: "relative config", environment: Environment{Home: base.Home, ConfigHome: "relative", StateHome: base.StateHome}, message: "root must be an absolute path"},
		{name: "relative state", environment: Environment{Home: base.Home, ConfigHome: base.ConfigHome, StateHome: "relative"}, message: "root must be an absolute path"},
		{name: "control home", environment: Environment{Home: base.Home + "\nbad", ConfigHome: base.ConfigHome, StateHome: base.StateHome}, message: "contains control characters"},
		{name: "control config", environment: Environment{Home: base.Home, ConfigHome: base.ConfigHome + "\nbad", StateHome: base.StateHome}, message: "contains control characters"},
		{name: "control state", environment: Environment{Home: base.Home, ConfigHome: base.ConfigHome, StateHome: base.StateHome + "\nbad"}, message: "contains control characters"},
		{name: "config symlink", environment: base, setup: func(t *testing.T) {
			link := filepath.Join(base.ConfigHome, "open-agent-workflow")
			if err := os.Symlink(t.TempDir(), link); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Remove(link) })
		}, message: "destination path contains a symlink"},
		{name: "state symlink", environment: base, setup: func(t *testing.T) {
			link := filepath.Join(base.StateHome, "open-agent-workflow")
			if err := os.Symlink(t.TempDir(), link); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Remove(link) })
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
		{suffix: "a//b", message: "unsafe component"},
		{suffix: "a/./b", message: "unsafe component"},
		{suffix: "a/../b", message: "unsafe component"},
		{suffix: "../b", message: "unsafe component"},
		{suffix: "a/\nb", message: "control characters"},
		{suffix: "file/child", message: "not a directory"},
	} {
		if _, err := validatedDestinationPath(root, test.suffix); err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("validatedDestinationPath(%q) error = %v", test.suffix, err)
		}
	}
}

func TestValidatedDestinationPathPreservesConsumedRootSpelling(t *testing.T) {
	root := t.TempDir() + string(filepath.Separator)
	path, err := validatedDestinationPath(root, "open-agent-workflow/POLICY.md")
	if err != nil {
		t.Fatal(err)
	}
	want := root + string(filepath.Separator) + "open-agent-workflow" + string(filepath.Separator) + "POLICY.md"
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestStateActionRelativeSuffixAcceptsCanonicalEquivalentRootSpelling(t *testing.T) {
	root := t.TempDir() + string(filepath.Separator)
	destination := filepath.Join(root, "open-agent-workflow", "backups", "operation")

	suffix, err := stateActionRelativeSuffix(root, destination)
	if err != nil {
		t.Fatal(err)
	}
	if want := "open-agent-workflow/backups/operation"; suffix != want {
		t.Fatalf("suffix = %q, want %q", suffix, want)
	}
	outside := filepath.Join(filepath.Dir(filepath.Clean(root)), "outside")
	if _, err := stateActionRelativeSuffix(root, outside); err == nil || !strings.Contains(err.Error(), "escapes its allowed root") {
		t.Fatalf("outside destination error = %v", err)
	}
	alias := filepath.Clean(root) + string(filepath.Separator) + "child" + string(filepath.Separator) + ".." + string(filepath.Separator) + "artifact"
	if _, err := stateActionRelativeSuffix(root, alias); err == nil || !strings.Contains(err.Error(), "does not match its allowed root") {
		t.Fatalf("lexical alias error = %v", err)
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
			filepath.Join(environment.Home, ".claude", "skills"),
			filepath.Join(environment.Home, ".claude", "skills", "oaw"),
		},
		targets: []targetRecord{
			{
				id: "claude", artifact: routerArtifactID,
				path: filepath.Join(environment.Home, ".claude", "CLAUDE.md"),
				mode: "managed-block", checksum: "1:1", origin: "created-file",
			},
			{
				id: "claude", artifact: nativeEntrypointArtifactID,
				path: filepath.Join(environment.Home, ".claude", "skills", "oaw", "SKILL.md"),
				mode: "owned-file", checksum: "2:2", origin: "created-file",
			},
		},
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

func TestArtifactDestinationRejectsUnsupportedCoordinates(t *testing.T) {
	root := t.TempDir()
	coords := coordinates{environment: Environment{Home: root, ConfigHome: root}}
	for _, test := range []struct {
		scope, id string
	}{
		{scope: "user", id: "cursor"},
		{scope: "unknown", id: "claude"},
		{scope: "user", id: "unknown"},
	} {
		if _, err := artifactDestination(coords, test.scope, root, test.id, routerArtifactID); err == nil {
			t.Errorf("artifactDestination(%q, %q) accepted", test.scope, test.id)
		}
	}
}

func TestArtifactDestinationUsesVerifiedHostCoordinates(t *testing.T) {
	root := t.TempDir()
	environment := Environment{
		Home:       filepath.Join(root, "home"),
		ConfigHome: filepath.Join(root, "config"),
		StateHome:  filepath.Join(root, "state"),
	}
	for _, directory := range []string{environment.Home, environment.ConfigHome, environment.StateHome} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	coords := coordinates{environment: environment}
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, scope, target, artifact, want string
	}{
		{name: "Claude user router", scope: "user", target: "claude", artifact: routerArtifactID, want: filepath.Join(environment.Home, ".claude", "CLAUDE.md")},
		{name: "Claude user Skill", scope: "user", target: "claude", artifact: nativeEntrypointArtifactID, want: filepath.Join(environment.Home, ".claude", "skills", "oaw", "SKILL.md")},
		{name: "Codex user router", scope: "user", target: "codex", artifact: routerArtifactID, want: filepath.Join(environment.Home, ".codex", "AGENTS.md")},
		{name: "Codex user Skill", scope: "user", target: "codex", artifact: nativeEntrypointArtifactID, want: filepath.Join(environment.Home, ".agents", "skills", "oaw", "SKILL.md")},
		{name: "Codex user metadata", scope: "user", target: "codex", artifact: nativePolicyArtifactID, want: filepath.Join(environment.Home, ".agents", "skills", "oaw", "agents", "openai.yaml")},
		{name: "Gemini user command", scope: "user", target: "gemini", artifact: nativeEntrypointArtifactID, want: filepath.Join(environment.Home, ".gemini", "commands", "oaw.toml")},
		{name: "OpenCode user router", scope: "user", target: "opencode", artifact: routerArtifactID, want: filepath.Join(environment.ConfigHome, "opencode", "AGENTS.md")},
		{name: "OpenCode user command", scope: "user", target: "opencode", artifact: nativeEntrypointArtifactID, want: filepath.Join(environment.ConfigHome, "opencode", "commands", "oaw.md")},
		{name: "Claude project Skill", scope: "project", target: "claude", artifact: nativeEntrypointArtifactID, want: filepath.Join(project, ".claude", "skills", "oaw", "SKILL.md")},
		{name: "Codex project Skill", scope: "project", target: "codex", artifact: nativeEntrypointArtifactID, want: filepath.Join(project, ".agents", "skills", "oaw", "SKILL.md")},
		{name: "Codex project metadata", scope: "project", target: "codex", artifact: nativePolicyArtifactID, want: filepath.Join(project, ".agents", "skills", "oaw", "agents", "openai.yaml")},
		{name: "Gemini project command", scope: "project", target: "gemini", artifact: nativeEntrypointArtifactID, want: filepath.Join(project, ".gemini", "commands", "oaw.toml")},
		{name: "OpenCode project command", scope: "project", target: "opencode", artifact: nativeEntrypointArtifactID, want: filepath.Join(project, ".opencode", "commands", "oaw.md")},
		{name: "Cursor project Skill", scope: "project", target: "cursor", artifact: nativeEntrypointArtifactID, want: filepath.Join(project, ".cursor", "skills", "oaw", "SKILL.md")},
		{name: "Windsurf project workflow", scope: "project", target: "windsurf", artifact: nativeEntrypointArtifactID, want: filepath.Join(project, ".windsurf", "workflows", "oaw.md")},
		{name: "Cline project Skill", scope: "project", target: "cline", artifact: nativeEntrypointArtifactID, want: filepath.Join(project, ".cline", "skills", "oaw", "SKILL.md")},
		{name: "Roo project command", scope: "project", target: "roo", artifact: nativeEntrypointArtifactID, want: filepath.Join(project, ".roo", "commands", "oaw.md")},
		{name: "Copilot project Skill", scope: "project", target: "copilot", artifact: nativeEntrypointArtifactID, want: filepath.Join(project, ".github", "skills", "oaw", "SKILL.md")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := artifactDestination(coords, tt.scope, project, tt.target, tt.artifact)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("artifact destination = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestArtifactDestinationRejectsUnsupportedUnknownAndSymlinkedCoordinates(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(config, 0o755); err != nil {
		t.Fatal(err)
	}
	coords := coordinates{environment: Environment{Home: home, ConfigHome: config}}
	for name, test := range map[string]struct {
		scope, target, artifact string
	}{
		"project-only user target": {scope: "user", target: "cursor", artifact: nativeEntrypointArtifactID},
		"unknown target":           {scope: "project", target: "missing", artifact: routerArtifactID},
		"unknown artifact":         {scope: "user", target: "claude", artifact: "missing"},
		"unknown scope":            {scope: "machine", target: "claude", artifact: routerArtifactID},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := artifactDestination(coords, test.scope, root, test.target, test.artifact); err == nil {
				t.Fatalf("artifactDestination(%q, %q, %q) succeeded", test.scope, test.target, test.artifact)
			}
		})
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, ".claude", "skills")); err != nil {
		t.Fatal(err)
	}
	if _, err := artifactDestination(coords, "user", "", "claude", nativeEntrypointArtifactID); err == nil || !strings.Contains(err.Error(), "contains a symlink") {
		t.Fatalf("symlinked native path error = %v", err)
	}
}

func TestWriteRendersOneLinePerResultEntry(t *testing.T) {
	var output bytes.Buffer
	if err := WriteResult(Result{Lines: []string{"one", "two"}, Trailing: "partial"}, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "one\ntwo\npartial" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestWriteReturnsOutputFailure(t *testing.T) {
	if err := WriteResult(Result{Lines: []string{"one"}}, failingWriter{}); err == nil {
		t.Fatal("Write() ignored output failure")
	}
}

func TestWriteReturnsTrailingOutputFailure(t *testing.T) {
	if err := WriteResult(Result{Trailing: "partial"}, failingWriter{}); err == nil {
		t.Fatal("WriteResult() ignored trailing output failure")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
