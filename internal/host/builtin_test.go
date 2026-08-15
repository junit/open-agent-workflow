package host_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestBuiltinIntegrationsContainOnlyPolicyAuthority(t *testing.T) {
	integrations, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatalf("LoadBuiltinIntegrations() error = %v", err)
	}
	if len(integrations) != 9 {
		t.Fatalf("built-in Integration count = %d", len(integrations))
	}
	for _, integration := range integrations {
		if integration.Manifest.ControlSurface != host.SurfacePolicy || integration.Conformance != nil ||
			len(integration.Manifest.Protocols) != 0 || len(integration.Manifest.BindingKinds) != 0 ||
			len(integration.Manifest.Features) != 0 || len(integration.Manifest.DelegationFeatures) != 0 {
			t.Fatalf("built-in Integration claims machine authority: %#v", integration)
		}
	}

	integrations[0].Manifest.SupportedTopologies[0] = "edited"
	reloaded, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	if reloaded[0].Manifest.SupportedTopologies[0] == "edited" {
		t.Fatal("LoadBuiltinIntegrations returned shared mutable storage")
	}
}
