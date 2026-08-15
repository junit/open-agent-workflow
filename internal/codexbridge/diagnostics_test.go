package codexbridge

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestDiagnosticProjectionNeverIncludesSensitiveDetailOrAbsolutePath(t *testing.T) {
	err := NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "credential at /Users/example/repo", nil)
	value := ProjectDiagnostic(err)
	if strings.Contains(value.Detail, "token") || strings.Contains(value.Detail, "/Users/") {
		t.Fatalf("diagnostic leaked private data: %#v", value)
	}
	if value.Code != "HOST_BRIDGE_CONTEXT_REQUIRED" || value.Layer != "bridge" ||
		!value.RecoverableByObservation || value.RecoveryAction == "" || value.AffectedProviders == nil {
		t.Fatalf("diagnostic projection is incomplete: %#v", value)
	}
}

func TestDiagnosticProjectionKeepsSafeDetailAndMapsRecovery(t *testing.T) {
	value := NewDiagnostic("PROFILE_SELECTION_INVALID", "profile", "the Profile selector is invalid")
	if value.Detail != "the Profile selector is invalid" || value.RecoverableByObservation || value.RecoveryAction == "" {
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
		{"HOST_OBSERVATION_PARTIAL", "observation", true},
		{"HOST_OBSERVATION_FAILED", "observation", false},
		{"PROFILE_NOT_FOUND", "profile", false},
		{"PROFILE_AMBIGUOUS", "profile", false},
		{"ASSURANCE_BINDING_UNAVAILABLE", "assurance", false},
	}
	for _, test := range tests {
		value := NewDiagnostic(test.code, layerForCode(test.code), "safe")
		if value.Layer != test.layer || value.RecoverableByObservation != test.recoverable || value.RecoveryAction == "" {
			t.Fatalf("%s diagnostic=%#v", test.code, value)
		}
	}
}

func TestNormalizeDiagnosticsDeduplicatesDeterministicallyAndReportsTruncation(t *testing.T) {
	values := make([]Diagnostic, 0, maximumObserveProfileDiagnostics+8)
	for index := maximumObserveProfileDiagnostics + 6; index >= 0; index-- {
		value := NewDiagnostic(fmt.Sprintf("TEST_%03d", index), "binding", fmt.Sprintf("detail %03d", index))
		value.AffectedProviders = []string{fmt.Sprintf("provider/%03d", index)}
		values = append(values, value)
	}
	duplicate := NewDiagnostic("TEST_000", "binding", "detail 000")
	duplicate.AffectedProviders = []string{"provider/duplicate"}
	values = append(values, duplicate)

	first := normalizeDiagnostics(values, maximumObserveProfileDiagnostics)
	slices.Reverse(values)
	second := normalizeDiagnostics(values, maximumObserveProfileDiagnostics)
	if !slices.EqualFunc(first, second, func(left, right Diagnostic) bool {
		return left.Code == right.Code && left.Layer == right.Layer && left.Detail == right.Detail &&
			slices.Equal(left.AffectedProviders, right.AffectedProviders)
	}) {
		t.Fatalf("normalization depends on input order\nfirst=%#v\nsecond=%#v", first, second)
	}
	if len(first) != maximumObserveProfileDiagnostics {
		t.Fatalf("diagnostic count=%d, want %d", len(first), maximumObserveProfileDiagnostics)
	}
	if !slices.Equal(first[0].AffectedProviders, []string{"provider/000", "provider/duplicate"}) {
		t.Fatalf("deduplicated ownership=%#v", first[0])
	}
	summary := first[len(first)-1]
	if summary.Code != "HOST_DIAGNOSTICS_TRUNCATED" || summary.Detail != "8 additional diagnostic groups omitted by the output budget" {
		t.Fatalf("truncation summary=%#v", summary)
	}
}

func TestAggregateDiagnosticsDeduplicatesWithoutTruncating(t *testing.T) {
	values := make([]Diagnostic, 0, maximumObserveProfileDiagnostics+2)
	for index := 0; index <= maximumObserveProfileDiagnostics; index++ {
		values = append(values, NewDiagnostic(fmt.Sprintf("TEST_%03d", index), "binding", "safe"))
	}
	values = append(values, NewDiagnostic("TEST_000", "binding", "safe"))
	result := aggregateDiagnostics(values)
	if len(result) != maximumObserveProfileDiagnostics+1 || result[0].Code != "TEST_000" || result[len(result)-1].Code != "TEST_032" {
		t.Fatalf("aggregated diagnostics=%#v", result)
	}
}
