package codexbridge

import (
	"errors"
	"strings"
)

type Diagnostic struct {
	Code                     string   `json:"code"`
	Layer                    string   `json:"layer"`
	Detail                   string   `json:"detail"`
	AffectedProviders        []string `json:"affected_providers"`
	AffectedProfiles         []string `json:"affected_profiles"`
	DirectAvailable          bool     `json:"direct_available"`
	RecoverableByObservation bool     `json:"recoverable_by_observation"`
	RecoveryAction           string   `json:"recovery_action"`
	EvidenceDigest           string   `json:"evidence_digest"`
}

func layerForCode(code string) string {
	switch {
	case strings.HasPrefix(code, "HOST_BRIDGE_"):
		return "bridge"
	case strings.HasPrefix(code, "HOST_EVIDENCE_"):
		return "evidence"
	default:
		return "downstream"
	}
}

func ProjectDiagnostic(err error, evidenceDigest string, directAvailable bool) Diagnostic {
	code := Code(err)
	layer := layerForCode(code)
	detail := "Bridge operation failed"
	var value *Error
	if errors.As(err, &value) {
		layer, detail = value.Layer, value.Detail
	}
	diagnostic := NewDiagnostic(code, layer, detail, directAvailable)
	if validDiagnosticDigest(evidenceDigest) {
		diagnostic.EvidenceDigest = evidenceDigest
	}
	return diagnostic
}

func validDiagnosticDigest(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

func NewDiagnostic(code, layer, detail string, directAvailable bool) Diagnostic {
	return Diagnostic{
		Code:                     code,
		Layer:                    layer,
		Detail:                   redactDiagnosticDetail(detail),
		AffectedProviders:        []string{},
		AffectedProfiles:         []string{},
		DirectAvailable:          directAvailable,
		RecoverableByObservation: recoverableByObservation(code),
		RecoveryAction:           recoveryAction(code),
		EvidenceDigest:           "",
	}
}

func redactDiagnosticDetail(value string) string {
	for _, marker := range []string{"oawh1.", "/Users/", "/home/", "token", "credential", "Authorization"} {
		if strings.Contains(value, marker) {
			return "Bridge operation failed; inspect the stable diagnostic code"
		}
	}
	return value
}

func recoverableByObservation(code string) bool {
	switch code {
	case "HOST_BRIDGE_CONTEXT_REQUIRED", "HOST_EVIDENCE_HANDLE_REQUIRED", "HOST_EVIDENCE_HANDLE_INVALID", "HOST_EVIDENCE_EXPIRED", "HOST_EVIDENCE_SESSION_MISMATCH", "HOST_OBSERVATION_PARTIAL", "HOST_SESSION_CHANGED":
		return true
	default:
		return false
	}
}

func recoveryAction(code string) string {
	switch code {
	case "HOST_BRIDGE_CONTEXT_REQUIRED":
		return "review and trust the OAW PreToolUse Hook, then observe again"
	case "HOST_EVIDENCE_HANDLE_REQUIRED", "HOST_EVIDENCE_HANDLE_INVALID", "HOST_EVIDENCE_EXPIRED", "HOST_EVIDENCE_SESSION_MISMATCH":
		return "call observe_current in the active Codex session"
	case "HOST_OBSERVATION_PARTIAL":
		return "retain unknown environment dispositions and continue only within verified scope"
	case "HOST_SESSION_CHANGED":
		return "pause the Workflow and perform the legal recovery or switching transition"
	default:
		return "inspect the stable diagnostic code and update the Bridge bundle if required"
	}
}
