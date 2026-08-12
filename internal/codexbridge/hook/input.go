package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
)

const maxHookInputBytes = 1 << 20

type PreToolUseInput struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath *string         `json:"transcript_path"`
	TurnID         string          `json:"turn_id"`
	ToolUseID      string          `json:"tool_use_id"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`
	Model          string          `json:"model"`
	PermissionMode string          `json:"permission_mode"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
}

type SubagentStartInput struct {
	SessionID      string  `json:"session_id"`
	TranscriptPath *string `json:"transcript_path"`
	TurnID         string  `json:"turn_id"`
	CWD            string  `json:"cwd"`
	HookEventName  string  `json:"hook_event_name"`
	Model          string  `json:"model"`
	PermissionMode string  `json:"permission_mode"`
	AgentID        string  `json:"agent_id"`
	AgentType      string  `json:"agent_type"`
}

func ParsePreToolUse(raw []byte) (PreToolUseInput, error) {
	if len(raw) == 0 || len(raw) > maxHookInputBytes {
		return PreToolUseInput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook input is empty or oversized", nil)
	}
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(raw), maxHookInputBytes+1))
	decoder.DisallowUnknownFields()
	var input PreToolUseInput
	if err := decoder.Decode(&input); err != nil {
		return PreToolUseInput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook input is not valid PreToolUse JSON", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return PreToolUseInput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook input contains trailing JSON", err)
	}
	if err := validatePreToolUseInput(input); err != nil {
		return PreToolUseInput{}, err
	}
	return input, nil
}

func ParseSubagentStart(raw []byte) (SubagentStartInput, error) {
	if len(raw) == 0 || len(raw) > maxHookInputBytes {
		return SubagentStartInput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook input is empty or oversized", nil)
	}
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(raw), maxHookInputBytes+1))
	decoder.DisallowUnknownFields()
	var input SubagentStartInput
	if err := decoder.Decode(&input); err != nil {
		return SubagentStartInput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook input is not valid SubagentStart JSON", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return SubagentStartInput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook input contains trailing JSON", err)
	}
	if input.HookEventName != "SubagentStart" || !validHookText(input.SessionID, 512) || !validHookText(input.TurnID, 512) ||
		!validHookText(input.CWD, 4096) || !validHookText(input.Model, 512) || !validHookText(input.PermissionMode, 128) ||
		!validHookText(input.AgentID, 512) || !validHookText(input.AgentType, 512) ||
		input.TranscriptPath != nil && !validHookText(*input.TranscriptPath, 4096) {
		return SubagentStartInput{}, codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "SubagentStart identity is incomplete or invalid", nil)
	}
	return input, nil
}

func EventName(raw []byte) (string, error) {
	if len(raw) == 0 || len(raw) > maxHookInputBytes {
		return "", codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook input is empty or oversized", nil)
	}
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(raw), maxHookInputBytes+1))
	var envelope struct {
		HookEventName string `json:"hook_event_name"`
	}
	if err := decoder.Decode(&envelope); err != nil || !validHookText(envelope.HookEventName, 128) {
		return "", codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook event name is invalid", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook input contains trailing JSON", err)
	}
	return envelope.HookEventName, nil
}

func validatePreToolUseInput(input PreToolUseInput) error {
	if input.HookEventName != "PreToolUse" || !validHookText(input.SessionID, 512) ||
		!validHookText(input.TurnID, 512) || !validHookText(input.ToolUseID, 512) ||
		!validHookText(input.CWD, 4096) || !validHookText(input.Model, 512) ||
		!validHookText(input.PermissionMode, 128) || !validHookText(input.ToolName, 256) {
		return codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook input identity is incomplete or invalid", nil)
	}
	if !isBridgeTool(input.ToolName) {
		return codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook tool is outside the Bridge matcher set", nil)
	}
	var object map[string]json.RawMessage
	if len(input.ToolInput) == 0 || json.Unmarshal(input.ToolInput, &object) != nil || object == nil {
		return codexbridge.NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook tool_input must be a JSON object", nil)
	}
	return nil
}

func validHookText(value string, maximumRunes int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximumRunes &&
		strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func isBridgeTool(value string) bool {
	_, ok := bridgeToolOperation(value)
	return ok
}

func bridgeToolOperation(value string) (codexbridge.Operation, bool) {
	var candidate string
	switch value {
	case "mcp__oaw_codex_bridge__observe_current":
		candidate = string(codexbridge.OperationObserveCurrent)
	case "mcp__oaw_codex_bridge__core_inspect":
		candidate = string(codexbridge.OperationCoreInspect)
	case "mcp__oaw_codex_bridge__core_compile":
		candidate = string(codexbridge.OperationCoreCompile)
	case "mcp__oaw_codex_bridge__workflow_exchange":
		candidate = string(codexbridge.OperationWorkflowExchange)
	default:
		return "", false
	}
	operation, err := codexbridge.ParseOperation(candidate)
	return operation, err == nil
}

func isObservationTool(value string) bool {
	operation, ok := bridgeToolOperation(value)
	return ok && operation == codexbridge.OperationObserveCurrent
}
