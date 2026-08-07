package codexbridge

import (
	"strings"
	"testing"
)

func TestDiagnosticProjectionNeverIncludesHandleOrAbsolutePath(t *testing.T) {
	err := NewError("HOST_EVIDENCE_HANDLE_INVALID", "edited token at /Users/example/repo", nil)
	value := ProjectDiagnostic(err, "codex", true)
	if strings.Contains(value.Detail, "token") || strings.Contains(value.Detail, "/Users/") {
		t.Fatalf("diagnostic leaked private data: %#v", value)
	}
	if value.Code != "HOST_EVIDENCE_HANDLE_INVALID" || value.Layer != "evidence" ||
		!value.DirectAvailable || !value.RecoverableByObservation || value.RecoveryAction == "" ||
		value.AffectedProviders == nil || value.AffectedProfiles == nil {
		t.Fatalf("diagnostic projection is incomplete: %#v", value)
	}
}

func TestDiagnosticProjectionKeepsSafeDetailAndMapsRecovery(t *testing.T) {
	value := NewDiagnostic("HOST_SESSION_CHANGED", "downstream", "the current session changed", false)
	if value.Detail != "the current session changed" || !value.RecoverableByObservation ||
		value.RecoveryAction == "" || value.EvidenceDigest != "" {
		t.Fatalf("diagnostic = %#v", value)
	}
}

func TestDiagnosticRecoveryMappingsAreClosed(t *testing.T) {
	tests := []struct {
		code        string
		layer       string
		recoverable bool
	}{
		{"HOST_BRIDGE_CONTEXT_REQUIRED", "bridge", true},
		{"HOST_EVIDENCE_EXPIRED", "evidence", true},
		{"HOST_OBSERVATION_PARTIAL", "downstream", true},
		{"HOST_OBSERVATION_FAILED", "downstream", false},
	}
	for _, test := range tests {
		value := NewDiagnostic(test.code, layerForCode(test.code), "safe", false)
		if value.Layer != test.layer || value.RecoverableByObservation != test.recoverable || value.RecoveryAction == "" {
			t.Fatalf("%s diagnostic=%#v", test.code, value)
		}
	}
}
