package codexbridge

import (
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestNegotiateRequiresExactBridgeAndHookProtocols(t *testing.T) {
	mutations := []func(*VersionEvidence){
		func(value *VersionEvidence) { value.PluginVersion = "1.1.0" },
		func(value *VersionEvidence) { value.BridgeProtocol = "oaw.codex-bridge/v2" },
		func(value *VersionEvidence) { value.HookContextSchema = "oaw.codex-hook-context/v2" },
		func(value *VersionEvidence) { value.IntegrationVersion = "1.1.0" },
	}
	for _, mutate := range mutations {
		input := compatibleVersions()
		mutate(&input)
		if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
			t.Fatalf("input = %#v, error = %v", input, err)
		}
	}
}

func TestNegotiateRejectsPluginAndPublicSchemaDrift(t *testing.T) {
	mutations := []func(*VersionEvidence){
		func(value *VersionEvidence) { value.HostSessionSchema = "oaw.host-session/v3" },
		func(value *VersionEvidence) { value.InventorySchema = "oaw.host-binding-inventory/v3" },
		func(value *VersionEvidence) { value.EnvironmentSchema = "oaw.host-environment-report/v3" },
		func(value *VersionEvidence) { value.BundleSchema = "oaw.lifecycle-bundle/v4" },
		func(value *VersionEvidence) { value.WorkflowSchema = "oaw.workflow-command/v2" },
	}
	for _, mutate := range mutations {
		input := compatibleVersions()
		mutate(&input)
		if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
			t.Fatalf("input = %#v, error = %v", input, err)
		}
	}
}

func TestNegotiateProbesMethodsInsteadOfTrustingCodexVersion(t *testing.T) {
	input := compatibleVersions()
	input.CodexVersion = "99.0.0"
	input.MetadataMethods = []string{"hooks/list", "config/read"}

	if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
}

func TestNegotiateKeepsOptionalMetadataFailuresPartial(t *testing.T) {
	input := compatibleVersions()
	input.MetadataMethods = []string{"skills/list"}

	result, err := Negotiate(input)
	if err != nil || !slices.Equal(result.MissingOptionalMethods, []string{"config/read", "hooks/list"}) {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestNegotiateRejectsExperimentalPluginListAsRequirement(t *testing.T) {
	input := compatibleVersions()
	input.MetadataMethods = append(input.MetadataMethods, "plugin/list")

	result, err := Negotiate(input)
	if err != nil || slices.Contains(result.RequiredMethods, "plugin/list") {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestNegotiateRejectsUnverifiedCodexVersions(t *testing.T) {
	for _, version := range []string{"0.146.0", "0.146.1-beta.1", "codex-cli next", "0.146", ""} {
		input := compatibleVersions()
		input.CodexVersion = version
		if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
			t.Fatalf("version = %q, error = %v", version, err)
		}
	}
}

func TestNegotiateAcceptsVerifiedBaselineAndNewerVersions(t *testing.T) {
	for _, version := range []string{"codex-cli 0.146.1", "codex-cli/0.146.1", "0.147.0", "1.0.0"} {
		input := compatibleVersions()
		input.CodexVersion = version
		result, err := Negotiate(input)
		if err != nil || !result.Compatible || result.CodexVersion != version {
			t.Fatalf("version = %q, result = %#v, error = %v", version, result, err)
		}
	}
}

func TestNegotiateAcceptsCodexAppServerUserAgent(t *testing.T) {
	input := compatibleVersions()
	input.CodexVersion = "oaw-codex-bridge/0.146.1 (Mac OS 15.7.7; arm64) Orca/1.4.175 (oaw-codex-bridge; 1.0.0)"
	result, err := Negotiate(input)
	if err != nil || !result.Compatible {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestMetadataValidationUsesCompatibilityNegotiation(t *testing.T) {
	metadata := completeFactMetadata()
	metadata.CodexVersion = "0.146.0"
	if err := validateMetadataObservation(HookContext{CWD: "/repo"}, metadata); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}

	metadata = completeFactMetadata()
	metadata.Methods = []string{"config/read", "hooks/list"}
	if err := validateMetadataObservation(HookContext{CWD: "/repo"}, metadata); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
}

func compatibleVersions() VersionEvidence {
	return VersionEvidence{
		PluginVersion:      BridgeIntegrationVersion,
		BridgeProtocol:     BridgeProtocolVersion,
		HookContextSchema:  HookContextSchemaV1,
		IntegrationVersion: BridgeIntegrationVersion,
		HostSessionSchema:  host.HostSessionSchemaV2,
		InventorySchema:    host.BindingInventorySchemaV2,
		EnvironmentSchema:  host.HostEnvironmentReportSchemaV2,
		BundleSchema:       LifecycleBundleSchemaV3,
		WorkflowSchema:     coordinator.WorkflowCommandSchemaV1,
		CodexVersion:       "codex-cli 0.146.1",
		MetadataMethods:    []string{"skills/list", "hooks/list", "config/read"},
	}
}
