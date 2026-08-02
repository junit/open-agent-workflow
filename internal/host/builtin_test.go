package host_test

import (
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestLoadBuiltinIntegrationsPromotesOnlySelectedCodex(t *testing.T) {
	records, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatalf("LoadBuiltinIntegrations() error = %v", err)
	}
	wantHosts := []string{"claude", "cline", "codex", "copilot", "cursor", "gemini", "opencode", "roo", "windsurf"}
	gotHosts := make([]string, 0, len(records))
	for _, record := range records {
		if err := host.ValidateIntegrationRecord(record); err != nil {
			t.Fatalf("ValidateIntegrationRecord(%s) error = %v", record.ID, err)
		}
		if record.Manifest.HostID == "codex" {
			if record.ID != "oaw/codex-runner" || record.Manifest.IntegrationLevel != host.RunnerManaged || record.Conformance == nil || !record.Conformance.Passed {
				t.Fatalf("selected Codex is not a conforming runner: %#v", record)
			}
			gotHosts = append(gotHosts, record.Manifest.HostID)
			continue
		}
		if record.Manifest.IntegrationLevel != host.InstructionOnly || len(record.Manifest.Protocols) != 0 || len(record.Manifest.BindingKinds) != 0 || len(record.Manifest.Features) != 0 || record.Conformance != nil {
			t.Fatalf("built-in %s claims Runtime capabilities: %#v", record.ID, record)
		}
		gotHosts = append(gotHosts, record.Manifest.HostID)
	}
	if !slices.Equal(gotHosts, wantHosts) {
		t.Fatalf("built-in Hosts = %#v, want %#v", gotHosts, wantHosts)
	}
	records[0].Manifest.HostID = "changed"
	fresh, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil || fresh[0].Manifest.HostID != "claude" {
		t.Fatalf("LoadBuiltinIntegrations() exposed mutable storage: %#v, %v", fresh, err)
	}
}
