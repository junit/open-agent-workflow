package codexbridge

import (
	"slices"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

var observedMetadataMethods = []string{"config/read", "hooks/list", "skills/list"}

func AssembleFacts(context HookContext, metadata appserver.MetadataObservation, snapshot config.Snapshot, report discovery.Report, inventory host.BindingInventory, resolution core.ResolutionResult) (Facts, error) {
	rebuilt, err := host.NewBindingInventory(inventory.HostID, inventory.Observations)
	if err != nil || inventory.HostID != "codex" || inventory.SchemaVersion != host.BindingInventorySchemaV2 || rebuilt.Digest != inventory.Digest {
		return Facts{}, NewError("HOST_OBSERVATION_FAILED", "Skill inventory is not canonical", err)
	}
	if report.HostID() != "codex" || resolution.Report.HostID() != "codex" || resolution.Registry.HostID() != "codex" {
		return Facts{}, NewError("HOST_OBSERVATION_FAILED", "Host facts do not belong to Codex", nil)
	}
	expectedResolution, err := core.Resolve(core.ResolutionRequest{
		Configuration: snapshot, HostID: "codex", Discovery: report, Inventory: &rebuilt,
	})
	if err != nil || expectedResolution.Digest != resolution.Digest || expectedResolution.Report.Digest() != resolution.Report.Digest() || expectedResolution.Registry.Digest() != resolution.Registry.Digest() {
		return Facts{}, NewError("HOST_OBSERVATION_FAILED", "Provider resolution is not pinned to the observed Host facts", err)
	}
	resolution = expectedResolution
	if err := validateMetadataObservation(context, metadata); err != nil {
		return Facts{}, err
	}
	environment, err := buildCurrentEnvironment(context, metadata)
	if err != nil {
		return Facts{}, err
	}
	manifest, err := CodexHostManifest()
	if err != nil {
		return Facts{}, NewError("HOST_OBSERVATION_FAILED", "Codex Host Manifest is invalid", err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion:           host.HostSessionSchemaV2,
		HostID:                  "codex",
		IntegrationID:           BridgeIntegrationID,
		IntegrationVersion:      BridgeIntegrationVersion,
		SessionID:               context.SessionID,
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: rebuilt.Digest,
		EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		return Facts{}, NewError("HOST_OBSERVATION_FAILED", "Codex Host Session cannot be assembled", err)
	}
	return Facts{
		Session: session, Inventory: rebuilt, Environment: environment,
		Configuration: snapshot, Discovery: report, Resolutions: resolution.Report, Registry: resolution.Registry,
		FactDigests: FactDigests{
			Session: session.Digest, Inventory: rebuilt.Digest, Environment: environment.Digest,
			Configuration: snapshot.Digest(), Discovery: report.Digest(),
			Resolution: resolution.Report.Digest(), Registry: resolution.Registry.Digest(),
		},
	}, nil
}

func validateMetadataObservation(context HookContext, metadata appserver.MetadataObservation) error {
	if !validCanonicalPath(context.CWD) || metadata.Skills.CWD != context.CWD || !validMethodSet(metadata.Methods) {
		return NewError("HOST_OBSERVATION_FAILED", "Codex metadata is not bound to the current CWD", nil)
	}
	hooksObserved := slices.Contains(metadata.Methods, "hooks/list")
	if hooksObserved && metadata.Hooks.CWD != context.CWD {
		return NewError("HOST_OBSERVATION_FAILED", "Hook metadata is not bound to the current CWD", nil)
	}
	configObserved := slices.Contains(metadata.Methods, "config/read")
	if configObserved != metadata.Config.CWDObserved {
		return NewError("HOST_OBSERVATION_FAILED", "config/read CWD evidence is inconsistent", nil)
	}
	for _, disposition := range []string{metadata.Config.SandboxDisposition, metadata.Config.MCPDisposition, metadata.Config.HookDisposition, metadata.Config.ApprovalDisposition} {
		if !validProjectionDisposition(disposition) {
			return NewError("HOST_OBSERVATION_FAILED", "config/read contains an invalid disposition", nil)
		}
	}
	return nil
}

func validMethodSet(methods []string) bool {
	if !slices.IsSorted(methods) || !slices.Contains(methods, "skills/list") {
		return false
	}
	for index, method := range methods {
		if !slices.Contains(observedMetadataMethods, method) || index > 0 && methods[index-1] == method {
			return false
		}
	}
	return true
}

func buildCurrentEnvironment(context HookContext, metadata appserver.MetadataObservation) (host.EnvironmentReport, error) {
	configObserved := slices.Contains(metadata.Methods, "config/read") && metadata.Config.CWDObserved
	hooksObserved := slices.Contains(metadata.Methods, "hooks/list")
	unknown := execution.DispositionUnknown
	dispositions := map[string]execution.EnvironmentDisposition{
		"skills":    execution.DispositionInherited,
		"hooks":     unknown,
		"mcp":       unknown,
		"sandbox":   unknown,
		"approvals": unknown,
	}
	if configObserved {
		dispositions["mcp"] = projectedDisposition(metadata.Config.MCPDisposition)
		dispositions["sandbox"] = projectedDisposition(metadata.Config.SandboxDisposition)
		dispositions["approvals"] = projectedDisposition(metadata.Config.ApprovalDisposition)
		if hooksObserved {
			dispositions["hooks"] = projectedDisposition(metadata.Config.HookDisposition)
		}
	}

	observations := make([]execution.EnvironmentObservation, 0, len(dispositions))
	for _, surface := range []string{"skills", "hooks", "mcp", "sandbox", "approvals"} {
		disposition := dispositions[surface]
		digest, err := environmentEvidenceDigest(surface, disposition, metadata)
		if err != nil {
			return host.EnvironmentReport{}, NewError("HOST_OBSERVATION_FAILED", "environment evidence cannot be canonicalized", err)
		}
		observations = append(observations, execution.EnvironmentObservation{
			Surface: surface, Disposition: disposition, Source: "codex-app-server", Digest: digest,
		})
	}
	result, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2,
		SessionID:     context.SessionID, Topology: execution.TopologyCurrent, Observations: observations,
	})
	if err != nil {
		return host.EnvironmentReport{}, NewError("HOST_OBSERVATION_FAILED", "Codex environment report cannot be assembled", err)
	}
	return result, nil
}

func environmentEvidenceDigest(surface string, disposition execution.EnvironmentDisposition, metadata appserver.MetadataObservation) (string, error) {
	record := struct {
		Surface      string                           `json:"surface"`
		Disposition  execution.EnvironmentDisposition `json:"disposition"`
		Observed     bool                             `json:"observed"`
		Method       string                           `json:"method"`
		CodexVersion string                           `json:"codex_version"`
		Skills       *appserver.SkillsEntry           `json:"skills,omitempty"`
		Hooks        *appserver.HooksEntry            `json:"hooks,omitempty"`
	}{
		Surface: surface, Disposition: disposition,
		CodexVersion: metadata.CodexVersion,
	}
	switch surface {
	case "skills":
		record.Observed = true
		record.Method = "skills/list"
		value := cloneSkillsEntry(metadata.Skills)
		record.Skills = &value
	case "hooks":
		record.Observed = slices.Contains(metadata.Methods, "hooks/list")
		if record.Observed {
			record.Method = "hooks/list"
			value := cloneHooksEntry(metadata.Hooks)
			record.Hooks = &value
		}
	default:
		record.Observed = slices.Contains(metadata.Methods, "config/read") && metadata.Config.CWDObserved
		if record.Observed {
			record.Method = "config/read"
		}
	}
	digest, _, err := canonicaljson.Digest(record)
	return digest, err
}

func cloneSkillsEntry(value appserver.SkillsEntry) appserver.SkillsEntry {
	value.Errors = append([]appserver.MetadataError{}, value.Errors...)
	value.Skills = append([]appserver.SkillMetadata{}, value.Skills...)
	return value
}

func cloneHooksEntry(value appserver.HooksEntry) appserver.HooksEntry {
	value.Errors = append([]appserver.MetadataError{}, value.Errors...)
	value.Warnings = append([]string{}, value.Warnings...)
	value.Hooks = append([]appserver.HookMetadata{}, value.Hooks...)
	return value
}

func projectedDisposition(value string) execution.EnvironmentDisposition {
	if execution.EnvironmentDisposition(value) == execution.DispositionHostConfigured {
		return execution.DispositionHostConfigured
	}
	return execution.DispositionUnknown
}

func validProjectionDisposition(value string) bool {
	switch execution.EnvironmentDisposition(value) {
	case execution.DispositionInherited, execution.DispositionHostConfigured, execution.DispositionRestricted, execution.DispositionUnknown, execution.DispositionUnavailable:
		return true
	default:
		return false
	}
}
