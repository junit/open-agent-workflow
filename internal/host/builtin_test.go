package host_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestBuiltinCodexHostUsesBridgeCutoverAuthority(t *testing.T) {
	integrations, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatalf("LoadBuiltinIntegrations() error = %v", err)
	}
	var codex *host.IntegrationRecord
	for index := range integrations {
		if integrations[index].ID == "oaw/codex-host" {
			codex = &integrations[index]
			break
		}
	}
	if codex == nil || codex.SchemaVersion != host.HostIntegrationSchemaV3 || codex.IntegrationVersion != "2.0.0" ||
		codex.Manifest.SchemaVersion != host.HostManifestSchemaV3 || codex.Conformance == nil ||
		codex.Conformance.SchemaVersion != host.HostConformanceReportSchemaV4 {
		t.Fatalf("built-in Codex Host Integration = %#v", codex)
	}

	codex.Manifest.Protocols[0] = "edited"
	reloaded, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	for _, integration := range reloaded {
		if integration.ID == "oaw/codex-host" && integration.Manifest.Protocols[0] == "edited" {
			t.Fatal("LoadBuiltinIntegrations returned shared mutable storage")
		}
	}
}
