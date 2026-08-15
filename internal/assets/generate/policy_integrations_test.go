package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestGeneratePolicyIntegrationsIsIdempotentAndCarriesNoMachineAuthority(t *testing.T) {
	root := t.TempDir()
	assetsRoot := filepath.Join(root, "internal", "assets")
	if err := os.MkdirAll(assetsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := generateActiveAssets(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(assetsRoot, "host-integrations.json")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := generateActiveAssets(root); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("Policy Integration generation is not idempotent")
	}
	set, err := host.DecodeIntegrationSetJSON(second)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Integrations) != len(policyHostIDs) {
		t.Fatalf("Integration count = %d, want %d", len(set.Integrations), len(policyHostIDs))
	}
	for _, integration := range set.Integrations {
		if integration.Manifest.ControlSurface != host.SurfacePolicy || integration.Conformance != nil ||
			len(integration.Manifest.Protocols) != 0 || len(integration.Manifest.BindingKinds) != 0 ||
			len(integration.Manifest.Features) != 0 || len(integration.Manifest.DelegationFeatures) != 0 {
			t.Fatalf("Policy Integration claims machine authority: %#v", integration)
		}
	}
}
