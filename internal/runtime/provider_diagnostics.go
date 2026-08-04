package runtime

import (
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func providerResolutionDiagnostic(report registry.ResolutionReport, providerID string) (Diagnostic, bool) {
	resolution, found := report.Resolution(providerID)
	if !found || resolution.State == registry.Verified {
		return Diagnostic{}, false
	}
	return Diagnostic{
		Code: resolution.Reason,
		Message: fmt.Sprintf(
			"Provider %s is %s with %d candidate(s). Run oaw providers inspect --host <host>, update the user-owned Provider pin, then start a new Run.",
			providerID, resolution.State, len(resolution.Candidates),
		),
	}, true
}

func validProviderResolutionReason(value string) bool {
	switch value {
	case "PROVIDER_NOT_FOUND",
		"HOST_BINDING_EVIDENCE_REQUIRED",
		"PROVIDER_CANDIDATE_AMBIGUOUS",
		"PROVIDER_PIN_INCOMPATIBLE",
		"PROVIDER_BINDING_UNAVAILABLE",
		"PROVIDER_DISABLED_BY_USER",
		"PROVIDER_PROJECT_CONTENT_UNTRUSTED":
		return true
	default:
		return false
	}
}
