package integration_test

import (
	"slices"
	"testing"

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
	if len(records) != 9 || len(snapshot.Record().HostIntegrations) != 9 {
		t.Fatalf("default Host Integration count = %d", len(records))
	}
	for _, record := range records {
		if record.ID == codexbridge.BridgeIntegrationID || record.ID == "oaw/codex-host" {
			t.Fatalf("default configuration includes Bridge authority: %#v", record)
		}
		if record.Manifest.ControlSurface != host.SurfacePolicy ||
			!slices.Equal(record.Manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) ||
			len(record.Manifest.Protocols) != 0 || len(record.Manifest.BindingKinds) != 0 ||
			len(record.Manifest.Features) != 0 || len(record.Manifest.DelegationFeatures) != 0 ||
			len(record.Manifest.HostActions) != 0 || record.Conformance != nil || record.Digest == "" {
			t.Fatalf("policy Integration claims Host-native guarantees: %#v", record)
		}
	}
}
