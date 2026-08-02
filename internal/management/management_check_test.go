package management

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
)

func TestCheckManagementDomainExercisesDiagnosticsAndHealth(t *testing.T) {
	catalog, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("empty user", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		result, err := Check(catalog, fixture.environment, CheckRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{
			"provider superpowers: missing",
			"provider matt: missing",
			"provider ecc: missing",
			"target claude: missing (user, project)",
			"installed claude: not-installed",
		} {
			if !resultContainsLine(result, expected) {
				t.Fatalf("result is missing %q: %#v", expected, result.Lines)
			}
		}
	})

	t.Run("detected providers and project targets", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		writePrepareFile(t, filepath.Join(fixture.environment.Home, ".codex", "plugins", "superpowers", "skills", "using-superpowers", "SKILL.md"), []byte("skill\n"), 0o644)
		for _, skill := range []string{"to-spec", "to-tickets", "tdd", "diagnosing-bugs"} {
			writePrepareFile(t, filepath.Join(fixture.environment.Home, ".agents", "skills", skill, "SKILL.md"), []byte("skill\n"), 0o644)
		}
		writePrepareFile(t, filepath.Join(fixture.environment.Home, ".agents", "skills", "everything-claude-code", "SKILL.md"), []byte("skill\n"), 0o644)
		for _, executable := range []string{"claude", "codex", "gemini", "opencode"} {
			path := filepath.Join(fixture.environment.Path, executable)
			writePrepareFile(t, path, []byte("#!/bin/sh\nexit 0\n"), 0o755)
		}
		project := filepath.Join(fixture.root, "project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		result, err := Check(catalog, fixture.environment, CheckRequest{Project: project})
		if err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{
			"provider superpowers: detected",
			"provider matt: detected",
			"provider ecc: detected",
			"target claude: detected (user, project)",
			"target cursor: adapter-only (project)",
			"installed cursor: not-installed",
		} {
			if !resultContainsLine(result, expected) {
				t.Fatalf("result is missing %q: %#v", expected, result.Lines)
			}
		}
	})

	t.Run("clean and drifted managed state", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, prepared)
		result, err := Check(catalog, fixture.environment, CheckRequest{Targets: "claude"})
		if err != nil || !resultContainsLine(result, "installed claude: clean") {
			t.Fatalf("clean Check() result=%#v error=%v", result, err)
		}
		writePrepareFile(t, prepared.policyAction.destination, []byte("drift\n"), 0o600)
		result, err = Check(catalog, fixture.environment, CheckRequest{Targets: "claude"})
		if err != nil || !resultContainsLine(result, "installed claude: drift") {
			t.Fatalf("drift Check() result=%#v error=%v", result, err)
		}
		writePrepareFile(t, prepared.policyAction.destination, prepared.policyAction.data, 0o600)
		writePrepareFile(t, prepared.targetActions[0].destination, []byte(beginMarker+"\ntarget drift\n"+endMarker+"\n"), 0o644)
		result, err = Check(catalog, fixture.environment, CheckRequest{Targets: "claude"})
		if err != nil || !resultContainsLine(result, "installed claude: drift") {
			t.Fatalf("target drift Check() result=%#v error=%v", result, err)
		}
		writePrepareFile(t, prepared.targetActions[0].destination, prepared.targetActions[0].data, 0o644)
		state := parsePreparedState(t, prepared.stateActions[0])
		state.targets[0].path = filepath.Join(fixture.environment.Home, ".claude", "wrong.md")
		stateBytes, serializeErr := serializeInstallState(state)
		if serializeErr != nil {
			t.Fatal(serializeErr)
		}
		writePrepareFile(t, prepared.stateActions[0].destination, stateBytes, 0o600)
		result, err = Check(catalog, fixture.environment, CheckRequest{Targets: "claude"})
		if err != nil || !resultContainsLine(result, "installed claude: invalid-state") {
			t.Fatalf("target mismatch Check() result=%#v error=%v", result, err)
		}
	})

	t.Run("clean project owned file", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		project := filepath.Join(fixture.root, "project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Project: project, Targets: "cursor"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, prepared)
		result, err := Check(catalog, fixture.environment, CheckRequest{Project: project, Targets: "cursor"})
		if err != nil || !resultContainsLine(result, "installed cursor: clean") {
			t.Fatalf("Check() result=%#v error=%v", result, err)
		}
		writePrepareFile(t, prepared.targetActions[0].destination, []byte("drift\n"), 0o644)
		result, err = Check(catalog, fixture.environment, CheckRequest{Project: project, Targets: "cursor"})
		if err != nil || !resultContainsLine(result, "installed cursor: drift") {
			t.Fatalf("owned drift Check() result=%#v error=%v", result, err)
		}
		writePrepareFile(t, prepared.targetActions[0].destination, prepared.targetActions[0].data, 0o644)
		result, err = Check(catalog, fixture.environment, CheckRequest{Project: project, Targets: "roo"})
		if err != nil || !resultContainsLine(result, "installed roo: not-installed") {
			t.Fatalf("untracked missing Check() result=%#v error=%v", result, err)
		}
		writePrepareFile(t, filepath.Join(project, ".roo", "rules", "open-agent-workflow.md"), []byte("foreign\n"), 0o644)
		result, err = Check(catalog, fixture.environment, CheckRequest{Project: project, Targets: "roo"})
		if err != nil || !resultContainsLine(result, "installed roo: drift") {
			t.Fatalf("untracked owned Check() result=%#v error=%v", result, err)
		}
	})

	t.Run("invalid state and untracked target", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		statePath := filepath.Join(fixture.environment.StateHome, "open-agent-workflow", "installations", "user.state")
		writePrepareFile(t, statePath, []byte("format\t2\n"), 0o600)
		result, err := Check(catalog, fixture.environment, CheckRequest{Targets: "claude"})
		if err != nil || !resultContainsLine(result, "installed claude: invalid-state") {
			t.Fatalf("Check() result=%#v error=%v", result, err)
		}
		if err := os.Remove(statePath); err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md"), []byte(beginMarker+"\nuntracked\n"+endMarker+"\n"), 0o644)
		result, err = Check(catalog, fixture.environment, CheckRequest{Targets: "claude"})
		if err != nil || !resultContainsLine(result, "installed claude: drift") {
			t.Fatalf("Check() result=%#v error=%v", result, err)
		}
	})
}

func TestCheckManagementDomainRejectsInvalidRequestsAndPreservesPartialResult(t *testing.T) {
	catalog, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	fixture := newPrepareFixture(t)
	for name, request := range map[string]CheckRequest{
		"whitespace":       {Targets: "claude, codex"},
		"empty member":     {Targets: "claude,,codex"},
		"unknown":          {Targets: "vscode"},
		"unsupported user": {Targets: "cursor"},
		"missing project":  {Project: filepath.Join(fixture.root, "missing")},
		"control project":  {Project: fixture.root + "\nbad"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Check(catalog, fixture.environment, request); err == nil {
				t.Fatalf("Check(%#v) succeeded", request)
			}
		})
	}

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(fixture.environment.Home, ".claude")); err != nil {
		t.Fatal(err)
	}
	result, err := Check(catalog, fixture.environment, CheckRequest{Targets: "claude"})
	if err == nil || result.Trailing != "installed claude: " {
		t.Fatalf("partial Check() result=%#v error=%v", result, err)
	}
}

func resultContainsLine(result Result, expected string) bool {
	for _, line := range result.Lines {
		if line == expected {
			return true
		}
	}
	return false
}
