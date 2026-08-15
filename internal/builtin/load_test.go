package builtin

import (
	"encoding/json"
	"strings"
	"testing"

	oaw "github.com/wifibaby4u/open-agent-workflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

func TestLoadContainsOnlyProviderBindingIdentity(t *testing.T) {
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	providers := value.Providers()
	if len(providers) != 3 {
		t.Fatalf("Providers = %d, want 3", len(providers))
	}
	for _, provider := range providers {
		if provider.SchemaVersion != catalog.ProviderDescriptorSchemaV5 || provider.DescriptorVersion != "5.0.0" {
			t.Fatalf("Provider %s identity = %s/%s", provider.ID, provider.SchemaVersion, provider.DescriptorVersion)
		}
		if len(provider.Distributions) == 0 || len(provider.Discovery) == 0 || len(provider.Bindings) == 0 {
			t.Fatalf("Provider %s has incomplete identity evidence", provider.ID)
		}
	}
	raw, err := json.Marshal(providers)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"request_modes", "capabilities", "responsibilities", "stage_span",
		"supported_topologies", "internal_calls", "outcome_owner",
	} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("Provider identity inventory contains workflow field %q", forbidden)
		}
	}
}

func TestLoadReturnsIndependentCatalogSnapshots(t *testing.T) {
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	first := value.Providers()
	first[0].Bindings[0].Reference = "changed"
	second := value.Providers()
	if second[0].Bindings[0].Reference == "changed" {
		t.Fatal("Catalog returned mutable Provider storage")
	}
}

func TestCatalogCoversEveryBuiltInProfileReferenceForCodex(t *testing.T) {
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	references := make(map[string]bool)
	for _, provider := range value.Providers() {
		for _, binding := range provider.Bindings {
			if binding.Host == "codex" {
				references[binding.Reference] = true
			}
		}
	}
	for _, file := range oaw.CanonicalPolicySet() {
		if !strings.HasPrefix(file.Path, "profiles/") {
			continue
		}
		profile, err := oaw.ParsePolicyProfile(file.Path, file.Content)
		if err != nil {
			t.Fatalf("ParsePolicyProfile(%s): %v", file.Path, err)
		}
		for _, occurrence := range profile.Occurrences {
			if !references[occurrence.Reference] {
				t.Errorf("%s reference %q has no Codex Binding identity", file.Path, occurrence.Reference)
			}
		}
	}
}

func TestMattBindingsMatchHostInstallationLayouts(t *testing.T) {
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range value.Providers() {
		if provider.ID != "oaw/matt" {
			continue
		}
		for _, binding := range provider.Bindings {
			switch binding.Host {
			case "codex":
				if binding.InstallRoot != "skills/"+binding.Reference {
					t.Errorf("Codex Matt Binding %s InstallRoot = %q", binding.ID, binding.InstallRoot)
				}
			case "claude":
				if binding.InstallRoot != binding.ContentRoot {
					t.Errorf("Claude Matt Binding %s InstallRoot = %q, ContentRoot = %q", binding.ID, binding.InstallRoot, binding.ContentRoot)
				}
			}
		}
		return
	}
	t.Fatal("Matt Provider is missing")
}
