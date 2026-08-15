package hook

import (
	"encoding/json"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
)

func TestProcessPreToolUseInjectsV3Context(t *testing.T) {
	raw := []byte(`{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"workspace-write","tool_name":"mcp__oaw_codex_bridge__observe_profile","tool_input":{"profile":"project:team"}}`)
	output, err := ProcessPreToolUse(raw)
	if err != nil || output.HookSpecificOutput == nil || output.HookSpecificOutput.PermissionDecision != "allow" {
		t.Fatalf("ProcessPreToolUse() = %#v, %v", output, err)
	}
	encoded := output.HookSpecificOutput.UpdatedInput["_oaw_host_context"]
	var context codexbridge.HookContext
	if err := json.Unmarshal(encoded, &context); err != nil {
		t.Fatal(err)
	}
	if context.SchemaVersion != codexbridge.HookContextSchemaV3 ||
		context.BridgeProtocolVersion != codexbridge.BridgeProtocolVersion || context.SessionID != "session-a" {
		t.Fatalf("Hook context = %#v", context)
	}
}

func TestProcessPreToolUseDeniesCallerContextAndRetiredTools(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"workspace-write","tool_name":"mcp__oaw_codex_bridge__observe_profile","tool_input":{"profile":"project:team","_oaw_host_context":{}}}`),
		[]byte(`{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"workspace-write","tool_name":"mcp__oaw_codex_bridge__core_compile","tool_input":{}}`),
	} {
		output, err := ProcessPreToolUse(raw)
		if err != nil || output.HookSpecificOutput == nil || output.HookSpecificOutput.PermissionDecision != "deny" {
			t.Fatalf("ProcessPreToolUse(%s) = %#v, %v", raw, output, err)
		}
	}
}
