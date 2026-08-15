package codexbridge

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const maximumObserveProfileDiagnostics = 32

type Diagnostic struct {
	Code                     string   `json:"code"`
	Layer                    string   `json:"layer"`
	Detail                   string   `json:"detail"`
	AffectedProviders        []string `json:"affected_providers"`
	RecoverableByObservation bool     `json:"recoverable_by_observation"`
	RecoveryAction           string   `json:"recovery_action"`
}

func layerForCode(code string) string {
	switch {
	case strings.HasPrefix(code, "HOST_BRIDGE_"):
		return "bridge"
	case strings.HasPrefix(code, "HOST_OBSERVATION_"):
		return "observation"
	case strings.HasPrefix(code, "ASSURANCE_"):
		return "assurance"
	case strings.HasPrefix(code, "PROFILE_"):
		return "profile"
	case strings.HasPrefix(code, "PROVIDER_"), strings.HasPrefix(code, "HOST_BINDING_"), strings.HasPrefix(code, "HOST_SKILL_"):
		return "binding"
	default:
		return "bridge"
	}
}

func ProjectDiagnostic(err error) Diagnostic {
	code := Code(err)
	layer := layerForCode(code)
	detail := "Bridge operation failed"
	var value *Error
	if errors.As(err, &value) {
		layer, detail = value.Layer, value.Detail
	}
	return NewDiagnostic(code, layer, detail)
}

func NewDiagnostic(code, layer, detail string) Diagnostic {
	return Diagnostic{
		Code:                     code,
		Layer:                    layer,
		Detail:                   redactDiagnosticDetail(detail),
		AffectedProviders:        []string{},
		RecoverableByObservation: recoverableByObservation(code),
		RecoveryAction:           recoveryAction(code),
	}
}

type diagnosticKey struct {
	code                     string
	layer                    string
	detail                   string
	recoverableByObservation bool
	recoveryAction           string
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
	summary := NewDiagnostic("HOST_DIAGNOSTICS_TRUNCATED", "observation", fmt.Sprintf("%d additional diagnostic groups omitted by the output budget", len(omitted)))
	for _, value := range omitted {
		summary.AffectedProviders = append(summary.AffectedProviders, value.AffectedProviders...)
	}
	summary.AffectedProviders = sortedUniqueDiagnosticOwners(summary.AffectedProviders)
	return append(result[:retained], summary)
}

func aggregateDiagnostics(values []Diagnostic) []Diagnostic {
	if len(values) == 0 {
		return []Diagnostic{}
	}
	aggregated := make(map[diagnosticKey]Diagnostic, len(values))
	for _, value := range values {
		key := diagnosticKey{
			code: value.Code, layer: value.Layer, detail: value.Detail,
			recoverableByObservation: value.RecoverableByObservation, recoveryAction: value.RecoveryAction,
		}
		current, found := aggregated[key]
		if !found {
			current = value
			current.AffectedProviders = []string{}
		}
		current.AffectedProviders = append(current.AffectedProviders, value.AffectedProviders...)
		aggregated[key] = current
	}

	result := make([]Diagnostic, 0, len(aggregated))
	for _, value := range aggregated {
		value.AffectedProviders = sortedUniqueDiagnosticOwners(value.AffectedProviders)
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
		value.Code, value.Layer, value.Detail,
		fmt.Sprintf("%t", value.RecoverableByObservation), value.RecoveryAction,
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
	case "HOST_BRIDGE_CONTEXT_REQUIRED", "HOST_OBSERVATION_PARTIAL":
		return true
	default:
		return false
	}
}

func recoveryAction(code string) string {
	switch code {
	case "HOST_BRIDGE_CONTEXT_REQUIRED":
		return "review and trust the OAW PreToolUse Hook, then call observe_profile again"
	case "HOST_OBSERVATION_PARTIAL":
		return "retry observe_profile after current Codex Skill metadata is available"
	case "PROFILE_SELECTION_INVALID":
		return "supply one source-qualified Markdown Profile selector"
	case "PROFILE_NOT_FOUND":
		return "inspect the current Profile inventory and select an existing source-qualified Profile"
	case "PROFILE_AMBIGUOUS":
		return "remove or rename the duplicate Profile ID before requesting an Overlay"
	case "ASSURANCE_BINDING_UNAVAILABLE":
		return "repair the exact Provider Binding or use the Profile without the optional machine claim"
	default:
		return "inspect the stable diagnostic code and the selected Profile Binding evidence"
	}
}
