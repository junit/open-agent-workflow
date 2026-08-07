package hook

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestObserveRewriteIsTheOnlyAutomaticAllow(t *testing.T) {
	ctx := validInput("mcp__oaw_codex_bridge__observe_current")
	result, err := RewriteObserveInput(ctx)
	decision := result.HookSpecificOutput
	if err != nil || decision == nil || decision.HookEventName != "PreToolUse" || decision.PermissionDecision != "allow" || len(decision.UpdatedInput) == 0 {
		t.Fatalf("result = %#v, %v", result, err)
	}
	encoded, err := json.Marshal(result)
	if err != nil || bytes.Contains(encoded, []byte(`{"permissionDecision":`)) || !bytes.Contains(encoded, []byte(`{"hookSpecificOutput":`)) {
		t.Fatalf("wire output = %s", encoded)
	}
	if _, ok := decision.UpdatedInput["_oaw_host_context"]; !ok {
		t.Fatalf("reserved context missing: %#v", decision.UpdatedInput)
	}
	for _, name := range []string{"mcp__oaw_codex_bridge__core_inspect", "mcp__oaw_codex_bridge__core_compile", "mcp__oaw_codex_bridge__workflow_exchange"} {
		result, err := ValidateHandleInput(validHandleInput(t, name))
		if err != nil || result.HookSpecificOutput != nil {
			t.Fatalf("%s changed approval: %#v, %v", name, result, err)
		}
	}
}

func TestLaterOperationContextMismatchReturnsWrappedDeny(t *testing.T) {
	input := validHandleInput(t, "mcp__oaw_codex_bridge__core_inspect")
	input.SessionID = "foreign-session"
	result, err := ValidateHandleInput(input)
	decision := result.HookSpecificOutput
	if err != nil || decision == nil || decision.HookEventName != "PreToolUse" ||
		decision.PermissionDecision != "deny" || decision.PermissionDecisionReason == "" || len(decision.UpdatedInput) != 0 {
		t.Fatalf("result = %#v, %v", result, err)
	}
}

func TestLaterOperationCWDAndMalformedHandleReturnDeny(t *testing.T) {
	foreignCWD := validHandleInput(t, "mcp__oaw_codex_bridge__core_inspect")
	foreignCWD.CWD = "/foreign-repo"
	malformed := validInput("mcp__oaw_codex_bridge__core_inspect")
	malformed.ToolInput = json.RawMessage(`{"host_evidence_handle":[]}`)
	missing := validInput("mcp__oaw_codex_bridge__core_inspect")
	for _, input := range []PreToolUseInput{foreignCWD, malformed, missing} {
		result, err := ValidateHandleInput(input)
		if err != nil || result.HookSpecificOutput == nil || result.HookSpecificOutput.PermissionDecision != "deny" {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	}
}

func TestObserveRewriteRejectsCallerSuppliedReservedContext(t *testing.T) {
	input := validInput("mcp__oaw_codex_bridge__observe_current")
	input.ToolInput = json.RawMessage(`{"_oaw_host_context":{}}`)
	if _, err := RewriteObserveInput(input); err == nil {
		t.Fatal("caller-supplied reserved context was accepted")
	}
}

func TestLaterOperationWithMatchingHandleReturnsNoOutput(t *testing.T) {
	for _, name := range []string{"mcp__oaw_codex_bridge__core_inspect", "mcp__oaw_codex_bridge__core_compile", "mcp__oaw_codex_bridge__workflow_exchange"} {
		result, err := ProcessPreToolUse(mustJSON(t, validHandleInput(t, name)))
		if err != nil || result.HookSpecificOutput != nil {
			t.Fatalf("%s result=%#v err=%v", name, result, err)
		}
	}
}

func mustJSON(t *testing.T, value PreToolUseInput) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
