package host_test

import (
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestBuiltinCodexRuntimeIntegrationIsSelectedAndPinned(t *testing.T) {
	integrations, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	var codex host.IntegrationRecord
	for _, integration := range integrations {
		if integration.Manifest.HostID == "codex" {
			codex = integration
			break
		}
	}
	if codex.ID != "oaw/codex-runner" {
		t.Fatalf("Codex integration = %q, want oaw/codex-runner", codex.ID)
	}
	if codex.Manifest.IntegrationLevel != host.RunnerManaged {
		t.Fatalf("Codex integration level = %q, want runner-managed", codex.Manifest.IntegrationLevel)
	}
	if codex.Manifest.Protocols == nil || len(codex.Manifest.Protocols) != 1 || codex.Manifest.Protocols[0] != host.RuntimeProtocolV1 {
		t.Fatalf("Codex protocols = %#v", codex.Manifest.Protocols)
	}
	if codex.Conformance == nil || !codex.Conformance.Passed || codex.Audit.Status != host.AuditPassed {
		t.Fatalf("Codex proof = audit=%#v conformance=%#v", codex.Audit, codex.Conformance)
	}
	if !slices.Contains(codex.Manifest.Features, host.FeatureProviderBindingInventory) || !containsConformanceCheck(codex.Conformance.Checks, host.CheckProviderBindingInventory) {
		t.Fatalf("Codex binding inventory proof is missing: %#v", codex)
	}
	if codex.Digest == "" || codex.ManifestDigest == "" || codex.Audit.Digest == "" || codex.Conformance.Digest == "" {
		t.Fatalf("Codex digests are incomplete: %#v", codex)
	}
	if err := host.ValidateIntegrationRecord(codex); err != nil {
		t.Fatalf("ValidateIntegrationRecord(Codex) error = %v", err)
	}
}

func containsConformanceCheck(checks []host.ConformanceCheck, id host.CheckID) bool {
	for _, check := range checks {
		if check.ID == id && check.Passed {
			return true
		}
	}
	return false
}

func TestBuiltinNonCodexHostsRemainInstructionOnly(t *testing.T) {
	integrations, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	for _, integration := range integrations {
		if integration.Manifest.HostID == "codex" {
			continue
		}
		if integration.Manifest.IntegrationLevel != host.InstructionOnly || integration.Conformance != nil || integration.Audit.Status != host.AuditPending {
			t.Fatalf("non-Codex Host was promoted: %#v", integration)
		}
	}
}
