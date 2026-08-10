package registry_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestRegistryV4PublicStateVocabulary(t *testing.T) {
	want := []registry.ProviderState{
		registry.ProviderNotFound,
		registry.ProviderCandidate,
		registry.ProviderVerified,
		registry.ProviderAmbiguous,
		registry.ProviderIncompatible,
		registry.ProviderBindingUnavailable,
		registry.ProviderDisabled,
		registry.ProviderUntrusted,
	}
	seen := make(map[registry.ProviderState]struct{}, len(want))
	for _, state := range want {
		if state == "" {
			t.Fatal("Registry v4 exposes an empty Provider state")
		}
		if _, found := seen[state]; found {
			t.Fatalf("duplicate Provider state %q", state)
		}
		seen[state] = struct{}{}
	}
}

func TestRegistryV4MissingLookupsAreClosed(t *testing.T) {
	var value registry.Registry
	if _, found := value.Provider("missing/provider"); found {
		t.Fatal("zero Registry returned a Provider")
	}
	if _, found := value.Binding("missing/provider", "binding"); found {
		t.Fatal("zero Registry returned a Binding")
	}
	if bindings := value.Bindings("missing/provider"); len(bindings) != 0 {
		t.Fatalf("zero Registry Bindings() = %#v", bindings)
	}
	if _, found := value.Capability("missing/provider", "capability"); found {
		t.Fatal("zero Registry returned a Capability")
	}
}
