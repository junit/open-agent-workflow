package codexbridge

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestDiagnosticProjectionNeverIncludesHandleOrAbsolutePath(t *testing.T) {
	err := NewError("HOST_EVIDENCE_HANDLE_INVALID", "edited token at /Users/example/repo", nil)
	evidenceDigest := strings.Repeat("a", 64)
	value := ProjectDiagnostic(err, evidenceDigest, true)
	if strings.Contains(value.Detail, "token") || strings.Contains(value.Detail, "/Users/") {
		t.Fatalf("diagnostic leaked private data: %#v", value)
	}
	if value.Code != "HOST_EVIDENCE_HANDLE_INVALID" || value.Layer != "evidence" ||
		!value.DirectAvailable || !value.RecoverableByObservation || value.RecoveryAction == "" ||
		value.AffectedProviders == nil || value.AffectedProfiles == nil || value.EvidenceDigest != evidenceDigest {
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

func TestNormalizeDiagnosticsDeduplicatesDeterministicallyAndReportsTruncation(t *testing.T) {
	values := make([]Diagnostic, 0, maximumObserveCurrentDiagnostics+8)
	for index := maximumObserveCurrentDiagnostics + 6; index >= 0; index-- {
		value := NewDiagnostic(fmt.Sprintf("TEST_%03d", index), "binding", fmt.Sprintf("detail %03d", index), true)
		value.AffectedProviders = []string{fmt.Sprintf("provider/%03d", index)}
		values = append(values, value)
	}
	duplicate := NewDiagnostic("TEST_000", "binding", "detail 000", true)
	duplicate.AffectedProviders = []string{"provider/duplicate"}
	duplicate.AffectedProfiles = []string{"PROFILE-B"}
	values = append(values, duplicate)

	first := normalizeDiagnostics(values, maximumObserveCurrentDiagnostics)
	slices.Reverse(values)
	second := normalizeDiagnostics(values, maximumObserveCurrentDiagnostics)
	if !slices.EqualFunc(first, second, func(left, right Diagnostic) bool {
		return left.Code == right.Code && left.Layer == right.Layer && left.Detail == right.Detail &&
			slices.Equal(left.AffectedProviders, right.AffectedProviders) && slices.Equal(left.AffectedProfiles, right.AffectedProfiles)
	}) {
		t.Fatalf("normalization depends on input order\nfirst=%#v\nsecond=%#v", first, second)
	}
	if len(first) != maximumObserveCurrentDiagnostics {
		t.Fatalf("diagnostic count=%d, want %d", len(first), maximumObserveCurrentDiagnostics)
	}
	if !slices.Equal(first[0].AffectedProviders, []string{"provider/000", "provider/duplicate"}) ||
		!slices.Equal(first[0].AffectedProfiles, []string{"PROFILE-B"}) {
		t.Fatalf("deduplicated ownership=%#v", first[0])
	}
	summary := first[len(first)-1]
	if summary.Code != "HOST_DIAGNOSTICS_TRUNCATED" || summary.Detail != "8 additional diagnostic groups omitted by the output budget" {
		t.Fatalf("truncation summary=%#v", summary)
	}
}

func TestAggregateDiagnosticsDeduplicatesWithoutTruncating(t *testing.T) {
	values := make([]Diagnostic, 0, maximumObserveCurrentDiagnostics+2)
	for index := 0; index <= maximumObserveCurrentDiagnostics; index++ {
		values = append(values, NewDiagnostic(fmt.Sprintf("TEST_%03d", index), "binding", "safe", true))
	}
	values = append(values, NewDiagnostic("TEST_000", "binding", "safe", true))
	result := aggregateDiagnostics(values)
	if len(result) != maximumObserveCurrentDiagnostics+1 || result[0].Code != "TEST_000" || result[len(result)-1].Code != "TEST_032" {
		t.Fatalf("aggregated diagnostics=%#v", result)
	}
}
