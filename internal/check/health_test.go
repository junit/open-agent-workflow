package check_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/check"
)

const (
	testBeginMarker = "<!-- BEGIN OPEN AGENT WORKFLOW -->"
	testEndMarker   = "<!-- END OPEN AGENT WORKFLOW -->"
)

func TestExecuteReportsCleanAndDriftedManagedInstallation(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	fixture := installUserManagedFixture(t, environment, "claude")

	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute(clean) error = %v", err)
	}
	assertLine(t, result.Lines, "installed claude: clean")

	if err := os.WriteFile(fixture.policyPath, []byte("drifted policy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute(policy drift) error = %v", err)
	}
	assertLine(t, result.Lines, "installed claude: drift")

	if err := os.WriteFile(fixture.policyPath, []byte("canonical policy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.targetPath, []byte(testBeginMarker+"\nchanged\n"+testEndMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute(target drift) error = %v", err)
	}
	assertLine(t, result.Lines, "installed claude: drift")
}

func TestExecuteCollapsesMalformedStateToDiagnostic(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	fixture := installUserManagedFixture(t, environment, "claude")
	if err := os.WriteFile(fixture.statePath, []byte("format\t2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "installed claude: invalid-state")
}

func TestExecuteFallsBackToUntrackedTargetHealth(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	installUserManagedFixture(t, environment, "claude")
	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "codex"})
	if err != nil {
		t.Fatalf("Execute(missing) error = %v", err)
	}
	assertLine(t, result.Lines, "installed codex: not-installed")

	writeFixtureFile(t, filepath.Join(environment.Home, ".codex", "AGENTS.md"), testBeginMarker+"\nuntracked\n"+testEndMarker+"\n")
	result, err = check.Execute(testCatalog(t), environment, check.Request{Targets: "codex"})
	if err != nil {
		t.Fatalf("Execute(untracked) error = %v", err)
	}
	assertLine(t, result.Lines, "installed codex: drift")
}

func TestExecuteReportsUntrackedOwnedProjectFileAsDrift(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(project, ".cursor", "rules", "open-agent-workflow.mdc"), "untracked\n")
	result, err := check.Execute(testCatalog(t), environment, check.Request{Project: project, Targets: "cursor"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "installed cursor: drift")
}

func TestExecuteReportsCleanProjectOwnedFile(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	physicalProject, err := filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(environment.ConfigHome, "open-agent-workflow", "ENGINEERING.md")
	targetPath := filepath.Join(physicalProject, ".cursor", "rules", "open-agent-workflow.mdc")
	writeFixtureFile(t, policyPath, "canonical policy\n")
	writeFixtureFile(t, targetPath, "owned adapter\n")
	identity := strings.Replace(systemChecksumText(t, physicalProject), ":", "-", 1)
	statePath := filepath.Join(environment.StateHome, "open-agent-workflow", "installations", "projects", identity+".state")
	state := fmt.Sprintf(
		"format\t1\nversion\t0.1.0\nscope\tproject\nproject\t%s\npolicy\t%s\t%s\ntarget\tcursor\t%s\towned-file\t%s\texisting-file\n",
		physicalProject, policyPath, systemChecksum(t, policyPath), targetPath, systemChecksum(t, targetPath),
	)
	writeFixtureFile(t, statePath, state)
	result, err := check.Execute(testCatalog(t), environment, check.Request{Project: project, Targets: "cursor"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertLine(t, result.Lines, "installed cursor: clean")
}

func TestExecuteRejectsSymlinkedTargetCoordinate(t *testing.T) {
	root := t.TempDir()
	environment := testEnvironment(t, root)
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(environment.Home, ".claude")); err != nil {
		t.Fatal(err)
	}
	result, err := check.Execute(testCatalog(t), environment, check.Request{Targets: "claude"})
	var checkError *check.Error
	if err == nil || !strings.Contains(err.Error(), "destination path contains a symlink") || !errors.As(err, &checkError) || checkError.Status != 65 {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Trailing != "installed claude: " {
		t.Fatalf("Trailing = %q", result.Trailing)
	}
}

type userManagedFixture struct {
	policyPath string
	targetPath string
	statePath  string
}

func installUserManagedFixture(t *testing.T, environment check.Environment, targetID string) userManagedFixture {
	t.Helper()
	policyPath := filepath.Join(environment.ConfigHome, "open-agent-workflow", "ENGINEERING.md")
	writeFixtureFile(t, policyPath, "canonical policy\n")
	var targetPath string
	switch targetID {
	case "claude":
		targetPath = filepath.Join(environment.Home, ".claude", "CLAUDE.md")
	case "codex":
		targetPath = filepath.Join(environment.Home, ".codex", "AGENTS.md")
	default:
		t.Fatalf("unsupported fixture target %q", targetID)
	}
	block := testBeginMarker + "\nfixture\n" + testEndMarker + "\n"
	writeFixtureFile(t, targetPath, "user content\n"+block)
	blockPath := filepath.Join(t.TempDir(), "block")
	writeFixtureFile(t, blockPath, block)
	statePath := filepath.Join(environment.StateHome, "open-agent-workflow", "installations", "user.state")
	state := fmt.Sprintf(
		"format\t1\nversion\t0.1.0\nscope\tuser\npolicy\t%s\t%s\ntarget\t%s\t%s\tmanaged-block\t%s\texisting-file\n",
		policyPath, systemChecksum(t, policyPath), targetID, targetPath, systemChecksum(t, blockPath),
	)
	writeFixtureFile(t, statePath, state)
	if err := os.Chmod(statePath, 0o600); err != nil {
		t.Fatal(err)
	}
	return userManagedFixture{policyPath: policyPath, targetPath: targetPath, statePath: statePath}
}

func systemChecksum(t *testing.T, path string) string {
	t.Helper()
	output, err := exec.Command("cksum", path).Output()
	if err != nil {
		t.Fatalf("cksum %s: %v", path, err)
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 {
		t.Fatalf("cksum output = %q", output)
	}
	return fields[0] + ":" + fields[1]
}

func systemChecksumText(t *testing.T, value string) string {
	t.Helper()
	command := exec.Command("cksum")
	command.Stdin = strings.NewReader(value)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("cksum text: %v", err)
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 {
		t.Fatalf("cksum output = %q", output)
	}
	return fields[0] + ":" + fields[1]
}
