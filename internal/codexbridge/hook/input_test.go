package hook

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
)

func TestParsePreToolUseRequiresExactEventAndIdentity(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"session_id":"s","turn_id":"t","tool_use_id":"u","cwd":"/repo","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}`),
		[]byte(`{"session_id":"s","turn_id":"t","tool_use_id":"u","cwd":"/repo","hook_event_name":"PostToolUse","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}`),
		[]byte(`{"session_id":"s","tool_use_id":"u","cwd":"/repo","hook_event_name":"PreToolUse","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}`),
	} {
		if _, err := ParsePreToolUse(raw); codexbridge.Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestParsePreToolUseRejectsWrongToolAndMalformedInput(t *testing.T) {
	valid := validInput("mcp__oaw_codex_bridge__observe_current")
	for _, raw := range []string{
		`{"session_id":"s","turn_id":"t","tool_use_id":"u","cwd":"/repo","hook_event_name":"PreToolUse","model":"m","permission_mode":"default","tool_name":"Bash","tool_input":{}}`,
		`{"session_id":"s","turn_id":"t","tool_use_id":"u","cwd":"/repo","hook_event_name":"PreToolUse","model":"m","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":[]}`,
		`{"session_id":"s\u0000","turn_id":"t","tool_use_id":"u","cwd":"/repo","hook_event_name":"PreToolUse","model":"m","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}`,
		`{"session_id":"s","turn_id":"t","tool_use_id":"u","cwd":"/repo","hook_event_name":"PreToolUse","model":"m","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{},"extra":true}`,
	} {
		if _, err := ParsePreToolUse([]byte(raw)); codexbridge.Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
			t.Fatalf("raw=%s error=%v", raw, err)
		}
	}
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePreToolUse(append(encoded, bytesOfSpace...)); err != nil {
		t.Fatalf("valid input with trailing whitespace rejected: %v", err)
	}
	if _, err := ParsePreToolUse([]byte(strings.Repeat("x", maxHookInputBytes+1))); codexbridge.Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
		t.Fatalf("oversized input error = %v", err)
	}
}

func TestHookInputV2AcceptsOnlyFourBridgeTools(t *testing.T) {
	allowed := map[string]codexbridge.Operation{
		"mcp__oaw_codex_bridge__observe_current":   codexbridge.OperationObserveCurrent,
		"mcp__oaw_codex_bridge__core_inspect":      codexbridge.OperationCoreInspect,
		"mcp__oaw_codex_bridge__core_compile":      codexbridge.OperationCoreCompile,
		"mcp__oaw_codex_bridge__workflow_exchange": codexbridge.OperationWorkflowExchange,
	}
	for name, wantOperation := range allowed {
		if _, err := ParsePreToolUse(mustJSON(t, validInput(name))); err != nil {
			t.Fatalf("tool %q rejected: %v", name, err)
		}
		if operation, ok := bridgeToolOperation(name); !ok || operation != wantOperation {
			t.Fatalf("tool %q operation = %q, %t", name, operation, ok)
		}
	}
	for _, name := range []string{"mcp__oaw_codex_bridge__workflow_start", "mcp__oaw_codex_bridge__provider_inspect", "mcp__oaw_codex_bridge__plugin_list"} {
		if _, err := ParsePreToolUse(mustJSON(t, validInput(name))); codexbridge.Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
			t.Fatalf("tool %q error = %v", name, err)
		}
	}
}

func TestProcessMalformedInputReturnsDeny(t *testing.T) {
	result, err := ProcessPreToolUse([]byte(`{"hook_event_name":"PreToolUse"}`))
	if err != nil || result.HookSpecificOutput == nil || result.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func FuzzParsePreToolUse(f *testing.F) {
	f.Add([]byte(`{"session_id":"s","turn_id":"t","tool_use_id":"u","cwd":"/repo","hook_event_name":"PreToolUse","model":"m","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 64<<10 {
			return
		}
		output, _ := ProcessPreToolUse(raw)
		if output.HookSpecificOutput != nil && output.HookSpecificOutput.PermissionDecision == "allow" {
			input, err := ParsePreToolUse(raw)
			if err != nil || !isObservationTool(input.ToolName) {
				t.Fatalf("non-observation input received allow: tool=%q err=%v", input.ToolName, err)
			}
		}
	})
}

func validInput(toolName string) PreToolUseInput {
	return PreToolUseInput{
		SessionID: "s", TurnID: "t", ToolUseID: "u", CWD: "/repo",
		HookEventName: "PreToolUse", Model: "gpt-test", PermissionMode: "default",
		ToolName: toolName, ToolInput: json.RawMessage(`{}`),
	}
}

func validHandleInput(t *testing.T, toolName string) PreToolUseInput {
	t.Helper()
	input := validInput(toolName)
	sessionDigest, cwdDigest, err := codexbridge.ContextDigestHeaders(codexbridge.HookContext{SessionID: input.SessionID, CWD: input.CWD})
	if err != nil {
		t.Fatal(err)
	}
	handle := codexbridge.HostEvidenceHandle{
		Version: codexbridge.EvidenceHandleVersion, SessionDigest: sessionDigest,
		CWDDigest: cwdDigest, Token: strings.Repeat("h", 22),
	}
	encoded, err := json.Marshal(struct {
		HostEvidenceHandle codexbridge.HostEvidenceHandle `json:"host_evidence_handle"`
	}{handle})
	if err != nil {
		t.Fatal(err)
	}
	input.ToolInput = encoded
	return input
}

var bytesOfSpace = []byte{' ', '\n', '\t'}
