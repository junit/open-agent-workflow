package integration_test

import (
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestDefaultConfigurationKeepsPolicyAndHostIntegrationsSeparate(t *testing.T) {
	snapshot, err := config.Load(config.LoadOptions{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	records := snapshot.HostIntegrations()
	if len(records) != 10 || len(snapshot.Record().HostIntegrations) != 10 {
		t.Fatalf("default Host Integration count = %d", len(records))
	}
	policyCount := 0
	nativeCount := 0
	for _, record := range records {
		switch record.Manifest.ControlSurface {
		case host.SurfacePolicy:
			policyCount++
			if record.ID == "oaw/codex-runner" || !slices.Equal(record.Manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) ||
				len(record.Manifest.Protocols) != 0 || len(record.Manifest.Features) != 0 || record.Conformance != nil || record.Digest == "" {
				t.Fatalf("policy Integration claims Host-native guarantees: %#v", record)
			}
		case host.SurfaceHostNative:
			nativeCount++
			if record.ID != codexbridge.BridgeIntegrationID || record.IntegrationVersion != codexbridge.BridgeIntegrationVersion ||
				record.Manifest.SchemaVersion != host.HostManifestSchemaV3 ||
				!slices.Equal(record.Manifest.BindingKinds, []catalog.BindingKind{catalog.BindingSkill}) ||
				!slices.Equal(record.Manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) ||
				!slices.Equal(record.Manifest.DelegationFeatures, []host.FeatureID{host.FeatureChildDelegation}) ||
				len(record.Manifest.HostActions) != 0 ||
				record.Conformance == nil || record.Conformance.SchemaVersion != host.HostConformanceReportSchemaV4 ||
				!integrationCanSupplyInventory(snapshot, record.ID) {
				t.Fatalf("unexpected Host-native Integration: %#v", record)
			}
		default:
			t.Fatalf("unknown Integration control surface: %#v", record)
		}
	}
	if policyCount != 9 || nativeCount != 1 {
		t.Fatalf("policy count = %d, Host-native count = %d", policyCount, nativeCount)
	}
}
