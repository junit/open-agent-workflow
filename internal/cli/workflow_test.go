package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
)

func TestWorkflowExchangeEmitsOneCanonicalResult(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "workflows")
	command := coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1,
		Kind:          coordinator.CommandInspect,
		WorkflowID:    "workflow-missing",
	}
	raw, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := RunWithInput(
		[]string{"workflow", "exchange", "--state-root", stateRoot},
		bytes.NewReader(raw),
		&stdout,
		&stderr,
	)
	if status != 65 {
		t.Fatalf("workflow exchange status = %d, want 65; stderr=%q", status, stderr.String())
	}
	assertCanonicalWorkflowResult(t, stdout.Bytes(), coordinator.ResultRejected, "WORKFLOW_NOT_FOUND")
	if !strings.Contains(stderr.String(), "WORKFLOW_NOT_FOUND") {
		t.Fatalf("workflow exchange stderr = %q", stderr.String())
	}
}

func TestWorkflowExchangeInvalidCommandReturnsCanonicalRejection(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := RunWithInput(
		[]string{"workflow", "exchange", "--state-root", filepath.Join(t.TempDir(), "workflows")},
		strings.NewReader(`{"schema_version":"oaw.runtime/v1","kind":"START"}`),
		&stdout,
		&stderr,
	)
	if status != 65 {
		t.Fatalf("workflow exchange status = %d, want 65; stderr=%q", status, stderr.String())
	}
	assertCanonicalWorkflowResult(t, stdout.Bytes(), coordinator.ResultRejected, "SCHEMA_UNSUPPORTED")
	if !strings.Contains(stderr.String(), "SCHEMA_UNSUPPORTED") {
		t.Fatalf("workflow exchange stderr = %q", stderr.String())
	}
}

func TestRuntimeRemovedRejectsWithoutCreatingState(t *testing.T) {
	assertRemovedCommand(t, []string{"runtime", "exchange"})
}

func TestRunRemovedRejectsWithoutCreatingState(t *testing.T) {
	assertRemovedCommand(t, []string{"run", "--host", "codex"})
}

func TestWorkflowExchangeIsOnlyExecutionCommandInUsage(t *testing.T) {
	text := usage()
	if !strings.Contains(text, "oaw workflow exchange") {
		t.Fatalf("usage omits workflow exchange: %q", text)
	}
	for _, retired := range []string{"oaw runtime exchange", "oaw run --host"} {
		if strings.Contains(text, retired) {
			t.Fatalf("usage contains retired command %q: %q", retired, text)
		}
	}
}

func TestTopLevelHelpPublishesCoordinatorWithoutModelLaunchOptions(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := RunWithInput([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "oaw workflow exchange") {
		t.Fatalf("top-level help status/stdout/stderr = %d/%q/%q", status, stdout.String(), stderr.String())
	}
	for _, retired := range []string{"oaw run", "oaw runtime", "--host codex", "--sandbox", "execution-root", "private HOME"} {
		if strings.Contains(stdout.String(), retired) {
			t.Fatalf("top-level help contains retired execution surface %q: %q", retired, stdout.String())
		}
	}
}

func assertRemovedCommand(t *testing.T, command []string) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "workflows")
	args := append(append([]string{}, command...), "--state-root", stateRoot)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := RunWithInput(args, strings.NewReader(`{}`), &stdout, &stderr)
	if status != 64 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "INVALID_ARGUMENT") {
		t.Fatalf("removed command status/stdout/stderr = %d/%q/%q", status, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(stateRoot); !os.IsNotExist(err) {
		t.Fatalf("removed command created state at %q: %v", stateRoot, err)
	}
}

func assertCanonicalWorkflowResult(t *testing.T, raw []byte, kind coordinator.ResultKind, diagnosticCode string) {
	t.Helper()
	var result coordinator.Result
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode Workflow Result %q: %v", raw, err)
	}
	if result.Kind != kind || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != diagnosticCode {
		t.Fatalf("Workflow Result = %#v", result)
	}
	canonical, err := coordinator.EncodeResult(result)
	if err != nil {
		t.Fatalf("re-encode Workflow Result: %v", err)
	}
	if !bytes.Equal(raw, canonical) {
		t.Fatalf("Workflow Result is not canonical\n got: %s\nwant: %s", raw, canonical)
	}
}
