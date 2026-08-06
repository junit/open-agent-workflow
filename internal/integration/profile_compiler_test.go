package integration_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestBuiltInProfileAliasesRequireVerifiedCoverage(t *testing.T) {
	available, err := builtin.Load()
	if err != nil {
		t.Fatalf("builtin.Load() error = %v", err)
	}
	for _, alias := range []string{"SP-FULL", "MATT-FULL", "ECC-FULL", "MATT-SP-HYBRID"} {
		t.Run(alias, func(t *testing.T) {
			graph, err := profile.CompileProfile(available, verifiedFor(available, nil), compileRequest(alias))
			if err != nil {
				t.Fatalf("CompileProfile(%q) error = %v", alias, err)
			}
			if graph.RecipeID() == "" || graph.Digest() == "" {
				t.Fatalf("compiled graph = %#v", graph)
			}
		})
	}

	_, err = profile.CompileProfile(available, verifiedFor(available, map[string]map[string]bool{
		"oaw/superpowers": {"completion": true},
	}), compileRequest("SP-FULL"))
	requireCode(t, err, "PROFILE_CAPABILITY_MISSING")
}

func TestEveryBuiltInRecipeCompilesFromVerifiedCapabilities(t *testing.T) {
	available, err := builtin.Load()
	if err != nil {
		t.Fatalf("builtin.Load() error = %v", err)
	}
	verified := verifiedFor(available, nil)
	for _, recipe := range available.Recipes() {
		t.Run(recipe.ID, func(t *testing.T) {
			graph, err := profile.CompileRecipe(available, verified, recipe, compileRequest(recipe.ID))
			if err != nil {
				t.Fatalf("CompileRecipe(%q) error = %v", recipe.ID, err)
			}
			if graph.RecipeID() != recipe.ID || graph.Digest() == "" {
				t.Fatalf("compiled graph = %#v", graph)
			}
		})
	}
}

func TestHybridOmitsUnverifiedOptionalECCHandlers(t *testing.T) {
	available, err := builtin.Load()
	if err != nil {
		t.Fatalf("builtin.Load() error = %v", err)
	}
	graph, err := profile.CompileProfile(available, verifiedFor(available, map[string]map[string]bool{
		"oaw/ecc": {"build-repair": true, "security-review": true},
	}), compileRequest("MATT-SP-HYBRID"))
	if err != nil {
		t.Fatalf("CompileProfile(MATT-SP-HYBRID) error = %v", err)
	}
	for _, node := range graph.Nodes() {
		if node.ProviderID == "oaw/ecc" {
			t.Fatalf("optional ECC node retained: %#v", node)
		}
	}
	if len(graph.IncidentRoutes()) != 1 || graph.IncidentRoutes()[0].Incident != "functional-failure" {
		t.Fatalf("incident routes = %#v", graph.IncidentRoutes())
	}
}

func TestCustomRecipeUsesTheSameCompilerContract(t *testing.T) {
	base, err := builtin.Load()
	if err != nil {
		t.Fatalf("builtin.Load() error = %v", err)
	}
	providers := base.Providers()
	var customProvider catalog.ProviderDescriptorRecord
	for _, provider := range providers {
		if provider.ID == "oaw/ecc" {
			customProvider = provider
		}
	}
	if customProvider.ID == "" {
		t.Fatal("built-in ECC descriptor not found")
	}
	customProvider.ID = "acme/suite"
	customProvider.DisplayName = "Acme Suite"
	customRecipe := catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV2, RecipeVersion: "2.0.0", ID: "acme/reliable-delivery", DisplayName: "Acme Delivery",
		RequiredResponsibilities: []string{"requirements", "completion"},
		Nodes: []catalog.RecipeNode{
			{ID: "requirements", Kind: catalog.PhaseNode, Responsibility: "requirements", Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: customProvider.Capabilities[0].ID}, Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "completion"}}},
			{ID: "completion", Kind: catalog.GateNode, Responsibility: "completion", Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "completion"}, Transitions: []catalog.RecipeTransition{}},
		},
		IncidentRoutes: []catalog.IncidentRoute{}, Entry: "requirements", TerminalGates: []string{"completion"}, StableBoundaries: []string{"ticket-complete"},
		EnvironmentRequirements: []execution.EnvironmentRequirement{},
	}
	customCatalog, err := catalog.New(append(providers, customProvider), append(base.Recipes(), customRecipe), base.Aliases())
	if err != nil {
		t.Fatalf("catalog.New(custom) error = %v", err)
	}
	graph, err := profile.CompileProfile(customCatalog, verifiedFor(customCatalog, nil), compileRequest("acme/reliable-delivery"))
	if err != nil {
		t.Fatalf("CompileProfile(custom) error = %v", err)
	}
	if graph.RecipeID() != "acme/reliable-delivery" || graphNode(graph, "requirements").ProviderID != "acme/suite" {
		t.Fatalf("custom graph = %#v", graph)
	}
}

func TestEquivalentRecipeInputsHaveIdenticalGraphDigest(t *testing.T) {
	available, err := builtin.Load()
	if err != nil {
		t.Fatalf("builtin.Load() error = %v", err)
	}
	recipes := available.Recipes()
	var recipe catalog.ProfileRecipeRecord
	for _, candidate := range recipes {
		if candidate.ID == "oaw/reliable-feature" {
			recipe = candidate
		}
	}
	firstRequest := compileRequest(recipe.ID)
	firstRequest.Bindings = []profile.ProfileBinding{{
		Selector: catalog.CapabilitySelector{ProviderID: "oaw/matt", CapabilityID: "tdd"}, PreferredProviderID: "oaw/matt",
	}}
	first, err := profile.CompileRecipe(available, verifiedFor(available, nil), recipe, firstRequest)
	if err != nil {
		t.Fatalf("first CompileRecipe() error = %v", err)
	}
	reverseRecipe(&recipe)
	secondRequest := compileRequest(recipe.ID)
	secondRequest.Bindings = []profile.ProfileBinding{{
		Selector: catalog.CapabilitySelector{ProviderID: "oaw/matt", CapabilityID: "tdd"}, PreferredProviderID: "oaw/matt",
	}}
	second, err := profile.CompileRecipe(available, verifiedFor(available, nil), recipe, secondRequest)
	if err != nil {
		t.Fatalf("second CompileRecipe() error = %v", err)
	}
	if first.Digest() != second.Digest() || first.RecipeDigest() != second.RecipeDigest() {
		t.Fatalf("equivalent graph digests differ: %s/%s and %s/%s", first.Digest(), first.RecipeDigest(), second.Digest(), second.RecipeDigest())
	}
}

type effectiveRegistry struct {
	hostID       string
	providers    map[string]registry.ProviderInstance
	capabilities map[string]registry.VerifiedCapability
}

func (value effectiveRegistry) HostID() string { return value.hostID }

func (value effectiveRegistry) Provider(id string) (registry.ProviderInstance, bool) {
	provider, found := value.providers[id]
	return provider, found
}

func (value effectiveRegistry) Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool) {
	capability, found := value.capabilities[providerID+"\x00"+capabilityID]
	return capability, found
}

func verifiedFor(available catalog.Catalog, omitted map[string]map[string]bool) effectiveRegistry {
	result := effectiveRegistry{hostID: "codex", providers: map[string]registry.ProviderInstance{}, capabilities: map[string]registry.VerifiedCapability{}}
	for _, provider := range available.Providers() {
		if omitted[provider.ID] != nil && omitted[provider.ID]["*"] {
			continue
		}
		instance := registry.ProviderInstance{ProviderID: provider.ID, HostID: "codex", Digest: canonicaljson.DigestBytes([]byte(provider.ID + "-instance"))}
		for _, capability := range provider.Capabilities {
			if omitted[provider.ID] != nil && omitted[provider.ID][capability.ID] {
				continue
			}
			binding := capability.HostBindings[0]
			verified := registry.VerifiedCapability{
				ID: capability.ID, Binding: binding,
				SupportedTopologies:   append([]execution.Topology{}, capability.SupportedTopologies...),
				BindingEvidenceDigest: canonicaljson.DigestBytes([]byte(provider.ID + "/" + capability.ID)),
			}
			instance.Capabilities = append(instance.Capabilities, verified)
			result.capabilities[provider.ID+"\x00"+capability.ID] = verified
		}
		if len(instance.Capabilities) > 0 {
			result.providers[provider.ID] = instance
		}
	}
	return result
}

func compileRequest(profileID string) profile.CompileRequest {
	return profile.CompileRequest{
		Profile: profileID, HostTopologies: []execution.Topology{execution.TopologyCurrent},
		EnvironmentObservations: []execution.EnvironmentObservation{},
	}
}

func reverseRecipe(recipe *catalog.ProfileRecipeRecord) {
	reverseStrings(recipe.RequiredResponsibilities)
	reverseStrings(recipe.TerminalGates)
	reverseStrings(recipe.StableBoundaries)
	for i := range recipe.Nodes {
		reverseTransitions(recipe.Nodes[i].Transitions)
	}
	for left, right := 0, len(recipe.Nodes)-1; left < right; left, right = left+1, right-1 {
		recipe.Nodes[left], recipe.Nodes[right] = recipe.Nodes[right], recipe.Nodes[left]
	}
	for left, right := 0, len(recipe.IncidentRoutes)-1; left < right; left, right = left+1, right-1 {
		recipe.IncidentRoutes[left], recipe.IncidentRoutes[right] = recipe.IncidentRoutes[right], recipe.IncidentRoutes[left]
	}
}

func reverseStrings(values []string) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseTransitions(values []catalog.RecipeTransition) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func graphNode(graph profile.ExecutionGraph, id string) profile.GraphNode {
	for _, node := range graph.Nodes() {
		if node.ID == id {
			return node
		}
	}
	return profile.GraphNode{}
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", code)
	}
	compileErr, ok := err.(*profile.CompileError)
	if !ok || compileErr.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}
