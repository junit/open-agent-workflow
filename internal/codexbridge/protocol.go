package codexbridge

import "errors"

const (
	BridgeProtocolVersion = "oaw.codex-bridge/v2"
	HookContextSchemaV2   = "oaw.codex-hook-context/v2"
	EvidenceHandleVersion = "oaw.host-evidence-handle/v2"
)

type Operation string

const (
	OperationObserveCurrent   Operation = "observe_current"
	OperationCoreInspect      Operation = "core.inspect"
	OperationCoreCompile      Operation = "core.compile"
	OperationWorkflowExchange Operation = "workflow_exchange"
)

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

type HostEvidenceHandle struct {
	Version       string `json:"version"`
	SessionDigest string `json:"session_digest"`
	CWDDigest     string `json:"cwd_digest"`
	Token         string `json:"token"`
}

type FactDigests struct {
	Session       string `json:"session"`
	Reporter      string `json:"reporter_identity"`
	Inventory     string `json:"inventory"`
	Environment   string `json:"environment"`
	Features      string `json:"features"`
	Actions       string `json:"actions"`
	Configuration string `json:"configuration"`
	Discovery     string `json:"discovery"`
	Resolution    string `json:"resolution"`
	Registry      string `json:"registry"`
	Version       string `json:"version_evidence"`
}

type OperationRequest struct {
	Handle HostEvidenceHandle `json:"host_evidence_handle"`
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
	var external interface{ ErrorCode() string }
	if errors.As(err, &external) {
		return external.ErrorCode()
	}
	return ""
}

func ParseOperation(value string) (Operation, error) {
	operation := Operation(value)
	switch operation {
	case OperationObserveCurrent, OperationCoreInspect, OperationCoreCompile, OperationWorkflowExchange:
		return operation, nil
	default:
		return "", NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "operation is not in the v2 allowlist", nil)
	}
}
