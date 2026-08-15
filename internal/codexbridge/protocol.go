package codexbridge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	BridgeProtocolVersion = "oaw.codex-bridge/v3"
	HookContextSchemaV3   = "oaw.codex-hook-context/v3"
)

type Operation string

const OperationObserveProfile Operation = "observe_profile"

type HookContext struct {
	SchemaVersion         string `json:"schema_version"`
	BridgeProtocolVersion string `json:"bridge_protocol_version"`
	SessionID             string `json:"session_id"`
	TurnID                string `json:"turn_id"`
	ToolUseID             string `json:"tool_use_id"`
	CWD                   string `json:"cwd"`
	Model                 string `json:"model"`
	PermissionMode        string `json:"permission_mode"`
}

type Error struct {
	Code   string
	Layer  string
	Detail string
	Cause  error
}

func (value *Error) Error() string {
	if value.Detail == "" {
		return value.Code
	}
	return value.Code + ": " + value.Detail
}

func (value *Error) Unwrap() error { return value.Cause }

func NewError(code, detail string, cause error) error {
	return &Error{Code: code, Layer: layerForCode(code), Detail: detail, Cause: cause}
}

func Code(err error) string {
	if err == nil {
		return ""
	}
	var value *Error
	if errors.As(err, &value) {
		return value.Code
	}
	return ""
}

func ParseOperation(value string) (Operation, error) {
	operation := Operation(value)
	if operation != OperationObserveProfile {
		return "", NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "operation is not in the v3 allowlist", nil)
	}
	return operation, nil
}

func ValidateHookContext(context HookContext) error {
	if context.SchemaVersion != HookContextSchemaV3 || context.BridgeProtocolVersion != BridgeProtocolVersion {
		return NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook context schema or Bridge protocol is unsupported", nil)
	}
	if _, _, err := ContextDigestHeaders(context); err != nil {
		return err
	}
	for _, field := range []string{context.TurnID, context.ToolUseID, context.Model, context.PermissionMode} {
		if !validContextText(field, 4096) {
			return NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook context identity is incomplete or invalid", nil)
		}
	}
	return nil
}

func ContextDigestHeaders(context HookContext) (string, string, error) {
	if !validContextText(context.SessionID, 512) {
		return "", "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "invalid Codex session identity", nil)
	}
	cwd, err := canonicalCWD(context.CWD)
	if err != nil {
		return "", "", err
	}
	return digestHeader("session", context.SessionID), digestHeader("cwd", cwd), nil
}

func digestHeader(kind, value string) string {
	digest := sha256.Sum256([]byte(BridgeProtocolVersion + "\x00" + kind + "\x00" + value))
	return hex.EncodeToString(digest[:])
}

func canonicalCWD(value string) (string, error) {
	if !utf8.ValidString(value) || value == "" || strings.IndexFunc(value, unicode.IsControl) >= 0 || !filepath.IsAbs(value) {
		return "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "cwd must be an absolute path without controls", nil)
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "canonicalize cwd", err)
	}
	return filepath.Clean(absolute), nil
}

func validContextText(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && len(value) <= maximum &&
		strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}
