package host

import (
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

type WorkflowAdmission struct {
	HostID            string    `json:"host_id"`
	IntegrationID     string    `json:"integration_id"`
	IntegrationDigest string    `json:"integration_digest"`
	ManifestDigest    string    `json:"manifest_digest"`
	AuditDigest       string    `json:"audit_digest"`
	ConformanceDigest string    `json:"conformance_digest"`
	EffectiveFeatures []Feature `json:"effective_features"`
}

func AdmitWorkflow(records []IntegrationRecord, frame RuntimeFrame, bindings []catalog.HostBinding) (WorkflowAdmission, error) {
	if strings.TrimSpace(frame.IntegrationID) != frame.IntegrationID || frame.IntegrationID == "" {
		return WorkflowAdmission{}, hostError("HOST_INTEGRATION_REQUIRED", "Workflow requires a selected Host Integration", nil)
	}
	var integration *IntegrationRecord
	for _, record := range records {
		if record.ID == frame.IntegrationID {
			copyValue := CloneIntegration(record)
			integration = &copyValue
			break
		}
	}
	if integration == nil || ValidateIntegrationRecord(*integration) != nil || integration.Manifest.IntegrationLevel == InstructionOnly || integration.Audit.Status != AuditPassed || integration.Conformance == nil || !integration.Conformance.Passed {
		return WorkflowAdmission{}, hostError("HOST_INTEGRATION_NOT_ADMITTED", "Host Integration is absent, untrusted, instruction-only, or nonconforming", nil)
	}
	if frame.HostID != integration.Manifest.HostID {
		return WorkflowAdmission{}, hostError("HOST_PROVIDER_SCOPE_MISMATCH", "Runtime frame Host does not match the selected Host Integration", nil)
	}
	unavailable := append([]Feature{}, frame.UnavailableFeatures...)
	sort.Slice(unavailable, func(left, right int) bool { return unavailable[left] < unavailable[right] })
	for index, feature := range unavailable {
		if !slices.Contains(knownFeatures, feature) || !slices.Contains(integration.Manifest.Features, feature) || index > 0 && unavailable[index-1] == feature {
			return WorkflowAdmission{}, hostError("HOST_RUNTIME_REQUIREMENTS_UNMET", "Host frame contains an unknown or duplicate unavailable Feature", nil)
		}
	}
	effective := make([]Feature, 0, len(integration.Manifest.Features))
	for _, feature := range integration.Manifest.Features {
		if slices.Contains(unavailable, feature) {
			continue
		}
		effective = append(effective, feature)
	}
	for _, required := range integration.Manifest.Features {
		if !slices.Contains(effective, required) {
			return WorkflowAdmission{}, hostError("HOST_RUNTIME_REQUIREMENTS_UNMET", "Host temporarily lacks a required Workflow Feature", nil)
		}
	}
	if len(bindings) == 0 {
		return WorkflowAdmission{}, hostError("HOST_BINDING_UNSUPPORTED", "Workflow graph has no Host Bindings", nil)
	}
	for _, binding := range bindings {
		if binding.Host != integration.Manifest.HostID || !slices.Contains(integration.Manifest.BindingKinds, binding.Kind) || strings.TrimSpace(binding.Reference) != binding.Reference || binding.Reference == "" || strings.IndexFunc(binding.Reference, unicode.IsControl) >= 0 {
			return WorkflowAdmission{}, hostError("HOST_BINDING_UNSUPPORTED", "Workflow graph Binding is not admitted by the Host Manifest", nil)
		}
	}
	return WorkflowAdmission{
		HostID: frame.HostID, IntegrationID: frame.IntegrationID, IntegrationDigest: integration.Digest,
		ManifestDigest: integration.ManifestDigest, AuditDigest: integration.Audit.Digest,
		ConformanceDigest: integration.Conformance.Digest, EffectiveFeatures: effective,
	}, nil
}

func CloneWorkflowAdmission(value WorkflowAdmission) WorkflowAdmission {
	value.EffectiveFeatures = append([]Feature{}, value.EffectiveFeatures...)
	return value
}
