package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/install"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestParseBridgeCommands(t *testing.T) {
	cases := []struct {
		args      []string
		operation string
	}{
		{args: []string{"serve", "codex"}, operation: "serve"},
		{args: []string{"hook", "codex"}, operation: "hook"},
		{args: []string{"install", "codex"}, operation: "install"},
		{args: []string{"update", "codex"}, operation: "update"},
		{args: []string{"check", "codex"}, operation: "check"},
		{args: []string{"uninstall", "codex"}, operation: "uninstall"},
	}
	for _, test := range cases {
		t.Run(test.operation, func(t *testing.T) {
			parsed, err := parseBridge(test.args)
			if err != nil || parsed.Operation != test.operation || parsed.Host != "codex" || parsed.Format != "text" {
				t.Fatalf("parseBridge(%v) = %#v, %v", test.args, parsed, err)
			}
		})
	}
}

func TestParseBridgeManagementOptions(t *testing.T) {
	parsed, err := parseBridge([]string{"install", "codex", "--dry-run", "--format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.DryRun || parsed.Format != "json" {
		t.Fatalf("parsed = %#v", parsed)
	}
	parsed, err = parseBridge([]string{"check", "codex", "--format=json"})
	if err != nil || parsed.Format != "json" {
		t.Fatalf("parsed = %#v, %v", parsed, err)
	}
}

func TestBridgeRejectsUnknownHostAndInvalidOptions(t *testing.T) {
	cases := [][]string{
		{"serve", "claude"},
		{"install", "codex", "--force"},
		{"uninstall", "codex", "--dry-run"},
		{"serve", "codex", "--format=json"},
		{"check", "codex", "--format", "yaml"},
		{"check", "codex", "--format=json", "--format=text"},
	}
	for _, args := range cases {
		if _, err := parseBridge(args); err == nil {
			t.Fatalf("parseBridge(%v) succeeded", args)
		}
	}
}

func TestBridgeRejectsUnknownHostAndLegacyRunner(t *testing.T) {
	for _, args := range [][]string{{"bridge", "serve", "claude"}, {"runtime"}, {"run"}} {
		if status := RunWithInput(args, strings.NewReader(""), io.Discard, io.Discard); status != 64 {
			t.Fatalf("%v status = %d", args, status)
		}
	}
}

func TestBridgeHookObservationWritesOfficialAllowEnvelope(t *testing.T) {
	input := `{"session_id":"session-a","turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}`
	stdout := &bytes.Buffer{}
	status := runBridgeHook(strings.NewReader(input), stdout, io.Discard)
	if status != 0 {
		t.Fatalf("status = %d", status)
	}
	var document struct {
		HookSpecificOutput struct {
			HookEventName      string                     `json:"hookEventName"`
			PermissionDecision string                     `json:"permissionDecision"`
			UpdatedInput       map[string]json.RawMessage `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.HookSpecificOutput.HookEventName != "PreToolUse" || document.HookSpecificOutput.PermissionDecision != "allow" || len(document.HookSpecificOutput.UpdatedInput["_oaw_host_context"]) == 0 {
		t.Fatalf("output = %s", stdout.Bytes())
	}
	var hostContext codexbridge.HookContext
	if err := json.Unmarshal(document.HookSpecificOutput.UpdatedInput["_oaw_host_context"], &hostContext); err != nil ||
		hostContext.SchemaVersion != codexbridge.HookContextSchemaV2 || hostContext.BridgeProtocolVersion != codexbridge.BridgeProtocolVersion {
		t.Fatalf("Hook Context = %#v, error = %v", hostContext, err)
	}
}

func TestBridgeHookValidLaterOperationWritesNoStdout(t *testing.T) {
	context := codexbridge.HookContext{
		SchemaVersion: codexbridge.HookContextSchemaV2, BridgeProtocolVersion: codexbridge.BridgeProtocolVersion,
		SessionID: "session-a", TurnID: "turn-a", ToolUseID: "tool-a", CWD: "/repo", Model: "gpt-test", PermissionMode: "default",
	}
	sessionDigest, cwdDigest, err := codexbridge.ContextDigestHeaders(context)
	if err != nil {
		t.Fatal(err)
	}
	handleBytes, err := json.Marshal(codexbridge.HostEvidenceHandle{
		Version: codexbridge.EvidenceHandleVersion, SessionDigest: sessionDigest, CWDDigest: cwdDigest, Token: "opaque",
	})
	if err != nil {
		t.Fatal(err)
	}
	handle := string(handleBytes)
	input := `{"session_id":"session-a","turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__core_inspect","tool_input":{"host_evidence_handle":` + handle + `}}`
	stdout := &bytes.Buffer{}
	if status := runBridgeHook(strings.NewReader(input), stdout, io.Discard); status != 0 {
		t.Fatalf("status = %d", status)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.Bytes())
	}
}

func TestBridgeHookSubagentStartRecordsCooperativeSessionEvidenceFromValidStdin(t *testing.T) {
	root := t.TempDir()
	stateHome := filepath.Join(root, "state")
	dataHome := filepath.Join(root, "data")
	projectRoot := filepath.Join(root, "project")
	for _, directory := range []string{stateHome, dataHome, projectRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Chdir(projectRoot)
	input := `{"session_id":"session-private-a","transcript_path":"/private/transcript.jsonl","turn_id":"turn-a","cwd":"` + projectRoot + `","hook_event_name":"SubagentStart","model":"gpt-test","permission_mode":"default","agent_id":"agent-private-a","agent_type":"reviewer"}`
	stdout := &bytes.Buffer{}
	if status := runBridgeHook(strings.NewReader(input), stdout, io.Discard); status != 0 {
		t.Fatalf("status = %d", status)
	}
	if stdout.Len() != 0 {
		t.Fatalf("SubagentStart wrote context: %q", stdout.Bytes())
	}
	environment, err := bridgeEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	store, err := codexbridge.NewSessionFeatureEvidenceStore(codexbridge.SessionFeatureEvidenceOptions{
		Root: filepath.Join(environment.StateRoot, "features"), TTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := store.ObserveFeatures(codexbridge.HookContext{SessionID: "session-private-a", CWD: projectRoot})
	if len(result.Diagnostics) != 0 || len(result.Observations) != 1 || result.Observations[0].Feature != host.FeatureChildDelegation ||
		!strings.HasPrefix(result.Observations[0].EvidenceReference, "evidence://codex/cooperative-subagent-start/") {
		t.Fatalf("result = %#v", result)
	}
}

type recordingCodexRunner struct {
	Commands [][]string
}

func (r *recordingCodexRunner) Run(_ context.Context, arguments ...string) (install.CLIResult, error) {
	r.Commands = append(r.Commands, slices.Clone(arguments))
	switch strings.Join(arguments, " ") {
	case "plugin list --json":
		return install.CLIResult{Stdout: []byte(`{"installed":[]}`)}, nil
	case "plugin marketplace list --json":
		return install.CLIResult{Stdout: []byte(`{"marketplaces":[]}`)}, nil
	default:
		return install.CLIResult{}, nil
	}
}

func testBridgeInstallEnvironment(t *testing.T) install.Environment {
	t.Helper()
	root := t.TempDir()
	stateHome := filepath.Join(root, "state")
	dataHome := filepath.Join(root, "data")
	projectRoot := filepath.Join(root, "project")
	for _, directory := range []string{stateHome, dataHome, projectRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	environment, err := install.NewEnvironment(stateHome, dataHome, "codex", projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	return environment
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test binary"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestBridgeHookMalformedInputFailsClosed(t *testing.T) {
	stdout := &bytes.Buffer{}
	if status := runBridgeHook(strings.NewReader(`{"hook_event_name":"wrong"}`), stdout, io.Discard); status != 0 {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(stdout.String(), `"permissionDecision":"deny"`) || strings.Contains(stdout.String(), "updatedInput") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestBridgeCheckTextRedactsExactPaths(t *testing.T) {
	result := install.CheckResult{
		SchemaVersion: install.ManagementResultSchemaV1,
		Files: []install.FileStatus{{
			Path: "/private/user/state/secret", PathDigest: "abc123", Status: "clean",
		}},
		CurrentSessionLoaded: false,
	}
	stdout := &bytes.Buffer{}
	if err := writeBridgeCheck(result, "text", stdout); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "/private/user") || !strings.Contains(stdout.String(), "abc123") {
		t.Fatalf("text output = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "proof_scope: installation-integrity") || !strings.Contains(stdout.String(), "live_protocol_proof: false") {
		t.Fatalf("installation check did not state its evidence boundary: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "version_evidence") || strings.Contains(stdout.String(), "oaw.codex-bridge/v2") {
		t.Fatalf("installation check claimed live protocol evidence: %q", stdout.String())
	}
	stdout.Reset()
	if err := writeBridgeCheck(result, "json", stdout); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "/private/user/state/secret") {
		t.Fatalf("JSON output omitted machine-readable path: %q", stdout.String())
	}
}

func TestBridgeEnvironmentUsesXDGRootsAndCurrentProject(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Chdir(root)
	environment, err := bridgeEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if environment.StateRoot != filepath.Join(root, "state", "open-agent-workflow", "codex-bridge") ||
		environment.DataRoot != filepath.Join(root, "data", "open-agent-workflow", "codex-bridge") ||
		environment.ProjectRoot != physicalRoot || environment.CodexBinary != "codex" {
		t.Fatalf("environment = %#v", environment)
	}
}

func TestExecuteBridgeInstallDryRunUsesExplicitBinary(t *testing.T) {
	environment := testBridgeInstallEnvironment(t)
	binary := filepath.Join(environment.ProjectRoot, "oaw")
	writeExecutable(t, binary)
	runner := &recordingCodexRunner{}
	result, check, err := executeBridgeManagement(context.Background(), bridgeCommand{Operation: "install", Host: "codex", DryRun: true}, environment, runner, binary, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation != "install" || result.Changed || check != nil || len(runner.Commands) != 0 {
		t.Fatalf("result = %#v, check = %#v, commands = %#v", result, check, runner.Commands)
	}
}
