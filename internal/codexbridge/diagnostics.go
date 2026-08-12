package codexbridge

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const maximumObserveCurrentDiagnostics = 32

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

type diagnosticKey struct {
	code                     string
	layer                    string
	detail                   string
	directAvailable          bool
	recoverableByObservation bool
	recoveryAction           string
	evidenceDigest           string
}

func normalizeDiagnostics(values []Diagnostic, limit int) []Diagnostic {
	result := aggregateDiagnostics(values)
	if len(result) == 0 {
		return result
	}
	if limit < 1 {
		limit = 1
	}
	if len(result) <= limit {
		return result
	}

	retained := limit - 1
	omitted := result[retained:]
	summary := NewDiagnostic(
		"HOST_DIAGNOSTICS_TRUNCATED", "observation",
		fmt.Sprintf("%d additional diagnostic groups omitted by the output budget", len(omitted)), true,
	)
	for _, value := range omitted {
		summary.AffectedProviders = append(summary.AffectedProviders, value.AffectedProviders...)
		summary.AffectedProfiles = append(summary.AffectedProfiles, value.AffectedProfiles...)
	}
	summary.AffectedProviders = sortedUniqueDiagnosticOwners(summary.AffectedProviders)
	summary.AffectedProfiles = sortedUniqueDiagnosticOwners(summary.AffectedProfiles)
	return append(result[:retained], summary)
}

func aggregateDiagnostics(values []Diagnostic) []Diagnostic {
	if len(values) == 0 {
		return []Diagnostic{}
	}
	aggregated := make(map[diagnosticKey]Diagnostic, len(values))
	for _, value := range values {
		key := diagnosticKey{
			code: value.Code, layer: value.Layer, detail: value.Detail, directAvailable: value.DirectAvailable,
			recoverableByObservation: value.RecoverableByObservation, recoveryAction: value.RecoveryAction,
			evidenceDigest: value.EvidenceDigest,
		}
		current, found := aggregated[key]
		if !found {
			current = value
			current.AffectedProviders = []string{}
			current.AffectedProfiles = []string{}
		}
		current.AffectedProviders = append(current.AffectedProviders, value.AffectedProviders...)
		current.AffectedProfiles = append(current.AffectedProfiles, value.AffectedProfiles...)
		aggregated[key] = current
	}

	result := make([]Diagnostic, 0, len(aggregated))
	for _, value := range aggregated {
		value.AffectedProviders = sortedUniqueDiagnosticOwners(value.AffectedProviders)
		value.AffectedProfiles = sortedUniqueDiagnosticOwners(value.AffectedProfiles)
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool {
		return diagnosticSortKey(result[left]) < diagnosticSortKey(result[right])
	})
	return result
}

func sortedUniqueDiagnosticOwners(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return compactSortedStrings(result)
}

func compactSortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func diagnosticSortKey(value Diagnostic) string {
	return strings.Join([]string{
		value.Code, value.Layer, value.Detail, fmt.Sprintf("%t", value.DirectAvailable),
		fmt.Sprintf("%t", value.RecoverableByObservation), value.RecoveryAction, value.EvidenceDigest,
	}, "\x00")
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
