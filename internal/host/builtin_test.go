package host_test

import (
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestLoadBuiltinIntegrationsUsesNinePolicySurfaces(t *testing.T) {
	records, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatalf("LoadBuiltinIntegrations() error = %v", err)
	}
	wantHosts := []string{"claude", "cline", "codex", "copilot", "cursor", "gemini", "opencode", "roo", "windsurf"}
	policies := make([]host.IntegrationRecord, 0, len(wantHosts))
	for _, record := range records {
		if record.Manifest.ControlSurface == host.SurfacePolicy {
			policies = append(policies, record)
		}
	}
	if len(policies) != len(wantHosts) {
		t.Fatalf("built-in policy Integration count = %d, want %d", len(policies), len(wantHosts))
	}
	for index, record := range policies {
		if err := host.ValidateIntegrationRecord(record); err != nil {
			t.Fatalf("ValidateIntegrationRecord(%s) error = %v", record.ID, err)
		}
		wantHost := wantHosts[index]
		if record.ID != "oaw/"+wantHost+"-policy" || record.SchemaVersion != host.HostIntegrationSchemaV3 ||
			record.Manifest.SchemaVersion != host.HostManifestSchemaV3 || record.Manifest.HostID != wantHost ||
			record.Manifest.ControlSurface != host.SurfacePolicy || !slices.Equal(record.Manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) ||
			len(record.Manifest.Protocols) != 0 || len(record.Manifest.BindingKinds) != 0 || len(record.Manifest.Features) != 0 || len(record.Manifest.DelegationFeatures) != 0 || len(record.Manifest.HostActions) != 0 ||
			record.Audit.Status != host.AuditPending || record.Conformance != nil || record.Digest == "" {
			t.Fatalf("built-in %s is not a policy Integration: %#v", record.ID, record)
		}
	}
	records[0].Manifest.SupportedTopologies[0] = execution.TopologySubagent
	fresh, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil || !slices.Equal(fresh[0].Manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
		t.Fatalf("LoadBuiltinIntegrations() exposed mutable storage: %#v, %v", fresh, err)
	}
}

func TestBuiltinCodexPolicyAndHostRemainSeparate(t *testing.T) {
	integrations, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	policy := integrationByID(t, integrations, "oaw/codex-policy")
	native := integrationByID(t, integrations, "oaw/codex-host")
	if policy.Manifest.ControlSurface != host.SurfacePolicy || native.Manifest.ControlSurface != host.SurfaceHostNative {
		t.Fatalf("policy = %#v, native = %#v", policy, native)
	}
	if !slices.Equal(native.Manifest.BindingKinds, []catalog.BindingKind{catalog.BindingSkill}) ||
		!slices.Equal(native.Manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) ||
		!slices.Equal(native.Manifest.Features, []host.Feature{
			host.FeatureEnvironmentReporting,
			host.FeatureNormalizedReceipts,
			host.FeatureProviderBindingInventory,
		}) || native.Audit.Status != host.AuditPassed || native.Conformance == nil {
		t.Fatalf("native manifest = %#v", native)
	}
	if len(policy.Manifest.Protocols) != 0 || len(policy.Manifest.BindingKinds) != 0 || len(policy.Manifest.Features) != 0 ||
		policy.Audit.Status != host.AuditPending || policy.Conformance != nil {
		t.Fatalf("policy Integration gained Host-native authority: %#v", policy)
	}
}

func integrationByID(t *testing.T, values []host.IntegrationRecord, id string) host.IntegrationRecord {
	t.Helper()
	for _, value := range values {
		if value.ID == id {
			return value
		}
	}
	t.Fatalf("Integration %q not found", id)
	return host.IntegrationRecord{}
}
