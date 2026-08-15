package bridgecli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
)

func TestStandaloneBridgeCommandSurface(t *testing.T) {
	for _, args := range [][]string{
		{"serve", "codex"}, {"hook", "codex"}, {"install", "codex", "--dry-run"},
		{"update", "codex", "--format=json"}, {"check", "codex"}, {"uninstall", "codex"},
	} {
		if _, err := parse(args); err != nil {
			t.Fatalf("parse(%v) = %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"core", "inspect"}, {"workflow", "exchange"}, {"serve", "claude"}, {"check", "codex", "--dry-run"},
	} {
		if _, err := parse(args); err == nil {
			t.Fatalf("parse(%v) succeeded", args)
		}
	}
}

func TestStandaloneHelpNamesNoWorkflowAuthority(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := RunWithContext(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	for _, expected := range []string{"oaw-bridge serve codex", "oaw-bridge install codex", "optional Machine Assurance", "does not select or run"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("help omits %q: %s", expected, stdout.String())
		}
	}
	for _, forbidden := range []string{"core_compile", "workflow_exchange", "Resource Lease", "Lifecycle Bundle"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Errorf("help contains retired authority %q", forbidden)
		}
	}
}

func TestStandaloneHookInjectsCurrentV3Context(t *testing.T) {
	input := `{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"workspace-write","tool_name":"mcp__oaw_codex_bridge__observe_profile","tool_input":{"profile":"project:team"}}`
	var stdout, stderr bytes.Buffer
	status := RunWithContext(context.Background(), []string{"hook", "codex"}, strings.NewReader(input), &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	var output struct {
		HookSpecificOutput struct {
			PermissionDecision string                     `json:"permissionDecision"`
			UpdatedInput       map[string]json.RawMessage `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	var hostContext codexbridge.HookContext
	if err := json.Unmarshal(output.HookSpecificOutput.UpdatedInput["_oaw_host_context"], &hostContext); err != nil {
		t.Fatal(err)
	}
	if output.HookSpecificOutput.PermissionDecision != "allow" ||
		hostContext.SchemaVersion != codexbridge.HookContextSchemaV3 || hostContext.BridgeProtocolVersion != codexbridge.BridgeProtocolVersion {
		t.Fatalf("Hook output = %#v", output)
	}
}
