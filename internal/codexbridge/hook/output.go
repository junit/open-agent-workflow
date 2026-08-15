package hook

import (
	"encoding/json"

	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
)

type PreToolUseDecision struct {
	HookEventName            string                     `json:"hookEventName"`
	PermissionDecision       string                     `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string                     `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]json.RawMessage `json:"updatedInput,omitempty"`
}

type HookOutput struct {
	HookSpecificOutput *PreToolUseDecision `json:"hookSpecificOutput,omitempty"`
}

func ProcessPreToolUse(raw []byte) (HookOutput, error) {
	input, err := ParsePreToolUse(raw)
	if err != nil {
		return denyContextMismatch(), nil
	}
	var public map[string]json.RawMessage
	if err := json.Unmarshal(input.ToolInput, &public); err != nil || public == nil {
		return denyContextMismatch(), nil
	}
	if _, exists := public["_oaw_host_context"]; exists {
		return denyContextMismatch(), nil
	}
	context := codexbridge.HookContext{
		SchemaVersion:         codexbridge.HookContextSchemaV3,
		BridgeProtocolVersion: codexbridge.BridgeProtocolVersion,
		SessionID:             input.SessionID, TurnID: input.TurnID, ToolUseID: input.ToolUseID,
		CWD: input.CWD, Model: input.Model, PermissionMode: input.PermissionMode,
	}
	if err := codexbridge.ValidateHookContext(context); err != nil {
		return denyContextMismatch(), nil
	}
	private, err := json.Marshal(context)
	if err != nil {
		return denyContextMismatch(), nil
	}
	public["_oaw_host_context"] = private
	return HookOutput{HookSpecificOutput: &PreToolUseDecision{
		HookEventName: "PreToolUse", PermissionDecision: "allow", UpdatedInput: public,
	}}, nil
}

func denyContextMismatch() HookOutput {
	return HookOutput{HookSpecificOutput: &PreToolUseDecision{
		HookEventName: "PreToolUse", PermissionDecision: "deny",
		PermissionDecisionReason: "OAW Host evidence does not match the current Codex session and working directory.",
	}}
}
