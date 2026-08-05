package host_test

import (
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestLoadBuiltinIntegrationsUsesNinePolicySurfaces(t *testing.T) {
	records, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatalf("LoadBuiltinIntegrations() error = %v", err)
	}
	wantHosts := []string{"claude", "cline", "codex", "copilot", "cursor", "gemini", "opencode", "roo", "windsurf"}
	if len(records) != len(wantHosts) {
		t.Fatalf("built-in Integration count = %d, want %d", len(records), len(wantHosts))
	}
	for index, record := range records {
		if err := host.ValidateIntegrationRecord(record); err != nil {
			t.Fatalf("ValidateIntegrationRecord(%s) error = %v", record.ID, err)
		}
		wantHost := wantHosts[index]
		if record.ID != "oaw/"+wantHost+"-policy" || record.SchemaVersion != host.HostIntegrationSchemaV2 ||
			record.Manifest.SchemaVersion != host.HostManifestSchemaV2 || record.Manifest.HostID != wantHost ||
			record.Manifest.ControlSurface != host.SurfacePolicy || !slices.Equal(record.Manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) ||
			len(record.Manifest.Protocols) != 0 || len(record.Manifest.BindingKinds) != 0 || len(record.Manifest.Features) != 0 ||
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
