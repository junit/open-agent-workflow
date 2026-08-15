package hook

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
)

func TestParsePreToolUseAcceptsOnlyObserveProfile(t *testing.T) {
	valid := []byte(`{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"workspace-write","tool_name":"mcp__oaw_codex_bridge__observe_profile","tool_input":{"profile":"project:team"}}`)
	input, err := ParsePreToolUse(valid)
	if err != nil || input.ToolName != "mcp__oaw_codex_bridge__observe_profile" {
		t.Fatalf("ParsePreToolUse() = %#v, %v", input, err)
	}
	operation, ok := bridgeToolOperation(input.ToolName)
	if !ok || operation != codexbridge.OperationObserveProfile {
		t.Fatalf("bridgeToolOperation() = %q, %t", operation, ok)
	}

	for _, retired := range []string{"observe_current", "core_inspect", "core_compile", "workflow_exchange"} {
		raw := []byte(`{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"workspace-write","tool_name":"mcp__oaw_codex_bridge__` + retired + `","tool_input":{}}`)
		if _, err := ParsePreToolUse(raw); codexbridge.Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
			t.Fatalf("retired tool %s error = %v", retired, err)
		}
	}
}

func TestParsePreToolUseRejectsOpenOrMalformedDocuments(t *testing.T) {
	for _, raw := range [][]byte{
		{},
		[]byte(`{"hook_event_name":"SubagentStart"}`),
		[]byte(`{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"workspace-write","tool_name":"mcp__oaw_codex_bridge__observe_profile","tool_input":{},"unknown":true}`),
	} {
		if _, err := ParsePreToolUse(raw); codexbridge.Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
			t.Fatalf("ParsePreToolUse(%s) error = %v", raw, err)
		}
	}
}
