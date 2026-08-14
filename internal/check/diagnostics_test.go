package check_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/check"
)

func TestExecuteReportsEmptyUserFixtureExactly(t *testing.T) {
	root := t.TempDir()
	result, err := check.Execute(testCatalog(t), testEnvironment(t, root), check.Request{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := []string{
		"version: 0.1.0",
		"scope: user",
		"targets: claude,codex,gemini,opencode",
		"provider superpowers: missing",
		"provider matt: missing",
		"provider ecc: missing",
		"target claude: missing (user, project)",
		"target codex: missing (user, project)",
		"target gemini: missing (user, project)",
		"target opencode: missing (user, project)",
		"installed claude: not-installed",
		"installed codex: not-installed",
		"installed gemini: not-installed",
		"installed opencode: not-installed",
	}
	if !reflect.DeepEqual(result.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", result.Lines, want)
	}
}

func TestExecuteRecognizesMattSkillLockCompatibilityIndicator(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "provider matt: missing")
	writeFixtureFile(t, filepath.Join(environment.Home, ".agents", ".skill-lock.json"), "{}")
	result, err = check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "provider matt: detected")
}

func TestExecuteRecognizesEverySuperpowersCompatibilityIndicator(t *testing.T) {
	indicators := []string{
		".claude/plugins/superpowers/skills/using-superpowers/SKILL.md",
		".codex/plugins/superpowers/skills/using-superpowers/SKILL.md",
		".claude/plugins/marketplaces/superpowers-marketplace/skills/using-superpowers/SKILL.md",
		".claude/plugins/cache/claude-plugins-official/superpowers/v1/skills/using-superpowers/SKILL.md",
		".claude/plugins/cache/superpowers-marketplace/superpowers/v1/skills/using-superpowers/SKILL.md",
		".codex/plugins/cache/openai-api-curated/superpowers/v1/skills/using-superpowers/SKILL.md",
	}
	for _, indicator := range indicators {
		t.Run(indicator, func(t *testing.T) {
			root := t.TempDir()
			environment := testEnvironment(t, root)
			writeFixtureFile(t, filepath.Join(environment.Home, filepath.FromSlash(indicator)), "indicator")
			result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			assertLine(t, result.Lines, "provider superpowers: detected")
		})
	}
}

func TestExecuteKeepsECCCompatibilityIndicatorNarrow(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	writeFixtureFile(t, filepath.Join(environment.Home, ".agents", "skills", "everything-claude-code", "SKILL.md"), "skill")
	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "provider ecc: missing")
	writeFixtureFile(t, filepath.Join(environment.Home, ".claude", "plugins", "marketplaces", "everything-claude-code", "plugins", "ecc", ".codex-plugin", "plugin.json"), "plugin")
	result, err = check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "provider ecc: detected")
}

func TestExecuteRecognizesCurrentECCCodexCache(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	writeFixtureFile(t, filepath.Join(environment.Home, ".codex", "plugins", "cache", "ecc", "ecc", "2.2.0", ".codex-plugin", "plugin.json"), "plugin")

	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "codex"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "provider ecc: detected")
}

func TestExecuteIgnoresHiddenSuperpowersVersionDirectories(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	writeFixtureFile(t, filepath.Join(environment.Home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", ".hidden", "skills", "using-superpowers", "SKILL.md"), "indicator")
	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "provider superpowers: missing")
}

func TestExecuteRecognizesSymlinkedProviderIndicatorFile(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	indicator := filepath.Join(root, "indicator")
	writeFixtureFile(t, indicator, "provider")
	link := filepath.Join(environment.Home, ".codex", "plugins", "superpowers", "skills", "using-superpowers", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(indicator, link); err != nil {
		t.Fatal(err)
	}
	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "provider superpowers: detected")
}

func TestExecuteReportsBuiltInProvidersAndProjectReadiness(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	writeFixtureFile(t, filepath.Join(environment.Home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", "test-build", "skills", "using-superpowers", "SKILL.md"), "superpowers")
	writeFixtureFile(t, filepath.Join(environment.Home, ".agents", ".skill-lock.json"), "{}")
	writeFixtureFile(t, filepath.Join(environment.Home, ".codex", "plugins", "ecc", ".codex-plugin", "plugin.json"), "{}")
	for _, executable := range []string{"claude", "codex", "gemini", "opencode"} {
		writeExecutable(t, filepath.Join(environment.Path, executable))
	}
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := check.Execute(testCatalog(t), environment, check.Request{Project: project})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	wantDiagnostics := []string{
		"provider superpowers: detected",
		"provider matt: detected",
		"provider ecc: detected",
		"target claude: detected (user, project)",
		"target codex: detected (user, project)",
		"target gemini: detected (user, project)",
		"target opencode: detected (user, project)",
		"target cursor: adapter-only (project)",
		"target windsurf: adapter-only (project)",
		"target cline: adapter-only (project)",
		"target roo: adapter-only (project)",
		"target copilot: adapter-only (project)",
	}
	if got := result.Lines[3 : 3+len(wantDiagnostics)]; !reflect.DeepEqual(got, wantDiagnostics) {
		t.Fatalf("diagnostics = %#v, want %#v", got, wantDiagnostics)
	}
}

func TestExecuteDetectsCoreConfigurationDirectories(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	for _, directory := range []string{
		filepath.Join(environment.Home, ".claude"),
		filepath.Join(environment.Home, ".codex"),
		filepath.Join(environment.Home, ".gemini"),
		filepath.Join(environment.ConfigHome, "opencode"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	result, err := check.Execute(testCatalog(t), environment, check.Request{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, target := range []string{"claude", "codex", "gemini", "opencode"} {
		assertLine(t, result.Lines, "target "+target+": detected (user, project)")
	}
}

func TestExecuteDoesNotResolveEmptyHomeRelativeToWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.Mkdir(".claude", 0o755); err != nil {
		t.Fatal(err)
	}
	environment := check.Environment{
		ConfigHome: filepath.Join(root, "config"),
		StateHome:  filepath.Join(root, "state"),
		Path:       filepath.Join(root, "bin"),
	}
	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	var checkError *check.Error
	if err == nil || !errors.As(err, &checkError) || checkError.Status != 64 {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "target claude: missing (user, project)")
}

func testEnvironment(t *testing.T, root string) check.Environment {
	t.Helper()
	environment := check.Environment{
		Home:       filepath.Join(root, "home"),
		ConfigHome: filepath.Join(root, "config"),
		StateHome:  filepath.Join(root, "state"),
		Path:       filepath.Join(root, "bin"),
	}
	for _, directory := range []string{environment.Home, environment.ConfigHome, environment.StateHome, environment.Path} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", directory, err)
		}
	}
	return environment
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	writeFixtureFile(t, path, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertLine(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, line := range lines {
		if line == want {
			return
		}
	}
	t.Fatalf("Lines = %#v, missing %q", lines, want)
}
