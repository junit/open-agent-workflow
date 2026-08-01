package catalog

import (
	"strings"
	"testing"
)

func TestNewCatalogOrdersAndCopiesRecords(t *testing.T) {
	providers := []ProviderDescriptorRecord{testProvider("oaw/z", "z-cap", "implementation"), testProvider("oaw/a", "a-cap", "implementation")}
	recipes := []ProfileRecipeRecord{testRecipe("oaw/z", "oaw/a", "a-cap")}
	aliases := []ProfileAliasRecord{{Alias: "SP-FULL", RecipeID: "oaw/z"}}
	catalog, err := New(providers, recipes, aliases)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := catalog.Providers()[0].ID; got != "oaw/a" {
		t.Fatalf("Providers()[0].ID = %q, want oaw/a", got)
	}
	if catalog.Digest() == "" {
		t.Fatal("Digest() is empty")
	}
	providers[0].Capabilities[0].ID = "changed"
	copy := catalog.Providers()
	copy[0].Capabilities[0].ID = "changed-again"
	if catalog.Providers()[0].Capabilities[0].ID == "changed-again" {
		t.Fatal("Providers() exposed mutable storage")
	}
	if catalog.Digest() != catalog.Digest() {
		t.Fatal("Digest() is not stable")
	}
}

func TestNewCatalogRejectsCrossRecordInvariants(t *testing.T) {
	baseProvider := testProvider("oaw/provider", "implementation", "implementation")
	baseRecipe := testRecipe("oaw/recipe", "oaw/provider", "implementation")
	tests := []struct {
		name   string
		mutate func([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord) ([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord)
		code   string
	}{
		{"duplicate provider", func(p []ProviderDescriptorRecord, r []ProfileRecipeRecord, a []ProfileAliasRecord) ([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord) {
			return append(p, p[0]), r, a
		}, "DUPLICATE_PROVIDER_ID"},
		{"missing delegation target", func(p []ProviderDescriptorRecord, r []ProfileRecipeRecord, a []ProfileAliasRecord) ([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord) {
			p[0].Capabilities[0].DelegationAllowList = []string{"missing"}
			return p, r, a
		}, "DELEGATION_CAPABILITY_NOT_FOUND"},
		{"missing recipe provider", func(p []ProviderDescriptorRecord, r []ProfileRecipeRecord, a []ProfileAliasRecord) ([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord) {
			r[0].Nodes[0].Selector.ProviderID = "oaw/missing"
			return p, r, a
		}, "RECIPE_PROVIDER_NOT_FOUND"},
		{"missing recipe capability", func(p []ProviderDescriptorRecord, r []ProfileRecipeRecord, a []ProfileAliasRecord) ([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord) {
			r[0].Nodes[0].Selector.CapabilityID = "missing"
			return p, r, a
		}, "RECIPE_CAPABILITY_NOT_FOUND"},
		{"missing responsibility", func(p []ProviderDescriptorRecord, r []ProfileRecipeRecord, a []ProfileAliasRecord) ([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord) {
			r[0].RequiredResponsibilities = []string{"missing"}
			return p, r, a
		}, "RESPONSIBILITY_OWNER_MISSING"},
		{"missing node transition target", func(p []ProviderDescriptorRecord, r []ProfileRecipeRecord, a []ProfileAliasRecord) ([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord) {
			r[0].Nodes[0].Transitions[0].Target = "missing"
			return p, r, a
		}, "RECIPE_NODE_NOT_FOUND"},
		{"invalid terminal gate", func(p []ProviderDescriptorRecord, r []ProfileRecipeRecord, a []ProfileAliasRecord) ([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord) {
			r[0].TerminalGates = []string{"implementation"}
			return p, r, a
		}, "TERMINAL_GATE_INVALID"},
		{"missing alias recipe", func(p []ProviderDescriptorRecord, r []ProfileRecipeRecord, a []ProfileAliasRecord) ([]ProviderDescriptorRecord, []ProfileRecipeRecord, []ProfileAliasRecord) {
			return p, r, []ProfileAliasRecord{{Alias: "SP-FULL", RecipeID: "oaw/missing"}}
		}, "ALIAS_RECIPE_NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := cloneProviderList([]ProviderDescriptorRecord{baseProvider})
			recipes := cloneRecipeList([]ProfileRecipeRecord{baseRecipe})
			aliases := []ProfileAliasRecord{{Alias: "SP-FULL", RecipeID: "oaw/recipe"}}
			providers, recipes, aliases = tt.mutate(providers, recipes, aliases)
			_, err := New(providers, recipes, aliases)
			if err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("New() error = %v, want %s", err, tt.code)
			}
		})
	}
}

func TestNewCatalogRejectsNodeAndBindingErrors(t *testing.T) {
	provider := testProvider("oaw/provider", "implementation", "implementation")
	recipe := testRecipe("oaw/recipe", "oaw/provider", "implementation")
	cases := []struct {
		name   string
		mutate func(*ProviderDescriptorRecord, *ProfileRecipeRecord)
		code   string
	}{
		{"duplicate capability", func(p *ProviderDescriptorRecord, r *ProfileRecipeRecord) {
			p.Capabilities = append(p.Capabilities, p.Capabilities[0])
		}, "DUPLICATE_CAPABILITY_ID"},
		{"duplicate probe", func(p *ProviderDescriptorRecord, r *ProfileRecipeRecord) {
			p.Discovery = append(p.Discovery, p.Discovery[0])
		}, "DUPLICATE_DISCOVERY_PROBE_ID"},
		{"duplicate binding", func(p *ProviderDescriptorRecord, r *ProfileRecipeRecord) {
			p.Capabilities[0].HostBindings = append(p.Capabilities[0].HostBindings, p.Capabilities[0].HostBindings[0])
		}, "DUPLICATE_HOST_BINDING"},
		{"duplicate node", func(p *ProviderDescriptorRecord, r *ProfileRecipeRecord) { r.Nodes = append(r.Nodes, r.Nodes[0]) }, "DUPLICATE_RECIPE_NODE_ID"},
		{"procedure phase invalid", func(p *ProviderDescriptorRecord, r *ProfileRecipeRecord) {
			r.Nodes[0].Kind = ProcedureNode
			r.Nodes[0].Phase = "missing"
		}, "PROCEDURE_PHASE_INVALID"},
		{"procedure transition forbidden", func(p *ProviderDescriptorRecord, r *ProfileRecipeRecord) {
			r.Nodes = append(r.Nodes, RecipeNode{ID: "procedure", Kind: ProcedureNode, Responsibility: "implementation", Selector: r.Nodes[0].Selector, Phase: "implementation", Transitions: []RecipeTransition{{Signal: "succeeded", Target: "completion"}}})
		}, "PROCEDURE_TRANSITION_FORBIDDEN"},
		{"incident handler invalid", func(p *ProviderDescriptorRecord, r *ProfileRecipeRecord) {
			r.IncidentRoutes = []IncidentRoute{{Incident: "build-failure", Handler: "implementation"}}
		}, "INCIDENT_HANDLER_INVALID"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := cloneProviderList([]ProviderDescriptorRecord{provider})[0]
			r := cloneRecipeList([]ProfileRecipeRecord{recipe})[0]
			tt.mutate(&p, &r)
			if _, err := New([]ProviderDescriptorRecord{p}, []ProfileRecipeRecord{r}, nil); err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("New() error = %v, want %s", err, tt.code)
			}
		})
	}
}

func testProvider(id, capabilityID, responsibility string) ProviderDescriptorRecord {
	return ProviderDescriptorRecord{SchemaVersion: ProviderDescriptorSchemaV1, DescriptorVersion: "1.0.0", ID: id, DisplayName: id, Discovery: []DiscoveryProbe{{ID: "probe", Kind: "path-exists", Root: "user-home", Path: ".agents/skills/test/SKILL.md"}}, Capabilities: []CapabilityRecord{{ID: capabilityID, InputSchema: "in", OutcomeSchema: "out", MaximumEffects: []string{"read-project"}, Resources: []string{"project"}, RequestModes: []RequestMode{RequestModeWorkflow}, Responsibilities: []string{responsibility, "completion"}, ExecutorTopology: IsolatedRequired, HostBindings: []HostBinding{{Host: "codex", Kind: "skill", Reference: "test"}}}}}
}

func testRecipe(id, providerID, capabilityID string) ProfileRecipeRecord {
	return ProfileRecipeRecord{SchemaVersion: ProfileRecipeSchemaV1, RecipeVersion: "1.0.0", ID: id, DisplayName: id, RequiredResponsibilities: []string{"implementation", "completion"}, Nodes: []RecipeNode{{ID: "implementation", Kind: PhaseNode, Responsibility: "implementation", Selector: CapabilitySelector{ProviderID: providerID, CapabilityID: capabilityID}, Transitions: []RecipeTransition{{Signal: "succeeded", Target: "completion"}}}, {ID: "completion", Kind: GateNode, Responsibility: "completion", Selector: CapabilitySelector{ProviderID: providerID, CapabilityID: capabilityID}, Transitions: []RecipeTransition{}}}, Entry: "implementation", TerminalGates: []string{"completion"}, StableBoundaries: []string{"complete"}}
}
