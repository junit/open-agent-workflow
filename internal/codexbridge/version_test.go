package codexbridge

import (
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestVersionEvidenceV2CarriesCompleteAuthorityTuple(t *testing.T) {
	value := compatibleVersions()
	if value.PluginVersion != "1.2.3" ||
		value.BridgeProtocol != BridgeProtocolVersion ||
		value.HookContextSchema != HookContextSchemaV2 ||
		value.IntegrationVersion != BridgeIntegrationVersion ||
		value.ProviderDescriptorSchema != catalog.ProviderDescriptorSchemaV4 ||
		value.ProfileRecipeSchema != catalog.ProfileRecipeSchemaV3 ||
		value.HostManifestSchema != host.HostManifestSchemaV3 ||
		value.HostSessionSchema != host.HostSessionSchemaV3 ||
		value.HostBindingInventorySchema != host.BindingInventorySchemaV3 ||
		value.HostEnvironmentReportSchema != host.HostEnvironmentReportSchemaV2 ||
		value.HostInvocationReceiptSchema != host.HostInvocationReceiptSchemaV3 ||
		value.HostConformanceTranscriptSchema != host.HostConformanceTranscriptSchemaV4 ||
		value.HostConformanceReportSchema != host.HostConformanceReportSchemaV4 ||
		value.ExecutionGraphSchema != profile.ExecutionGraphSchemaV4 ||
		value.LifecycleBundleSchema != core.LifecycleBundleSchemaV4 ||
		value.CapabilityGrantSchema != admission.CapabilityGrantSchemaV3 ||
		value.DispatchPacketSchema != coordinator.DispatchPacketSchemaV2 ||
		value.WorkflowCommandSchema != coordinator.WorkflowCommandSchemaV2 ||
		value.WorkflowResultSchema != coordinator.WorkflowResultSchemaV2 ||
		value.WorkflowSnapshotSchema != coordinator.WorkflowSnapshotSchemaV2 ||
		value.WorkflowRevisionSchema != coordinator.WorkflowRevisionSchemaV2 ||
		len(value.Digest) != 64 {
		t.Fatalf("VersionEvidence = %#v", value)
	}
	if _, err := Negotiate(value); err != nil {
		t.Fatal(err)
	}
}

func TestVersionEvidenceV2RejectsIndependentFieldDriftWithRebuiltDigest(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*VersionEvidence)
	}{
		{"plugin", func(v *VersionEvidence) { v.PluginVersion = "" }},
		{"bridge", func(v *VersionEvidence) { v.BridgeProtocol = "oaw.codex-bridge/v1" }},
		{"hook", func(v *VersionEvidence) { v.HookContextSchema = "oaw.codex-hook-context/v1" }},
		{"integration", func(v *VersionEvidence) { v.IntegrationVersion = "1.0.0" }},
		{"codex", func(v *VersionEvidence) { v.CodexVersion = "0.146.0" }},
		{"methods", func(v *VersionEvidence) { v.MetadataMethods = []string{"config/read", "skills/list", "skills/list"} }},
		{"provider", func(v *VersionEvidence) { v.ProviderDescriptorSchema = "oaw.provider-descriptor/v3" }},
		{"recipe", func(v *VersionEvidence) { v.ProfileRecipeSchema = "oaw.profile-recipe/v2" }},
		{"manifest", func(v *VersionEvidence) { v.HostManifestSchema = "oaw.host-manifest/v2" }},
		{"session", func(v *VersionEvidence) { v.HostSessionSchema = "oaw.host-session/v2" }},
		{"inventory", func(v *VersionEvidence) { v.HostBindingInventorySchema = "oaw.host-binding-inventory/v2" }},
		{"environment", func(v *VersionEvidence) { v.HostEnvironmentReportSchema = "oaw.host-environment-report/v1" }},
		{"receipt", func(v *VersionEvidence) { v.HostInvocationReceiptSchema = "oaw.host-invocation-receipt/v2" }},
		{"transcript", func(v *VersionEvidence) { v.HostConformanceTranscriptSchema = "oaw.host-conformance-transcript/v3" }},
		{"report", func(v *VersionEvidence) { v.HostConformanceReportSchema = "oaw.host-conformance-report/v3" }},
		{"graph", func(v *VersionEvidence) { v.ExecutionGraphSchema = "oaw.execution-graph/v3" }},
		{"bundle", func(v *VersionEvidence) { v.LifecycleBundleSchema = "oaw.lifecycle-bundle/v3" }},
		{"grant", func(v *VersionEvidence) { v.CapabilityGrantSchema = "oaw.capability-grant/v2" }},
		{"dispatch", func(v *VersionEvidence) { v.DispatchPacketSchema = "oaw.dispatch-packet/v1" }},
		{"command", func(v *VersionEvidence) { v.WorkflowCommandSchema = "oaw.workflow-command/v1" }},
		{"result", func(v *VersionEvidence) { v.WorkflowResultSchema = "oaw.workflow-result/v1" }},
		{"snapshot", func(v *VersionEvidence) { v.WorkflowSnapshotSchema = "oaw.workflow-snapshot/v1" }},
		{"revision", func(v *VersionEvidence) { v.WorkflowRevisionSchema = "oaw.workflow-revision/v1" }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			input := compatibleVersions()
			mutation.mutate(&input)
			input.Digest = versionEvidenceDigest(t, input)
			if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
				t.Fatalf("input = %#v, error = %v", input, err)
			}
		})
	}
}

func TestVersionEvidenceV2RejectsDigestDriftAndDoesNotMutateCaller(t *testing.T) {
	input := compatibleVersions()
	before := append([]string{}, input.MetadataMethods...)
	input.Digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
	if !slices.Equal(input.MetadataMethods, before) {
		t.Fatalf("Negotiate mutated methods: %q", input.MetadataMethods)
	}
}

func TestVersionEvidenceV2CanonicalizesMetadataMethodsDefensively(t *testing.T) {
	methods := []string{"skills/list", "config/read", "hooks/list"}
	value, err := currentVersionEvidence("1.2.3", "codex-cli 0.146.1", methods)
	if err != nil {
		t.Fatal(err)
	}
	methods[0] = "forged"
	if !slices.Equal(value.MetadataMethods, []string{"config/read", "hooks/list", "skills/list"}) {
		t.Fatalf("methods = %q", value.MetadataMethods)
	}
}

func TestNegotiateKeepsOptionalMetadataFailuresPartial(t *testing.T) {
	input, err := currentVersionEvidence("1.2.3", "codex-cli 0.146.1", []string{"skills/list"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Negotiate(input)
	if err != nil || !slices.Equal(result.MissingOptionalMethods, []string{"config/read", "hooks/list"}) {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestNegotiateRejectsUnverifiedCodexVersions(t *testing.T) {
	for _, version := range []string{"0.146.0", "0.146.1-beta.1", "codex-cli next", "0.146", ""} {
		input := compatibleVersions()
		input.CodexVersion = version
		input.Digest = versionEvidenceDigest(t, input)
		if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
			t.Fatalf("version = %q, error = %v", version, err)
		}
	}
}

func TestNegotiateAcceptsVerifiedBaselineAndNewerVersions(t *testing.T) {
	for _, version := range []string{"codex-cli 0.146.1", "codex-cli/0.146.1", "0.147.0", "1.0.0", "oaw-codex-bridge/0.146.1 (Mac OS 15.7.7; arm64) Orca/1.4.175"} {
		input := compatibleVersions()
		input.CodexVersion = version
		input.Digest = versionEvidenceDigest(t, input)
		result, err := Negotiate(input)
		if err != nil || !result.Compatible || result.CodexVersion != version {
			t.Fatalf("version = %q, result = %#v, error = %v", version, result, err)
		}
	}
}

func TestMetadataValidationUsesCompleteVersionEvidence(t *testing.T) {
	metadata := completeFactMetadata()
	metadata.CodexVersion = "0.146.0"
	if err := validateMetadataObservation(HookContext{CWD: "/repo"}, metadata, "1.2.3"); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}

	metadata = completeFactMetadata()
	metadata.Methods = []string{"config/read", "hooks/list"}
	if err := validateMetadataObservation(HookContext{CWD: "/repo"}, metadata, "1.2.3"); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
}

func compatibleVersions() VersionEvidence {
	value, err := currentVersionEvidence("1.2.3", "codex-cli 0.146.1", []string{"skills/list", "hooks/list", "config/read"})
	if err != nil {
		panic(err)
	}
	return value
}

func versionEvidenceDigest(t *testing.T, value VersionEvidence) string {
	t.Helper()
	value.Digest = ""
	digest, err := digestVersionEvidence(value)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
