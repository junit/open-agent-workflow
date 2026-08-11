package hook

import (
	"bytes"
	"encoding/json"
	"io"

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

func RewriteObserveInput(input PreToolUseInput) (HookOutput, error) {
	if !isObservationTool(input.ToolName) {
		return HookOutput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "unexpected observation tool", nil)
	}
	var public map[string]json.RawMessage
	if err := json.Unmarshal(input.ToolInput, &public); err != nil || public == nil {
		return HookOutput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "tool input must be an object", err)
	}
	if _, exists := public["_oaw_host_context"]; exists {
		return HookOutput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "reserved context was caller supplied", nil)
	}
	context := codexbridge.HookContext{
		SchemaVersion:         codexbridge.HookContextSchemaV2,
		BridgeProtocolVersion: codexbridge.BridgeProtocolVersion,
		SessionID:             input.SessionID,
		TurnID:                input.TurnID,
		ToolUseID:             input.ToolUseID,
		CWD:                   input.CWD,
		Model:                 input.Model,
		PermissionMode:        input.PermissionMode,
	}
	if _, _, err := codexbridge.ContextDigestHeaders(context); err != nil {
		return HookOutput{}, err
	}
	private, err := json.Marshal(context)
	if err != nil {
		return HookOutput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "encode reserved context", err)
	}
	public["_oaw_host_context"] = private
	return HookOutput{HookSpecificOutput: &PreToolUseDecision{
		HookEventName: "PreToolUse", PermissionDecision: "allow", UpdatedInput: public,
	}}, nil
}

func denyContextMismatch() HookOutput {
	return HookOutput{HookSpecificOutput: &PreToolUseDecision{
		HookEventName:            "PreToolUse",
		PermissionDecision:       "deny",
		PermissionDecisionReason: "OAW Host evidence does not match the current Codex session and working directory.",
	}}
}

func ValidateHandleInput(input PreToolUseInput) (HookOutput, error) {
	switch input.ToolName {
	case "mcp__oaw_codex_bridge__core_inspect", "mcp__oaw_codex_bridge__core_compile", "mcp__oaw_codex_bridge__workflow_exchange":
	default:
		return denyContextMismatch(), nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(input.ToolInput, &object); err != nil || object == nil {
		return denyContextMismatch(), nil
	}
	encoded, ok := object["host_evidence_handle"]
	if !ok || len(encoded) == 0 {
		return denyContextMismatch(), nil
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var handle codexbridge.HostEvidenceHandle
	if err := decoder.Decode(&handle); err != nil || handle.Token == "" {
		return denyContextMismatch(), nil
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return denyContextMismatch(), nil
	}
	if err := codexbridge.ValidateHandleContext(handle, codexbridge.HookContext{
		SchemaVersion:         codexbridge.HookContextSchemaV2,
		BridgeProtocolVersion: codexbridge.BridgeProtocolVersion,
		SessionID:             input.SessionID,
		TurnID:                input.TurnID,
		ToolUseID:             input.ToolUseID,
		CWD:                   input.CWD,
		Model:                 input.Model,
		PermissionMode:        input.PermissionMode,
	}); err != nil {
		return denyContextMismatch(), nil
	}
	return HookOutput{}, nil
}

func ProcessPreToolUse(raw []byte) (HookOutput, error) {
	input, err := ParsePreToolUse(raw)
	if err != nil {
		return denyContextMismatch(), nil
	}
	if isObservationTool(input.ToolName) {
		output, rewriteErr := RewriteObserveInput(input)
		if rewriteErr != nil {
			return denyContextMismatch(), nil
		}
		return output, nil
	}
	return ValidateHandleInput(input)
}
